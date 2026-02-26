package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dwellir-public/cli/internal/api"
)

func TestJSONSuccess(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(&buf)
	err := f.Success("keys.list", map[string]string{"count": "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if got == "" {
		t.Fatal("expected non-empty output")
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"ok":true`)) {
		t.Errorf("expected ok:true in output, got: %s", got)
	}
}

func TestJSONError(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(&buf)
	err := f.Error("not_authenticated", "No token found.", "Run 'dwellir auth login'")
	if err == nil {
		t.Fatal("expected formatter to return non-nil error for error responses")
	}
	got := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte(`"ok":false`)) {
		t.Errorf("expected ok:false in output, got: %s", got)
	}
}

func TestHumanSuccess(t *testing.T) {
	var buf bytes.Buffer
	f := NewHumanFormatter(&buf)
	err := f.Success("keys.list", []api.APIKey{
		{
			APIKey:       "abc-123",
			Name:         "test-key",
			Enabled:      true,
			DailyQuota:   nil,
			MonthlyQuota: nil,
			CreatedAt:    "2026-02-26T00:00:00Z",
			UpdatedAt:    "2026-02-26T00:00:00Z",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if got == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(got, "API KEY") || !strings.Contains(got, "test-key") {
		t.Fatalf("expected table output with key data, got:\n%s", got)
	}
}

func TestHumanUsageSummary(t *testing.T) {
	var buf bytes.Buffer
	f := NewHumanFormatter(&buf)

	err := f.Success("usage.summary", &api.UsageSummary{
		TotalRequests:  100,
		TotalResponses: 95,
		RateLimited:    5,
		BillingStart:   "2026-02-01",
		BillingEnd:     "2026-02-29",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "Total requests") || !strings.Contains(got, "100") {
		t.Fatalf("expected key/value output, got:\n%s", got)
	}
	if strings.Contains(got, "FIELD") || strings.Contains(got, "VALUE") {
		t.Fatalf("expected key/value output without generic headers, got:\n%s", got)
	}
}

func TestHumanWriteMapDoesNotShowGenericHeaders(t *testing.T) {
	var buf bytes.Buffer
	f := NewHumanFormatter(&buf)

	err := f.Success("version", map[string]string{
		"version": "0.1.3",
		"commit":  "ce3431d",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "Version") || !strings.Contains(got, "0.1.3") {
		t.Fatalf("expected key/value data, got:\n%s", got)
	}
	if strings.Contains(got, "FIELD") || strings.Contains(got, "VALUE") {
		t.Fatalf("expected key/value output without generic headers, got:\n%s", got)
	}
}

func TestHumanDocsGetMarkdown(t *testing.T) {
	var buf bytes.Buffer
	f := NewHumanFormatter(&buf)

	err := f.Success("docs.get", api.DocsPage{
		Title:   "Authentication",
		Slug:    "authentication",
		URL:     "https://www.dwellir.com/docs/authentication",
		Content: "# Authentication\n\nUse tokens.\n",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "Authentication") || !strings.Contains(got, "Use tokens.") {
		t.Fatalf("expected rendered markdown output, got:\n%s", got)
	}
}

func TestHumanLogsFacets(t *testing.T) {
	var buf bytes.Buffer
	f := NewHumanFormatter(&buf)

	err := f.Success("logs.facets", &api.ErrorFacets{
		FQDNs: []api.FacetEntry{
			{Value: "eth-mainnet.dwellir.com", Count: 12},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "FQDNs") || !strings.Contains(got, "eth-mainnet.dwellir.com") {
		t.Fatalf("expected facet section output, got:\n%s", got)
	}
}

func TestHumanEndpointsUsesProtocolColumn(t *testing.T) {
	var buf bytes.Buffer
	f := NewHumanFormatter(&buf)

	err := f.Success("endpoints.search", []api.Chain{
		{
			Name:      "Base",
			Ecosystem: "evm",
			Networks: []api.Network{
				{
					Name: "Mainnet",
					Nodes: []api.Node{
						{
							NodeType: api.NodeType{Name: "archive"},
							HTTPS:    "https://api-base-mainnet-archive.n.dwellir.com/<key>",
							WSS:      "wss://api-base-mainnet-archive.n.dwellir.com/<key>",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "PROTOCOL") || !strings.Contains(got, "ENDPOINT") {
		t.Fatalf("expected protocol/endpoint headers, got:\n%s", got)
	}
	if strings.Contains(got, "HTTPS") || strings.Contains(got, "WSS") {
		t.Fatalf("expected no dedicated HTTPS/WSS columns, got:\n%s", got)
	}
	if !strings.Contains(got, "https://api-base-mainnet-archive.n.dwellir.com/") {
		t.Fatalf("expected https endpoint row, got:\n%s", got)
	}
	if !strings.Contains(got, "wss://api-base-mainnet-archive.n.dwellir.com/") {
		t.Fatalf("expected wss endpoint row, got:\n%s", got)
	}
}

func TestEndpointColumnWidth(t *testing.T) {
	tests := []struct {
		name     string
		terminal int
		want     int
	}{
		{name: "default when terminal unknown", terminal: 0, want: 48},
		{name: "minimum width clamp", terminal: 70, want: 18},
		{name: "dynamic width", terminal: 120, want: 58},
		{name: "maximum width clamp", terminal: 200, want: 64},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := endpointColumnWidth(tc.terminal)
			if got != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, got)
			}
		})
	}
}
