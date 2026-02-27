package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUsageHistoryRequestShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/user/analytics" {
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
		filter, ok := body["filter"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected filter object in request body, got %v", body)
		}
		if _, ok := filter["api_keys"]; !ok {
			t.Fatalf("expected api_keys in filter, got %v", filter)
		}
		if _, ok := filter["domains"]; !ok {
			t.Fatalf("expected domains in filter, got %v", filter)
		}

		_ = json.NewEncoder(w).Encode([]UsageHistory{})
	}))
	defer server.Close()

	api := NewUsageAPI(NewClient(server.URL, "token"))
	_, err := api.History(
		"day",
		"2026-02-01T00:00:00Z",
		"2026-02-02T00:00:00Z",
		"key-1",
		"api-base-mainnet.n.dwellir.com",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUsageHistoryMapsStartTimeToTimestamp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/user/analytics" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		payload := []map[string]interface{}{
			{
				"start_time": "2026-02-27T00:00:00Z",
				"domain":     "api-base-mainnet.n.dwellir.com",
				"method":     "eth_chainId",
				"requests":   10,
				"responses":  9,
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	api := NewUsageAPI(NewClient(server.URL, "token"))
	items, err := api.History("day", "", "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one usage row, got %d", len(items))
	}
	if items[0].Timestamp != "2026-02-27T00:00:00Z" {
		t.Fatalf("expected timestamp to map from start_time, got %q", items[0].Timestamp)
	}
	if items[0].Domain == "" {
		t.Fatalf("expected domain field to be mapped")
	}
}

func TestUsageRPSBuildsTimeSeriesFromHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/user/analytics" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		payload := []map[string]interface{}{
			{
				"start_time": "2026-02-27T00:00:00Z",
				"requests":   120,
				"responses":  100,
			},
			{
				"start_time": "2026-02-27T00:00:00Z",
				"requests":   60,
				"responses":  50,
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	api := NewUsageAPI(NewClient(server.URL, "token"))
	items, err := api.RPS("minute", "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one row output, got %d", len(items))
	}
	if items[0].RPS != 3 {
		t.Fatalf("expected rps=3, got %v", items[0].RPS)
	}
}
