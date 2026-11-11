//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestFeedbackIsListedInHelp(t *testing.T) {
	help := runCLI(t, "--help")
	if !strings.Contains(help.stdout, "feedback") {
		t.Fatalf("help does not list feedback:\n%s", help.stdout)
	}
}

func TestFeedbackSendsOneEventToPostHog(t *testing.T) {
	var mu sync.Mutex
	var events []map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Usage telemetry goes to /batch/ in the background; only feedback uses the capture route.
		if r.URL.Path == "/i/v0/e/" {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			events = append(events, body)
			mu.Unlock()
		}
		_, _ = w.Write([]byte(`{"status":"Ok"}`))
	}))
	defer server.Close()

	env := map[string]string{
		"DWELLIR_API_URL":      "",
		"DWELLIR_POSTHOG_KEY":  "test-key",
		"DWELLIR_POSTHOG_HOST": server.URL,
	}
	res := runCLIWithEnv(t, env, "feedback", "--json", "--sentiment", "positive", "--area", "cli",
		"--message", "Feedback from the terminal is handy.", "--context", "Trying the feedback command.")
	if res.exitCode != 0 {
		t.Fatalf("exit %d:\n%s%s", res.exitCode, res.stdout, res.stderr)
	}
	data, _ := parseJSON(t, res.stdout)["data"].(map[string]interface{})

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 1 {
		t.Fatalf("PostHog received %d feedback events, want 1", len(events))
	}
	props, _ := events[0]["properties"].(map[string]interface{})
	if events[0]["event"] != "cli_feedback" || events[0]["uuid"] != data["id"] ||
		props["message"] != "Feedback from the terminal is handy." || props["area"] != "cli" {
		t.Fatalf("event = %v, response data = %v", events[0], data)
	}
}

func TestFeedbackValidationErrorExplainsTheRetry(t *testing.T) {
	res := runCLI(t, "feedback", "--json", "--sentiment", "positive")
	if res.exitCode != 4 {
		t.Fatalf("exit %d, want 4:\n%s", res.exitCode, res.stdout)
	}
	body, _ := parseJSON(t, res.stdout)["error"].(map[string]interface{})
	help, _ := body["help"].(string)
	if !strings.Contains(help, "Retry: dwellir feedback") {
		t.Fatalf("error = %v", body)
	}
}
