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

func TestFeedbackIsUnknownCommandWhenTheGateIsOff(t *testing.T) {
	res := runCLIWithEnv(t, map[string]string{"DWELLIR_FEEDBACK": "0"}, "feedback", "--json")
	if res.exitCode == 0 || !strings.Contains(res.stdout+res.stderr, "unknown command") {
		t.Fatalf("exit %d, want an unknown command error:\n%s%s", res.exitCode, res.stdout, res.stderr)
	}
	help := runCLIWithEnv(t, map[string]string{"DWELLIR_FEEDBACK": "0"}, "--help")
	if strings.Contains(help.stdout, "feedback") {
		t.Fatalf("help lists feedback with the gate off:\n%s", help.stdout)
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
		"DWELLIR_FEEDBACK":     "1",
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

func TestFeedbackValidationExitCode(t *testing.T) {
	res := runCLIWithEnv(t, map[string]string{"DWELLIR_FEEDBACK": "1"}, "feedback", "--json", "--sentiment", "positive")
	if res.exitCode != 4 || parseJSON(t, res.stdout)["ok"] != false {
		t.Fatalf("exit %d, want 4:\n%s", res.exitCode, res.stdout)
	}
}

func TestDoctorReportsTheFeedbackGate(t *testing.T) {
	for _, env := range []string{"0", "1"} {
		res := runCLIWithEnv(t, map[string]string{"DWELLIR_FEEDBACK": env}, "doctor", "--json")
		check := findDoctorCheck(t, parseJSON(t, res.stdout), "feature_gates")
		details, _ := check["details"].(map[string]interface{})
		if details["feedback_enabled"] != (env == "1") || details["feedback_env"] != env {
			t.Fatalf("DWELLIR_FEEDBACK=%s: feature_gates details = %v", env, details)
		}
	}
}
