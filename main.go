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
	"strings"
	"time"
)

const (
	contentDir = "content"
	assetsDir  = "assets"
	distDir    = "dist"
	layoutFile = "layout.html"
	certDir    = ".certs"
)

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
	return filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		return processFile(path, string(layout))
	})
}

func processFile(path, layout string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(src)
	head := extractBetween(content, "<head>", "</head>")
	body := extractBetween(content, "<body>", "</body>")

	out := strings.ReplaceAll(layout, "{{HEAD}}", head)
	out = strings.ReplaceAll(out, "{{BODY}}", body)

	rel, _ := filepath.Rel(contentDir, path)
	dest := filepath.Join(distDir, rel)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	fmt.Printf("built: %s\n", dest)
	return os.WriteFile(dest, []byte(out), 0644)
}

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
	for _, root := range []string{contentDir, assetsDir, layoutFile} {
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

	caPath := filepath.Join(certDir, "ca.crt")
	if err := writePEM(caPath, "CERTIFICATE", caCertDER); err != nil {
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
