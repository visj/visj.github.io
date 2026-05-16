package main

// Poll script — lists contacts and comments from Cloudflare D1.
//
// Usage:
//   go run .                  list everything
//   go run . delete <id>      delete a row by id (contacts or comments table)
//
// Required env vars:
//   CF_ACCOUNT_ID, CF_D1_DATABASE_ID, CF_API_TOKEN

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	accountID  = mustEnv("CF_ACCOUNT_ID")
	dbID       = mustEnv("CF_D1_DATABASE_ID")
	apiToken   = mustEnv("CF_API_TOKEN")
	httpClient = &http.Client{Timeout: 15 * time.Second}
)

type Contact struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

type Comment struct {
	ID        string `json:"id"`
	ParentID  string `json:"parent_id"`
	Post      string `json:"post"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"created_at"`
}

func main() {
	if len(os.Args) == 3 && os.Args[1] == "delete" {
		deleteRow(os.Args[2])
		return
	}

	fmt.Println("\n── Kontaktmeddelanden ──────────────────────────────")
	var contacts []Contact
	must(queryInto("SELECT * FROM contacts ORDER BY created_at DESC", &contacts))
	if len(contacts) == 0 {
		fmt.Println("  (inga)")
	}
	for _, c := range contacts {
		fmt.Printf("\n  ID:   %s\n", c.ID)
		fmt.Printf("  Från: %s", c.Name)
		if c.Email != "" {
			fmt.Printf(" <%s>", c.Email)
		}
		fmt.Printf("\n  Tid:  %s\n", fmtTime(c.CreatedAt))
		fmt.Printf("  Text:\n    %s\n", indent(c.Message))
		fmt.Printf("  → Ta bort: go run . delete %s\n", c.ID)
	}

	fmt.Println("\n── Kommentarer ─────────────────────────────────────")
	var comments []Comment
	must(queryInto("SELECT * FROM comments ORDER BY created_at DESC", &comments))
	if len(comments) == 0 {
		fmt.Println("  (inga)")
	}
	for _, c := range comments {
		fmt.Printf("\n  ID:      %s\n", c.ID)
		fmt.Printf("  Inlägg:  %s\n", c.Post)
		fmt.Printf("  Från:    %s <%s>\n", c.Name, c.Email)
		if c.ParentID != "" {
			fmt.Printf("  Svarar:  %s\n", c.ParentID)
		}
		fmt.Printf("  Tid:     %s\n", fmtTime(c.CreatedAt))
		fmt.Printf("  Text:\n    %s\n", indent(c.Comment))
		fmt.Printf("\n  Klistra in i views/posts/%s/comments.html:\n", c.Post)
		fmt.Println("  " + strings.Repeat("─", 56))
		for _, line := range strings.Split(htmlSnippet(c), "\n") {
			fmt.Println("  " + line)
		}
		fmt.Println("  " + strings.Repeat("─", 56))
		fmt.Printf("  → Ta bort: go run . delete %s\n", c.ID)
	}
	fmt.Println()
}

func deleteRow(id string) {
	// Try comments first, then contacts
	for _, table := range []string{"comments", "contacts"} {
		rows, err := queryRaw(fmt.Sprintf("SELECT id FROM %s WHERE id = ?", table), id)
		if err != nil || len(rows) == 0 {
			continue
		}
		must(exec(fmt.Sprintf("DELETE FROM %s WHERE id = ?", table), id))
		fmt.Printf("Raderat %s från %s\n", id, table)
		return
	}
	fmt.Fprintln(os.Stderr, "Hittade inget med id:", id)
	os.Exit(1)
}

// ── D1 REST API ───────────────────────────────────────────────────────────────

func queryInto[T any](sql string, out *[]T) error {
	rows, err := queryRaw(sql)
	if err != nil {
		return err
	}
	data, _ := json.Marshal(rows)
	return json.Unmarshal(data, out)
}

func queryRaw(sql string, params ...any) ([]map[string]any, error) {
	res, err := d1("query", sql, params)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func exec(sql string, params ...any) error {
	_, err := d1("query", sql, params)
	return err
}

func d1(endpoint, sql string, params []any) ([]map[string]any, error) {
	if params == nil {
		params = []any{}
	}
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/%s",
		accountID, dbID, endpoint)

	payload, _ := json.Marshal(map[string]any{"sql": sql, "params": params})
	req, _ := http.NewRequest("POST", url, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Result []struct {
			Results []map[string]any `json:"results"`
		} `json:"result"`
		Success bool `json:"success"`
		Errors  []struct{ Message string } `json:"errors"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if !result.Success {
		if len(result.Errors) > 0 {
			return nil, fmt.Errorf("D1: %s", result.Errors[0].Message)
		}
		return nil, fmt.Errorf("D1: okänt fel")
	}
	if len(result.Result) > 0 {
		return result.Result[0].Results, nil
	}
	return nil, nil
}

// ── Formatting ────────────────────────────────────────────────────────────────

var svMonths = [...]string{
	"", "januari", "februari", "mars", "april", "maj", "juni",
	"juli", "augusti", "september", "oktober", "november", "december",
}

func htmlSnippet(c Comment) string {
	parentAttr := ""
	if c.ParentID != "" {
		parentAttr = fmt.Sprintf(` data-parent="%s"`, c.ParentID)
	}
	return fmt.Sprintf(
		"<div class=\"comment\" id=\"%s\"%s>\n"+
			"  <div class=\"comment-meta\">\n"+
			"    <span class=\"comment-name\">%s</span>\n"+
			"    <time>%s</time>\n"+
			"    <button class=\"reply-btn\" type=\"button\">Svara</button>\n"+
			"  </div>\n"+
			"  <p>%s</p>\n"+
			"</div>",
		c.ID, parentAttr, htmlEsc(c.Name), fmtDateSv(c.CreatedAt), htmlEsc(c.Comment),
	)
}

func fmtDateSv(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	return fmt.Sprintf("%d %s %d", t.Day(), svMonths[t.Month()], t.Year())
}

func fmtTime(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	return t.Format("2006-01-02 15:04")
}

func indent(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "\n", "\n    ")
}

func htmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func mustEnv(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	fmt.Fprintf(os.Stderr, "saknad env-variabel: %s\n", key)
	os.Exit(1)
	return ""
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
