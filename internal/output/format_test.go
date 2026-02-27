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

func TestHumanUsageSummaryFormatsLargeNumbers(t *testing.T) {
	var buf bytes.Buffer
	f := NewHumanFormatter(&buf)

	err := f.Success("usage.summary", &api.UsageSummary{
		TotalRequests:  185143770,
		TotalResponses: 35223132,
		RateLimited:    193949495,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "185,143,770") {
		t.Fatalf("expected formatted total requests with commas, got:\n%s", got)
	}
	if !strings.Contains(got, "35,223,132") {
		t.Fatalf("expected formatted total responses with commas, got:\n%s", got)
	}
	if !strings.Contains(got, "193,949,495") {
		t.Fatalf("expected formatted rate limited with commas, got:\n%s", got)
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
	if strings.Contains(got, "│") || strings.Contains(got, "|") {
		t.Fatalf("expected no vertical separators in human table output, got:\n%s", got)
	}
	if !strings.Contains(got, "https://api-base-mainnet-archive.n.dwellir.com/") {
		t.Fatalf("expected https endpoint row, got:\n%s", got)
	}
	if !strings.Contains(got, "wss://api-base-mainnet-archive.n.dwellir.com/") {
		t.Fatalf("expected wss endpoint row, got:\n%s", got)
	}
}

func TestHumanUsageMethodsBreakdown(t *testing.T) {
	var buf bytes.Buffer
	f := NewHumanFormatter(&buf)

	err := f.Success("usage.methods", []api.UsageBreakdown{
		{Group: "eth_call", Requests: 10, Responses: 9, RateLimited: 1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "eth_call") || !strings.Contains(got, "RATE LIMITED") {
		t.Fatalf("expected usage methods breakdown output, got:\n%s", got)
	}
}

func TestHumanEndpointsShowsPremiumStatusAndTrialExpiry(t *testing.T) {
	var buf bytes.Buffer
	f := NewHumanFormatter(&buf)

	err := f.Success("endpoints.search", []api.Chain{
		{
			Name:      "Hyperliquid HyperCore Orderbook",
			Ecosystem: "hyperliquid",
			Networks: []api.Network{
				{
					Name: "Mainnet",
					Nodes: []api.Node{
						{
							NodeType:      api.NodeType{Name: "Full"},
							HTTPS:         "[locked premium endpoint]",
							Premium:       true,
							PremiumStatus: "locked",
						},
					},
				},
				{
					Name: "Mainnet",
					Nodes: []api.Node{
						{
							NodeType:      api.NodeType{Name: "Full"},
							HTTPS:         "https://api-hyperliquid-mainnet-orderbook.n.dwellir.com/<key>/ws",
							Premium:       true,
							PremiumStatus: "trial-active",
							TrialEndsAt:   "2026-03-01T00:00:00Z",
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
	if !strings.Contains(got, "PREMIUM") {
		t.Fatalf("expected premium column/header, got:\n%s", got)
	}
	if !strings.Contains(strings.ToLower(got), "locked") {
		t.Fatalf("expected locked premium label, got:\n%s", got)
	}
	if !strings.Contains(strings.ToLower(got), "trial") || !strings.Contains(strings.ToLower(got), "until") {
		t.Fatalf("expected trial-until label, got:\n%s", got)
	}
}

func TestTOONSuccess(t *testing.T) {
	var buf bytes.Buffer
	f := New("toon", &buf)

	err := f.Success("version", map[string]string{"version": "0.1.10"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "ok: true") {
		t.Fatalf("expected TOON response envelope, got:\n%s", got)
	}
	if !strings.Contains(got, "command: version") {
		t.Fatalf("expected TOON metadata command field, got:\n%s", got)
	}
	if strings.Contains(got, "{") {
		t.Fatalf("expected TOON (non-JSON) output, got:\n%s", got)
	}
}
