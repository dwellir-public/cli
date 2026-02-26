//go:build e2e

package e2e

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUsageHistoryShowsAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v4/organization/analytics" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"detail":"forbidden"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	res := runCLIWithEnv(t, map[string]string{
		"DWELLIR_TOKEN":   "test-token",
		"DWELLIR_API_URL": server.URL,
	}, "usage", "history", "--interval", "day", "--human")

	if res.exitCode == 0 {
		t.Fatalf("expected non-zero exit code, got stdout=%s stderr=%s", res.stdout, res.stderr)
	}
	if !strings.Contains(res.stdout, "Error:") {
		t.Fatalf("expected formatted error output, got stdout=%q stderr=%q", res.stdout, res.stderr)
	}
	if !strings.Contains(res.stdout, "HTTP 403") {
		t.Fatalf("expected HTTP status in formatted error output, got stdout=%q", res.stdout)
	}
}
