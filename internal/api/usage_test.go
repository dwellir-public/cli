package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUsageHistoryRequestShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/organization/analytics" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if _, ok := body["start_time"]; !ok {
			t.Fatalf("expected start_time in request body, got %v", body)
		}
		if _, ok := body["end_time"]; !ok {
			t.Fatalf("expected end_time in request body, got %v", body)
		}
		if _, ok := body["limit"]; !ok {
			t.Fatalf("expected limit in request body, got %v", body)
		}

		_ = json.NewEncoder(w).Encode([]UsageHistory{})
	}))
	defer server.Close()

	api := NewUsageAPI(NewClient(server.URL, "token"))
	_, err := api.History("day", "2026-02-01T00:00:00Z", "2026-02-02T00:00:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUsageRPSRequestShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/organization/analytics/rps" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if _, ok := body["interval"]; !ok {
			t.Fatalf("expected interval in request body, got %v", body)
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{"rps": 42.5})
	}))
	defer server.Close()

	api := NewUsageAPI(NewClient(server.URL, "token"))
	items, err := api.RPS()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected single row output, got %d", len(items))
	}
	if items[0].RPS != 42.5 {
		t.Fatalf("expected rps=42.5, got %v", items[0].RPS)
	}
}
