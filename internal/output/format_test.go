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
			APIKey:       api.NewSecret("abc-123"),
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

func TestJSONKeysDeleteCleanupPendingDoesNotExposeEndpoints(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(&buf)

	err := f.Success("keys.delete", map[string]interface{}{
		"key":    "abc-123",
		"status": "cleanup_pending",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, `"status":"cleanup_pending"`) {
		t.Fatalf("expected cleanup_pending status, got:\n%s", got)
	}
	if strings.Contains(got, "unreachable_endpoints") {
		t.Fatalf("did not expect unreachable_endpoints in output, got:\n%s", got)
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

func TestHumanDocsSearchIncludesURL(t *testing.T) {
	var buf bytes.Buffer
	f := NewHumanFormatter(&buf)

	err := f.Success("docs.search", []api.DocsEntry{
		{
			Title:       "Dwellir CLI",
			Slug:        "cli",
			Section:     "Networks and Guides",
			Description: "Install and use the Dwellir CLI.",
			URL:         "https://www.dwellir.com/docs/cli",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "https://www.dwellir.com/docs/cli") {
		t.Fatalf("expected docs URL to be included in human output, got:\n%s", got)
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

func TestJSONPageReportsCursorInMeta(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(&buf)
	err := f.Success("logs.errors", Page{
		Items:      []api.ErrorLog{{RequestID: "req-1"}},
		NextCursor: "cursor-2",
		HasMore:    true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	for _, want := range []string{`"data":[{`, `"request_id":"req-1"`, `"next_cursor":"cursor-2"`, `"has_more":true`} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %s in output, got: %s", want, got)
		}
	}
}

func TestJSONLastPageReportsHasMoreFalse(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(&buf)
	if err := f.Success("logs.errors", Page{Items: []api.ErrorLog{}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, `"has_more":false`) || strings.Contains(got, "next_cursor") {
		t.Fatalf("expected has_more false and no cursor, got: %s", got)
	}
}

func TestHumanPagePrintsNextCursor(t *testing.T) {
	var buf bytes.Buffer
	f := NewHumanFormatter(&buf)
	err := f.Success("logs.errors", Page{
		Items:      []api.ErrorLog{{Timestamp: "2026-10-11T00:00:00Z", StatusCode: 429, ErrorMessage: "slow down"}},
		NextCursor: "cursor-2",
		HasMore:    true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "slow down") || !strings.Contains(got, "--cursor cursor-2") {
		t.Fatalf("expected log row and next cursor hint, got:\n%s", got)
	}
}

func TestFormattersMaskTypedKeyFields(t *testing.T) {
	const full = "9f1c2d3e-4b5a-6789-abcd-ef0123456789"
	key := api.NewSecret(full)
	payloads := map[string]interface{}{
		"keys.list":     []api.APIKey{{APIKey: key, Name: "ci"}},
		"keys.create":   &api.APIKey{APIKey: key, Name: "ci"},
		"usage.history": []api.UsageHistory{{APIKey: key, Requests: 1}},
		"logs.errors":   Page{Items: []api.ErrorLog{{APIKey: key, APIKeyName: "ci"}}},
		"logs.facets":   &api.ErrorFacets{APIKeys: []api.KeyFacetEntry{{Value: key, Name: "ci", Count: 2}}},
	}
	formatters := map[string]func(*bytes.Buffer) Formatter{
		"json":  func(b *bytes.Buffer) Formatter { return NewJSONFormatter(b) },
		"toon":  func(b *bytes.Buffer) Formatter { return NewTOONFormatter(b) },
		"human": func(b *bytes.Buffer) Formatter { return NewHumanFormatter(b) },
	}
	for command, data := range payloads {
		for name, newFormatter := range formatters {
			var buf bytes.Buffer
			if err := newFormatter(&buf).Success(command, data); err != nil {
				t.Fatalf("%s %s: unexpected error: %v", name, command, err)
			}
			got := buf.String()
			if strings.Contains(got, full) {
				t.Errorf("%s %s: output contains the full key:\n%s", name, command, got)
			}
			// Human tables for usage and logs do not show a key column.
			if name != "human" && !strings.Contains(got, "9f1c****6789") {
				t.Errorf("%s %s: expected the masked key, got:\n%s", name, command, got)
			}
		}
	}
}

func TestNewMasksKeysOutsideTypedFields(t *testing.T) {
	seen := api.NewSecret("7a1d2c3b-0000-1111-2222-333344445555")
	unseen := "0f9e8d7c-aaaa-bbbb-cccc-ddddeeeeffff"
	for _, format := range []string{"json", "toon", "human"} {
		var buf bytes.Buffer
		f := New(format, &buf)
		if err := f.Success("logs.errors", map[string]string{"message": "upstream rejected " + seen.Reveal()}); err != nil {
			t.Fatalf("%s: unexpected error: %v", format, err)
		}
		_ = f.Error("not_found", "No endpoints found for '"+unseen+"'.", "Retry "+seen.Reveal())
		got := buf.String()
		if strings.Contains(got, seen.Reveal()) || strings.Contains(got, unseen) {
			t.Errorf("%s: output contains a full key:\n%s", format, got)
		}
		if !strings.Contains(got, "7a1d****5555") || !strings.Contains(got, "0f9e****ffff") {
			t.Errorf("%s: expected masked keys, got:\n%s", format, got)
		}
	}
}

func TestNoticeGoesToMetaOrAfterHumanOutput(t *testing.T) {
	data := Notice{Data: Page{Items: []api.ErrorLog{{RequestID: "req-1"}}, NextCursor: "c2", HasMore: true}, Text: "Use keys copy."}

	var buf bytes.Buffer
	if err := NewJSONFormatter(&buf).Success("logs.errors", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{`"data":[{`, `"notice":"Use keys copy."`, `"next_cursor":"c2"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("expected %s in JSON output, got: %s", want, buf.String())
		}
	}

	buf.Reset()
	if err := NewHumanFormatter(&buf).Success("logs.errors", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); !strings.HasSuffix(got, "--cursor c2\n\nNote: Use keys copy.\n") {
		t.Errorf("expected the notice after the rows and cursor, got:\n%s", got)
	}
}
