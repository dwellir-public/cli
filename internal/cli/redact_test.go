package cli

import (
	"testing"

	"github.com/dwellir-public/cli/internal/api"
)

func TestMaskAPIKey(t *testing.T) {
	t.Cleanup(func() { showSecrets = false })

	cases := map[string]string{
		"":                                     "",
		"short":                                "****",
		"0123456789ab":                         "****",
		"9f1c2d3e-4b5a-6789-abcd-ef0123456789": "9f1c****6789",
	}
	for in, want := range cases {
		if got := maskAPIKey(in); got != want {
			t.Errorf("maskAPIKey(%q) = %q, want %q", in, got, want)
		}
	}

	showSecrets = true
	if got := maskAPIKey("9f1c2d3e-4b5a-6789-abcd-ef0123456789"); got != "9f1c2d3e-4b5a-6789-abcd-ef0123456789" {
		t.Errorf("expected full key with --show-secrets, got %q", got)
	}
}

func TestMaskUsageAndLogRows(t *testing.T) {
	key := "9f1c2d3e-4b5a-6789-abcd-ef0123456789"
	history := maskUsageHistory([]api.UsageHistory{{APIKey: key, APIKeyName: "ci"}})
	logs := maskErrorLogs([]api.ErrorLog{{APIKey: key}})
	facets := maskErrorFacets(&api.ErrorFacets{APIKeys: []api.FacetEntry{{Value: key, Count: 3}}})

	for name, got := range map[string]string{
		"usage history": history[0].APIKey,
		"error log":     logs[0].APIKey,
		"error facet":   facets.APIKeys[0].Value,
	} {
		if got != "9f1c****6789" {
			t.Errorf("%s: expected masked key, got %q", name, got)
		}
	}
	if history[0].APIKeyName != "ci" {
		t.Errorf("expected key name to be kept, got %q", history[0].APIKeyName)
	}
}
