package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogsErrorsResponseWrapper(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/organization/logs/errors" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		filter := body["filter"].(map[string]interface{})
		if _, ok := filter["api_keys"]; !ok {
			t.Fatalf("expected api_keys filter, got %v", body)
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"timestamp":             "2026-02-26T00:00:00Z",
					"request_id":            "req-1",
					"api_key":               "key-1",
					"fqdn":                  "example.com",
					"response_status_code":  500,
					"response_status_label": "Internal Server Error",
					"request_rpc_methods":   "eth_call",
					"request_http_method":   "POST",
					"error_message":         "boom",
					"backend_latency_ms":    10,
					"total_latency_ms":      15,
				},
			},
			"has_more": false,
		})
	}))
	defer server.Close()

	api := NewLogsAPI(NewClient(server.URL, "token"))
	logs, err := api.Errors(map[string]interface{}{"api_key": "key-1", "limit": 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log item, got %d", len(logs))
	}
}

func TestLogsStatsEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/organization/logs/error-classes" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{"status_code": 500, "status_label": "Internal Server Error", "count": 7},
			},
		})
	}))
	defer server.Close()

	api := NewLogsAPI(NewClient(server.URL, "token"))
	stats, err := api.Stats(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats) != 1 || stats[0].StatusCode != 500 || stats[0].Count != 7 {
		t.Fatalf("unexpected stats payload: %+v", stats)
	}
}

func TestLogsFacetsEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/organization/logs/error-facets" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"fqdns": []map[string]interface{}{
				{"fqdn": "eth-mainnet.dwellir.com", "count": 3},
			},
			"rpc_methods": []map[string]interface{}{
				{"rpc_method": "eth_call", "count": 9},
			},
		})
	}))
	defer server.Close()

	api := NewLogsAPI(NewClient(server.URL, "token"))
	facets, err := api.Facets(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(facets.FQDNs) != 1 || facets.FQDNs[0].Value != "eth-mainnet.dwellir.com" {
		t.Fatalf("unexpected fqdn facets: %+v", facets.FQDNs)
	}
	if len(facets.RPCMethods) != 1 || facets.RPCMethods[0].Value != "eth_call" {
		t.Fatalf("unexpected rpc facets: %+v", facets.RPCMethods)
	}
}
