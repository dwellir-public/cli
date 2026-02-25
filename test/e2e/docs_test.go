//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocsSearchJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/docs/llms.txt":
			_, _ = w.Write([]byte(`# Dwellir Documentation

## Core Documentation
- [Authentication](https://www.dwellir.com/docs/authentication): Learn auth in the CLI.
- [Getting Started](https://www.dwellir.com/docs/getting-started): Install the CLI.
`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	res := runCLIWithEnv(t, map[string]string{
		"DWELLIR_DOCS_INDEX_URL": server.URL + "/docs/llms.txt",
		"DWELLIR_DOCS_BASE_URL":  server.URL + "/docs",
	}, "docs", "search", "auth", "--json")

	if res.exitCode != 0 {
		t.Fatalf("expected success exit code, got %d\nstderr: %s\nstdout: %s", res.exitCode, res.stderr, res.stdout)
	}

	parsed := parseJSON(t, res.stdout)
	if ok, _ := parsed["ok"].(bool); !ok {
		t.Fatalf("expected ok=true, got: %v", parsed["ok"])
	}

	items, _ := parsed["data"].([]interface{})
	if len(items) == 0 {
		t.Fatalf("expected at least one docs result, got: %v", parsed["data"])
	}

	first, _ := items[0].(map[string]interface{})
	if first["slug"] != "authentication" {
		t.Fatalf("expected first slug authentication, got %v", first["slug"])
	}
}

func TestDocsGetJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/docs/llms.txt":
			_, _ = w.Write([]byte(`# Dwellir Documentation

## Core Documentation
- [Authentication](https://www.dwellir.com/docs/authentication): Learn auth in the CLI.
`))
		case "/docs/authentication.md":
			_, _ = w.Write([]byte("# Authentication\n\nUse tokens."))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	res := runCLIWithEnv(t, map[string]string{
		"DWELLIR_DOCS_INDEX_URL": server.URL + "/docs/llms.txt",
		"DWELLIR_DOCS_BASE_URL":  server.URL + "/docs",
	}, "docs", "get", "authentication", "--json")

	if res.exitCode != 0 {
		t.Fatalf("expected success exit code, got %d\nstderr: %s\nstdout: %s", res.exitCode, res.stderr, res.stdout)
	}

	parsed := parseJSON(t, res.stdout)
	if ok, _ := parsed["ok"].(bool); !ok {
		t.Fatalf("expected ok=true, got: %v", parsed["ok"])
	}

	page, _ := parsed["data"].(map[string]interface{})
	if page["slug"] != "authentication" {
		t.Fatalf("expected slug authentication, got %v", page["slug"])
	}
	if page["title"] != "Authentication" {
		t.Fatalf("expected title Authentication, got %v", page["title"])
	}
}

func TestDocsListDefaultLimit(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Dwellir Documentation\n\n## Core Documentation\n")
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&b, "- [Doc %02d](https://www.dwellir.com/docs/doc-%02d): Generated test page.\n", i, i)
	}
	llms := b.String()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/docs/llms.txt" {
			_, _ = w.Write([]byte(llms))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	res := runCLIWithEnv(t, map[string]string{
		"DWELLIR_DOCS_INDEX_URL": server.URL + "/docs/llms.txt",
		"DWELLIR_DOCS_BASE_URL":  server.URL + "/docs",
	}, "docs", "list", "--json")

	if res.exitCode != 0 {
		t.Fatalf("expected success exit code, got %d\nstderr: %s\nstdout: %s", res.exitCode, res.stderr, res.stdout)
	}

	parsed := parseJSON(t, res.stdout)
	items, _ := parsed["data"].([]interface{})
	if len(items) != 25 {
		t.Fatalf("expected default list limit of 25, got %d", len(items))
	}
}
