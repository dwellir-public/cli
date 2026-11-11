package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type capturedFeedback struct {
	APIKey     string                 `json:"api_key"`
	Event      string                 `json:"event"`
	DistinctID string                 `json:"distinct_id"`
	UUID       string                 `json:"uuid"`
	Properties map[string]interface{} `json:"properties"`
}

func feedbackServer(t *testing.T, status int) (*httptest.Server, *[]capturedFeedback) {
	t.Helper()
	var got []capturedFeedback
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/i/v0/e/" {
			t.Errorf("request %s %s, want POST /i/v0/e/", r.Method, r.URL.Path)
		}
		var body capturedFeedback
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		got = append(got, body)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"status":"Ok"}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("DWELLIR_API_URL", "")
	t.Setenv("DWELLIR_POSTHOG_KEY", "test-key")
	t.Setenv("DWELLIR_POSTHOG_HOST", server.URL)
	t.Cleanup(func() { client = nil })
	return server, &got
}

func TestSendFeedbackCapturesOneEventUnderTheUser(t *testing.T) {
	_, got := feedbackServer(t, http.StatusOK)
	Init("1.2.3", "user-123", "42", "Acme", "device", false)

	err := SendFeedback(context.Background(), "0190-feedback", map[string]interface{}{"message": "Docs search is quick."})
	if err != nil {
		t.Fatalf("SendFeedback() error = %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("captured %d requests, want 1", len(*got))
	}
	ev := (*got)[0]
	if ev.APIKey != "test-key" || ev.Event != FeedbackEvent || ev.DistinctID != "user-123" || ev.UUID != "0190-feedback" {
		t.Fatalf("event = %+v", ev)
	}
	groups, _ := ev.Properties["$groups"].(map[string]interface{})
	if ev.Properties["message"] != "Docs search is quick." || ev.Properties["version"] != "1.2.3" || groups["organization"] != "42" {
		t.Fatalf("properties = %v", ev.Properties)
	}
}

func TestSendFeedbackAnonymousOmitsUserAndOrg(t *testing.T) {
	_, got := feedbackServer(t, http.StatusOK)
	Init("1.2.3", "user-123", "42", "Acme", "device", true)

	if err := SendFeedback(context.Background(), "id", nil); err != nil {
		t.Fatalf("SendFeedback() error = %v", err)
	}
	ev := (*got)[0]
	if ev.DistinctID != "anon:device" || ev.Properties["org_id"] != nil || ev.Properties["$groups"] != nil {
		t.Fatalf("anonymous event leaks identity: %+v", ev)
	}
}

func TestSendFeedbackReportsRejection(t *testing.T) {
	feedbackServer(t, http.StatusBadRequest)
	Init("1.2.3", "user-123", "42", "Acme", "device", false)

	err := SendFeedback(context.Background(), "id", nil)
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("SendFeedback() error = %v, want HTTP 400", err)
	}
}

func TestSendFeedbackUnavailableOutsideProduction(t *testing.T) {
	tests := []struct {
		name, apiURL, key string
	}{
		{"no key in this build", "", ""},
		{"staging API", "https://marly-staging.dwellir.com", "test-key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := feedbackServer(t, http.StatusOK)
			t.Setenv("DWELLIR_API_URL", tt.apiURL)
			t.Setenv("DWELLIR_POSTHOG_KEY", tt.key)

			err := SendFeedback(context.Background(), "id", nil)
			if !errors.Is(err, ErrFeedbackUnavailable) || len(*got) != 0 {
				t.Fatalf("SendFeedback() error = %v, requests = %d; want ErrFeedbackUnavailable and no request", err, len(*got))
			}
		})
	}
}
