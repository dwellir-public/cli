package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocsListParsesLLMSIndex(t *testing.T) {
	llms := `# Dwellir Documentation

## Core Documentation
- [Authentication](https://www.dwellir.com/docs/authentication): Learn how to authenticate with tokens and profiles.
- [Getting Started](https://www.dwellir.com/docs/getting-started): Install and run your first command.

## Hyperliquid
- [Historical Data](https://www.dwellir.com/docs/hyperliquid/historical-data): Query historical market data.
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/docs/llms.txt":
			_, _ = w.Write([]byte(llms))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	docs := NewDocsAPIWithURLs(server.URL+"/docs/llms.txt", server.URL+"/docs", server.Client())
	entries, err := docs.List()
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Title != "Authentication" {
		t.Fatalf("expected first title Authentication, got %q", entries[0].Title)
	}
	if entries[0].Section != "Core Documentation" {
		t.Fatalf("expected section Core Documentation, got %q", entries[0].Section)
	}
	if entries[0].Slug != "authentication" {
		t.Fatalf("expected slug authentication, got %q", entries[0].Slug)
	}
	if entries[0].MarkdownURL != server.URL+"/docs/authentication.md" {
		t.Fatalf("expected markdown url %q, got %q", server.URL+"/docs/authentication.md", entries[0].MarkdownURL)
	}
}

func TestDocsSearchMatchesQueryAndLimit(t *testing.T) {
	llms := `# Dwellir Documentation

## Core Documentation
- [Authentication](https://www.dwellir.com/docs/authentication): Learn how to authenticate with tokens and profiles.
- [Getting Started](https://www.dwellir.com/docs/getting-started): Install and run your first command.

## Hyperliquid
- [Historical Data](https://www.dwellir.com/docs/hyperliquid/historical-data): Query historical market data.
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/docs/llms.txt" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(llms))
	}))
	defer server.Close()

	docs := NewDocsAPIWithURLs(server.URL+"/docs/llms.txt", server.URL+"/docs", server.Client())
	results, err := docs.Search("historical market", 1)
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result due to limit, got %d", len(results))
	}
	if results[0].Slug != "hyperliquid/historical-data" {
		t.Fatalf("expected hyperliquid/historical-data, got %q", results[0].Slug)
	}
}

func TestDocsGetSupportsSlugAndURLInputs(t *testing.T) {
	llms := `# Dwellir Documentation

## Core Documentation
- [Authentication](https://www.dwellir.com/docs/authentication): Learn how to authenticate with tokens and profiles.
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/docs/llms.txt":
			_, _ = w.Write([]byte(llms))
		case "/docs/authentication.md":
			_, _ = w.Write([]byte("# Authentication\n\nUse CLI tokens."))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	docs := NewDocsAPIWithURLs(server.URL+"/docs/llms.txt", server.URL+"/docs", server.Client())

	bySlug, err := docs.Get("authentication")
	if err != nil {
		t.Fatalf("unexpected get by slug error: %v", err)
	}
	if bySlug.Title != "Authentication" {
		t.Fatalf("expected title Authentication, got %q", bySlug.Title)
	}
	if !strings.Contains(bySlug.Content, "Use CLI tokens") {
		t.Fatalf("expected markdown content, got %q", bySlug.Content)
	}

	byURL, err := docs.Get(server.URL + "/docs/authentication")
	if err != nil {
		t.Fatalf("unexpected get by URL error: %v", err)
	}
	if byURL.Slug != "authentication" {
		t.Fatalf("expected slug authentication, got %q", byURL.Slug)
	}
}

func TestDocsGetNotFoundReturnsTypedError(t *testing.T) {
	llms := `# Dwellir Documentation

## Core Documentation
- [Authentication](https://www.dwellir.com/docs/authentication): Learn how to authenticate with tokens and profiles.
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/docs/llms.txt":
			_, _ = w.Write([]byte(llms))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	docs := NewDocsAPIWithURLs(server.URL+"/docs/llms.txt", server.URL+"/docs", server.Client())
	_, err := docs.Get("missing-page")
	if !errors.Is(err, ErrDocsPageNotFound) {
		t.Fatalf("expected ErrDocsPageNotFound, got %v", err)
	}
}
