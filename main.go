package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	viewsDir   = "views"
	postsDir   = "views/posts"
	authorsDir = "views/authors"
	pagesDir   = "views/pages"
	assetsDir  = "assets"
	distDir    = "dist"
	layoutFile = "layout.html"
	certDir    = ".certs"
)

// ── Post metadata ─────────────────────────────────────────────────────────────

type Post struct {
	Title       string
	Author      string
	Description string
	Date        string
	Tags        []string
	Lang        string // "sv" or "en"
	URL         string // e.g. "/om-brev" or "/en/on-letters"
	DestPath    string // path of rendered file inside dist/
}

type postSource struct {
	path string
	src  string
	date string
}

// ── Entry point ───────────────────────────────────────────────────────────────

func main() {
	watch := flag.Bool("watch", false, "watch for changes and serve over HTTPS")
	port := flag.Int("port", 8443, "HTTPS port for dev server")
	flag.Parse()

	if err := build(); err != nil {
		fmt.Fprintln(os.Stderr, "build error:", err)
		os.Exit(1)
	}
	if *watch {
		go watchAndRebuild()
		serve(*port)
	}
}

// ── Build ─────────────────────────────────────────────────────────────────────

func build() error {
	layout, err := os.ReadFile(layoutFile)
	if err != nil {
		return fmt.Errorf("reading layout.html: %w", err)
	}
	if err := os.RemoveAll(distDir); err != nil {
		return err
	}
	if err := os.MkdirAll(distDir, 0755); err != nil {
		return err
	}
	if err := copyDir(assetsDir, distDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("copying assets: %w", err)
	}
	posts, err := renderContent(string(layout))
	if err != nil {
		return err
	}
	if err := generateTopics(string(layout), posts); err != nil {
		return err
	}
	return generateBrowse(string(layout), posts)
}

func renderContent(layout string) ([]Post, error) {
	if err := renderPages(layout); err != nil {
		return nil, err
	}
	if err := renderAuthors(layout); err != nil {
		return nil, err
	}
	return renderPosts(layout)
}

// renderPosts walks views/posts/, sorts by date, injects next-post links, and
// copies the newest post to dist/index.html.
func renderPosts(layout string) ([]Post, error) {
	var all []postSource
	if err := walkViewDir(postsDir, "", func(path, src string) error {
		head := extractBetween(src, "<head>", "</head>")
		all = append(all, postSource{path: path, src: src, date: extractMeta(head, "date")})
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].date > all[j].date })

	metas := make([]Post, len(all))
	for i, s := range all {
		head := extractBetween(s.src, "<head>", "</head>")
		metas[i] = Post{Title: extractBetween(head, "<title>", "</title>"), URL: urlFromPath(s.path)}
	}

	posts := make([]Post, len(all))
	for i, s := range all {
		var next *Post
		if i+1 < len(metas) {
			next = &metas[i+1]
		}
		post, err := renderFile(s.path, s.src, layout, next)
		if err != nil {
			return nil, err
		}
		posts[i] = post
	}

	if len(posts) > 0 {
		indexPath := filepath.Join(distDir, "index.html")
		if posts[0].DestPath != indexPath {
			if err := copyFile(posts[0].DestPath, indexPath); err != nil {
				return nil, err
			}
			fmt.Printf("index: %s\n", posts[0].URL)
		}
	}
	return posts, nil
}

// renderPages walks views/pages/ and renders each as a plain static page.
func renderPages(layout string) error {
	return walkViewDir(pagesDir, "", func(path, src string) error {
		_, err := renderFile(path, src, layout, nil)
		return err
	})
}

// renderAuthors walks views/authors/, renders each bio page, and generates
// the /skribenter index listing all authors.
func renderAuthors(layout string) error {
	type authorEntry struct {
		Name string
		URL  string
	}
	var authors []authorEntry

	if err := walkViewDir(authorsDir, "skribenter", func(path, src string) error {
		post, err := renderFile(path, src, layout, nil)
		if err != nil {
			return err
		}
		authors = append(authors, authorEntry{Name: post.Title, URL: post.URL})
		return nil
	}); err != nil {
		return err
	}
	if len(authors) == 0 {
		return nil
	}
	sort.Slice(authors, func(i, j int) bool { return authors[i].Name < authors[j].Name })

	var b strings.Builder
	b.WriteString("<h1>Skribenter</h1>\n<ul class=\"post-list\">\n")
	for _, a := range authors {
		b.WriteString(fmt.Sprintf("\t<li><a href=\"%s\">%s</a></li>\n", a.URL, a.Name))
	}
	b.WriteString("</ul>\n")
	return writePage(layout, "/skribenter/index.html", "<title>Skribenter</title>", b.String())
}

// walkViewDir walks a views sub-directory, copies non-HTML assets to dist
// under distPrefix, and calls fn for each HTML file.
func walkViewDir(dir, distPrefix string, fn func(path, src string) error) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if filepath.Base(path) == "comments.html" {
			return nil // included by renderFile, not rendered standalone
		}
		if !strings.HasSuffix(path, ".html") {
			rel, _ := filepath.Rel(dir, path)
			dest := filepath.Join(distDir, distPrefix, rel)
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				return err
			}
			return copyFile(path, dest)
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return fn(path, string(src))
	})
}

func renderFile(path, src, layout string, next *Post) (Post, error) {
	head := extractBetween(src, "<head>", "</head>")
	body := extractBetween(src, "<body>", "</body>")

	title := extractBetween(head, "<title>", "</title>")
	date := extractMeta(head, "date")
	author := extractMeta(head, "author")
	description := extractMeta(head, "description")
	tagsRaw := extractMeta(head, "tags")

	var tags []string
	for _, t := range strings.Split(tagsRaw, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}

	isPost := strings.HasPrefix(filepath.ToSlash(path), "views/posts/")
	if isPost {
		body += commentSection(path, urlFromPath(path))
	}
	if next != nil {
		body += fmt.Sprintf("\n<div class=\"post-nav\"><a href=\"%s\">%s →</a></div>", next.URL, next.Title)
	}

	container := "container"
	if extractMeta(head, "layout") == "wide" {
		container = "meta-container"
	}
	scripts := "<script src=\"/js/article.js\"></script>"
	if isPost {
		scripts += "\n<script src=\"/js/comments.js\"></script>"
	}
	wrapped := "<div class=\"" + container + "\">\n" + body + "\n</div>\n" + scripts
	out := strings.ReplaceAll(layout, "{{HEAD}}", head)
	out = strings.ReplaceAll(out, "{{BODY}}", wrapped)

	url := urlFromPath(path)
	var dest string
	if url == "/" {
		dest = filepath.Join(distDir, "index.html")
	} else {
		dest = filepath.Join(distDir, filepath.FromSlash(strings.TrimPrefix(url, "/")), "index.html")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return Post{}, err
	}
	fmt.Printf("built: %s\n", dest)
	if err := os.WriteFile(dest, []byte(out), 0644); err != nil {
		return Post{}, err
	}

	return Post{
		Title:       title,
		Author:      author,
		Description: description,
		Date:        date,
		Tags:        tags,
		Lang:        langFromPath(path),
		URL:         urlFromPath(path),
		DestPath:    dest,
	}, nil
}

func commentSection(path, postURL string) string {
	slug := strings.TrimPrefix(postURL, "/")
	var existing string
	if data, err := os.ReadFile(filepath.Join(filepath.Dir(path), "comments.html")); err == nil {
		existing = "\n" + string(data)
	}
	return fmt.Sprintf(`
<section class="comments" data-post="%s">
<h2>Kommentarer</h2>%s
<div class="comment-form-wrap">
<h3>Lämna en kommentar</h3>
<div class="reply-notice" style="display:none">
  Svarar på: <span class="reply-to-name"></span>
  <button class="cancel-reply" type="button">×</button>
</div>
<form class="comment-form">
  <input type="hidden" name="parent_id" value="">
  <div class="form-field"><label for="c-name">Namn</label><input id="c-name" type="text" name="name" required></div>
  <div class="form-field"><label for="c-email">E-post</label><input id="c-email" type="email" name="email" required></div>
  <div class="form-field"><label for="c-text">Kommentar</label><textarea id="c-text" name="comment" rows="4" required></textarea></div>
  <div class="cf-turnstile" data-sitekey="0x4AAAAAADQiKUKHRY-f0Np2" data-theme="light"></div>
  <button type="submit">Skicka kommentar</button>
  <p class="form-status"></p>
</form>
</div>
</section>`, slug, existing)
}

// ── Topic / tag page generation ───────────────────────────────────────────────

// ── Browse page ───────────────────────────────────────────────────────────────

func generateBrowse(layout string, posts []Post) error {
	if len(posts) == 0 {
		return nil
	}
	sorted := make([]Post, len(posts))
	copy(sorted, posts)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Date > sorted[j].Date })

	var b strings.Builder
	b.WriteString("<h1>Bläddra</h1>\n<div class=\"browse-grid\">\n")
	for _, p := range sorted {
		b.WriteString("\t<div class=\"card\">\n")
		b.WriteString(fmt.Sprintf("\t\t<a class=\"card-link\" href=\"%s\">\n", p.URL))
		b.WriteString(fmt.Sprintf("\t\t\t<h3>%s</h3>\n", p.Title))
		meta := p.Date
		if p.Author != "" {
			meta = p.Author + " · " + meta
		}
		if meta != "" {
			b.WriteString(fmt.Sprintf("\t\t\t<p class=\"card-meta\">%s</p>\n", meta))
		}
		if p.Description != "" {
			b.WriteString(fmt.Sprintf("\t\t\t<p class=\"card-desc\">%s</p>\n", p.Description))
		}
		b.WriteString("\t\t</a>\n")
		b.WriteString("\t</div>\n")
	}
	b.WriteString("</div>\n")

	return writePage(layout, "/bladdra/index.html", "<title>Bläddra</title>", b.String())
}

// ── Topic / tag page generation ───────────────────────────────────────────────

func generateTopics(layout string, posts []Post) error {
	return generateTagPages(layout, posts, "/amnen", "Ämnen", "← Alla ämnen")
}

func generateTagPages(layout string, posts []Post, base, indexTitle, backLabel string) error {
	tagPosts := map[string][]Post{}
	tagNames := map[string]string{}

	for _, p := range posts {
		for _, t := range p.Tags {
			slug := tagSlug(t)
			if slug == "" {
				continue
			}
			tagPosts[slug] = append(tagPosts[slug], p)
			if _, exists := tagNames[slug]; !exists {
				tagNames[slug] = titleCase(t)
			}
		}
	}
	if len(tagPosts) == 0 {
		return nil
	}

	slugs := sortedKeys(tagPosts)

	// Tag index page
	var b strings.Builder
	b.WriteString("<h1>" + indexTitle + "</h1>\n<ul class=\"tag-list\">\n")
	for _, slug := range slugs {
		name := tagNames[slug]
		b.WriteString(fmt.Sprintf(
			"\t<li><a href=\"%s/%s\">%s</a> <span class=\"tag-count\">%d</span></li>\n",
			base, slug, name, len(tagPosts[slug]),
		))
	}
	b.WriteString("</ul>\n")
	if err := writePage(layout, base+"/index.html", "<title>"+indexTitle+"</title>", b.String()); err != nil {
		return err
	}

	// Per-tag pages
	for _, slug := range slugs {
		name := tagNames[slug]
		ps := tagPosts[slug]
		sort.Slice(ps, func(i, j int) bool { return ps[i].Date > ps[j].Date })

		var b strings.Builder
		b.WriteString("<h1>" + name + "</h1>\n<ul class=\"post-list\">\n")
		for _, p := range ps {
			b.WriteString(fmt.Sprintf("\t<li><a href=\"%s\">%s</a>", p.URL, p.Title))
			if p.Date != "" {
				b.WriteString(fmt.Sprintf(" <time>%s</time>", p.Date))
			}
			b.WriteString("</li>\n")
		}
		b.WriteString("</ul>\n")
		b.WriteString(fmt.Sprintf("<p class=\"back-link\"><a href=\"%s\">%s</a></p>\n", base, backLabel))

		head := fmt.Sprintf("<title>%s · %s</title>", name, indexTitle)
		if err := writePage(layout, base+"/"+slug+".html", head, b.String()); err != nil {
			return err
		}
	}
	return nil
}

func writePage(layout, path, head, body string) error {
	wrapped := "<div class=\"meta-container\">\n" + body + "\n</div>"
	out := strings.ReplaceAll(layout, "{{HEAD}}", head)
	out = strings.ReplaceAll(out, "{{BODY}}", wrapped)
	if !strings.HasSuffix(path, "/index.html") && strings.HasSuffix(path, ".html") {
		path = strings.TrimSuffix(path, ".html") + "/index.html"
	}
	dest := filepath.Join(distDir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	fmt.Printf("generated: %s\n", dest)
	return os.WriteFile(dest, []byte(out), 0644)
}

// ── Metadata extraction ───────────────────────────────────────────────────────

func extractBetween(s, open, close string) string {
	start := strings.Index(s, open)
	if start == -1 {
		return ""
	}
	start += len(open)
	end := strings.Index(s[start:], close)
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(s[start : start+end])
}

// extractMeta finds <meta name="NAME" content="VALUE"> regardless of attribute order.
func extractMeta(head, name string) string {
	lower := strings.ToLower(head)
	needle := `name="` + name + `"`
	idx := strings.Index(lower, needle)
	if idx == -1 {
		return ""
	}
	tagStart := strings.LastIndex(lower[:idx], "<meta")
	if tagStart == -1 {
		return ""
	}
	tagEnd := strings.Index(lower[tagStart:], ">")
	if tagEnd == -1 {
		return ""
	}
	tag := head[tagStart : tagStart+tagEnd+1]
	return extractAttr(tag, "content")
}

func extractAttr(tag, attr string) string {
	lower := strings.ToLower(tag)
	for _, q := range []string{`"`, `'`} {
		prefix := attr + "=" + q
		i := strings.Index(lower, prefix)
		if i == -1 {
			continue
		}
		start := i + len(prefix)
		end := strings.Index(tag[start:], q)
		if end == -1 {
			continue
		}
		return tag[start : start+end]
	}
	return ""
}

// ── Path helpers ──────────────────────────────────────────────────────────────

func langFromPath(path string) string {
	// English posts live under views/posts/en/
	rel, _ := filepath.Rel(postsDir, path)
	parts := strings.SplitN(filepath.ToSlash(rel), "/", 2)
	if len(parts) > 0 && parts[0] == "en" {
		return "en"
	}
	return "sv"
}

func urlFromPath(path string) string {
	abs, _ := filepath.Abs(path)

	tryStrip := func(base, urlPrefix string) (string, bool) {
		absBase, _ := filepath.Abs(base)
		rel, err := filepath.Rel(absBase, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", false
		}
		s := filepath.ToSlash(rel)
		s = strings.TrimSuffix(s, ".html")
		if strings.HasSuffix(s, "/index") {
			s = strings.TrimSuffix(s, "/index")
		}
		if s == "index" {
			s = ""
		}
		if urlPrefix != "" {
			return "/" + urlPrefix + "/" + s, true
		}
		return "/" + s, true
	}

	if url, ok := tryStrip(authorsDir, "skribenter"); ok {
		return url
	}
	if url, ok := tryStrip(postsDir, ""); ok {
		return url
	}
	if url, ok := tryStrip(pagesDir, ""); ok {
		return url
	}
	// fallback
	rel, _ := filepath.Rel(viewsDir, path)
	s := strings.TrimSuffix(filepath.ToSlash(rel), ".html")
	if strings.HasSuffix(s, "/index") {
		s = strings.TrimSuffix(s, "/index")
	}
	return "/" + s
}

// ── String helpers ────────────────────────────────────────────────────────────

func tagSlug(tag string) string {
	s := strings.ToLower(strings.TrimSpace(tag))
	s = strings.NewReplacer("ä", "a", "ö", "o", "å", "a", " ", "-").Replace(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}

func titleCase(s string) string {
	words := strings.Fields(strings.TrimSpace(s))
	for i, w := range words {
		runes := []rune(w)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}

func sortedKeys(m map[string][]Post) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ── Assets ────────────────────────────────────────────────────────────────────

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		dest := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		return copyFile(path, dest)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// ── Watcher ───────────────────────────────────────────────────────────────────

func watchAndRebuild() {
	var lastMod time.Time
	for {
		time.Sleep(200 * time.Millisecond)
		latest, changed := latestMod(lastMod)
		if !changed {
			continue
		}
		lastMod = latest
		fmt.Println("change detected, rebuilding...")
		if err := build(); err != nil {
			fmt.Fprintln(os.Stderr, "build error:", err)
		}
	}
}

func latestMod(since time.Time) (time.Time, bool) {
	var latest time.Time
	for _, root := range []string{viewsDir, assetsDir, layoutFile} {
		_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if t := info.ModTime(); t.After(latest) {
				latest = t
			}
			return nil
		})
	}
	return latest, latest.After(since)
}

// ── HTTPS server ──────────────────────────────────────────────────────────────

func serve(port int) {
	cert, isNew, err := loadOrCreateCert()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cert error:", err)
		os.Exit(1)
	}
	if isNew {
		printTrustInstructions()
	}
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.FileServer(http.Dir(distDir)),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}
	fmt.Printf("serving https://localhost:%d\n", port)
	if err := server.ListenAndServeTLS("", ""); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ── TLS cert generation ───────────────────────────────────────────────────────

func loadOrCreateCert() (tls.Certificate, bool, error) {
	certPath := filepath.Join(certDir, "server.crt")
	keyPath := filepath.Join(certDir, "server.key")
	if _, err := os.Stat(certPath); err == nil {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		return cert, false, err
	}
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return tls.Certificate{}, false, err
	}
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, false, err
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "visj local CA"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, false, err
	}
	caCert, _ := x509.ParseCertificate(caCertDER)
	if err := writePEM(filepath.Join(certDir, "ca.crt"), "CERTIFICATE", caCertDER); err != nil {
		return tls.Certificate{}, false, err
	}
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, false, err
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(2 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, false, err
	}
	if err := writePEM(certPath, "CERTIFICATE", serverCertDER); err != nil {
		return tls.Certificate{}, false, err
	}
	serverKeyDER, _ := x509.MarshalECPrivateKey(serverKey)
	if err := writePEM(keyPath, "EC PRIVATE KEY", serverKeyDER); err != nil {
		return tls.Certificate{}, false, err
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	return cert, true, err
}

func writePEM(path, typ string, der []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: typ, Bytes: der})
}

func printTrustInstructions() {
	caPath, _ := filepath.Abs(filepath.Join(certDir, "ca.crt"))
	fmt.Println("\nFirst run: trust the local CA once so the browser stops complaining:")
	switch runtime.GOOS {
	case "darwin":
		fmt.Printf("  sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain %s\n\n", caPath)
	case "linux":
		fmt.Printf("  sudo cp %s /usr/local/share/ca-certificates/visj-local.crt && sudo update-ca-certificates\n\n", caPath)
	default:
		fmt.Printf("  Import %s into your system trusted root CA store\n\n", caPath)
	}
}
