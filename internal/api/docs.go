package api

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
)

const (
	defaultDocsIndexURL = "https://www.dwellir.com/docs/llms.txt"
	defaultDocsBaseURL  = "https://www.dwellir.com/docs"
)

var (
	llmsLinkPattern     = regexp.MustCompile(`^- \[(.+?)\]\(([^)]+)\)(?::\s*(.*))?$`)
	heading1Pattern     = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
	ErrDocsPageNotFound = errors.New("documentation page not found")
)

// DocsEntry describes a single docs page indexed by llms.txt.
type DocsEntry struct {
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Section     string `json:"section,omitempty"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
	MarkdownURL string `json:"markdown_url"`
}

// DocsPage contains full markdown content for a single docs page.
type DocsPage struct {
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	URL         string `json:"url"`
	MarkdownURL string `json:"markdown_url"`
	Content     string `json:"content"`
}

type DocsAPI struct {
	httpClient *http.Client
	indexURL   string
	docsBase   string
}

func NewDocsAPI() *DocsAPI {
	return NewDocsAPIWithURLs(defaultDocsIndexURL, defaultDocsBaseURL, nil)
}

func NewDocsAPIWithURLs(indexURL, docsBaseURL string, client *http.Client) *DocsAPI {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	return &DocsAPI{
		httpClient: client,
		indexURL:   strings.TrimSpace(indexURL),
		docsBase:   strings.TrimSuffix(strings.TrimSpace(docsBaseURL), "/"),
	}
}

func (d *DocsAPI) List() ([]DocsEntry, error) {
	body, statusCode, err := d.fetchText(d.indexURL)
	if err != nil {
		return nil, err
	}
	if statusCode >= 400 {
		return nil, fmt.Errorf("fetching docs index (%d)", statusCode)
	}
	return parseLLMSIndex(body, d.docsBase)
}

func (d *DocsAPI) Search(query string, limit int) ([]DocsEntry, error) {
	entries, err := d.List()
	if err != nil {
		return nil, err
	}

	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(terms) == 0 {
		return applyDocsLimit(entries, limit), nil
	}

	type scored struct {
		entry DocsEntry
		score int
	}

	scoredEntries := make([]scored, 0, len(entries))
	for _, entry := range entries {
		score := scoreDocsEntry(entry, terms)
		if score == 0 {
			continue
		}
		scoredEntries = append(scoredEntries, scored{entry: entry, score: score})
	}

	sort.Slice(scoredEntries, func(i, j int) bool {
		if scoredEntries[i].score == scoredEntries[j].score {
			return scoredEntries[i].entry.Title < scoredEntries[j].entry.Title
		}
		return scoredEntries[i].score > scoredEntries[j].score
	})

	results := make([]DocsEntry, 0, len(scoredEntries))
	for _, item := range scoredEntries {
		results = append(results, item.entry)
	}
	return applyDocsLimit(results, limit), nil
}

func (d *DocsAPI) Get(query string) (DocsPage, error) {
	slug, err := parseDocsSlug(query)
	if err != nil {
		return DocsPage{}, err
	}

	mdURL := docsMarkdownURL(d.docsBase, slug)
	content, statusCode, err := d.fetchText(mdURL)
	if err != nil {
		return DocsPage{}, err
	}
	if statusCode == http.StatusNotFound {
		return DocsPage{}, fmt.Errorf("%w: %s", ErrDocsPageNotFound, slug)
	}
	if statusCode >= 400 {
		return DocsPage{}, fmt.Errorf("fetching docs page (%d)", statusCode)
	}

	title := pageTitle(content)
	if title == "" {
		title = titleFromSlug(slug)
	}

	return DocsPage{
		Title:       title,
		Slug:        slug,
		URL:         strings.TrimSuffix(d.docsBase, "/") + "/" + slug,
		MarkdownURL: mdURL,
		Content:     content,
	}, nil
}

func (d *DocsAPI) fetchText(rawURL string) (string, int, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "text/markdown, text/plain;q=0.9, */*;q=0.1")
	req.Header.Set("User-Agent", "dwellir-cli")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, fmt.Errorf("reading response: %w", err)
	}

	return string(body), resp.StatusCode, nil
}

func parseLLMSIndex(content, docsBase string) ([]DocsEntry, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	entries := []DocsEntry{}
	section := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			section = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			continue
		}
		m := llmsLinkPattern.FindStringSubmatch(line)
		if len(m) == 0 {
			continue
		}

		title := strings.TrimSpace(m[1])
		ref := strings.TrimSpace(m[2])
		description := strings.TrimSpace(m[3])

		slug, err := parseDocsSlug(ref)
		if err != nil {
			continue
		}

		entries = append(entries, DocsEntry{
			Title:       title,
			Slug:        slug,
			Section:     section,
			Description: description,
			URL:         strings.TrimSuffix(docsBase, "/") + "/" + slug,
			MarkdownURL: docsMarkdownURL(docsBase, slug),
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parsing llms index: %w", err)
	}
	return entries, nil
}

func scoreDocsEntry(entry DocsEntry, terms []string) int {
	title := strings.ToLower(entry.Title)
	slug := strings.ToLower(entry.Slug)
	desc := strings.ToLower(entry.Description)
	section := strings.ToLower(entry.Section)

	score := 0
	for _, term := range terms {
		matched := false
		if strings.Contains(title, term) {
			score += 6
			matched = true
		}
		if strings.Contains(slug, term) {
			score += 5
			matched = true
		}
		if strings.Contains(desc, term) {
			score += 4
			matched = true
		}
		if strings.Contains(section, term) {
			score += 2
			matched = true
		}
		if !matched {
			return 0
		}
	}
	return score
}

func applyDocsLimit(entries []DocsEntry, limit int) []DocsEntry {
	if limit > 0 && len(entries) > limit {
		return entries[:limit]
	}
	return entries
}

func docsMarkdownURL(docsBase, slug string) string {
	base := strings.TrimSuffix(docsBase, "/")
	return base + "/" + slug + ".md"
}

func parseDocsSlug(input string) (string, error) {
	candidate := strings.TrimSpace(input)
	if candidate == "" {
		return "", fmt.Errorf("docs page cannot be empty")
	}

	if u, err := url.Parse(candidate); err == nil && u.Scheme != "" && u.Host != "" {
		candidate = u.Path
	}

	candidate = strings.TrimSpace(candidate)
	candidate = strings.TrimPrefix(candidate, "/")
	candidate = strings.TrimPrefix(candidate, "docs/")
	candidate = strings.TrimSuffix(candidate, "/")
	candidate = strings.TrimSuffix(candidate, ".md")

	candidate = strings.TrimPrefix(path.Clean("/"+candidate), "/")
	if candidate == "" || candidate == "." {
		return "", fmt.Errorf("invalid docs page: %q", input)
	}
	if strings.HasPrefix(candidate, "../") || strings.Contains(candidate, "/../") {
		return "", fmt.Errorf("invalid docs page: %q", input)
	}
	return candidate, nil
}

func pageTitle(markdown string) string {
	matches := heading1Pattern.FindStringSubmatch(markdown)
	if len(matches) != 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func titleFromSlug(slug string) string {
	part := path.Base(slug)
	if part == "" || part == "." || part == "/" {
		return slug
	}
	words := strings.Split(strings.ReplaceAll(part, "-", " "), " ")
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}
