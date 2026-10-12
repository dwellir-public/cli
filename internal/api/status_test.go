package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestNewStatusFilterNormalizes(t *testing.T) {
	tests := []struct {
		name      string
		chains    []string
		networks  []string
		endpoints []string
		want      StatusFilter
	}{
		{
			name: "empty",
			want: StatusFilter{Chains: []string{}, Networks: []string{}, Endpoints: []string{}},
		},
		{
			name:      "lowercases, trims, splits commas, and drops duplicates",
			chains:    []string{" Base ", "ethereum,BASE", ""},
			networks:  []string{"Ethereum/Mainnet"},
			endpoints: []string{"API-Base-Mainnet.n.dwellir.com"},
			want: StatusFilter{
				Chains:    []string{"base", "ethereum"},
				Networks:  []string{"ethereum/mainnet"},
				Endpoints: []string{"api-base-mainnet.n.dwellir.com"},
			},
		},
		{
			name: "reduces endpoint URLs to the host",
			endpoints: []string{
				"https://api-base-mainnet.n.dwellir.com/abc-key",
				"wss://api-ethereum-mainnet.n.dwellir.com:443/key",
				"api-polygon-mainnet.n.dwellir.com/key",
			},
			want: StatusFilter{
				Chains:   []string{},
				Networks: []string{},
				Endpoints: []string{
					"api-base-mainnet.n.dwellir.com",
					"api-ethereum-mainnet.n.dwellir.com",
					"api-polygon-mainnet.n.dwellir.com",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewStatusFilter(tt.chains, tt.networks, tt.endpoints)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NewStatusFilter() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestStatusFilterMatches(t *testing.T) {
	filter := NewStatusFilter([]string{"base"}, []string{"ethereum/mainnet"}, []string{"api-polygon-mainnet.n.dwellir.com"})
	tests := []struct {
		name   string
		filter StatusFilter
		chain  string
		ref    string
		fqdn   string
		want   bool
	}{
		{name: "empty filter matches all", filter: StatusFilter{}, chain: "x", ref: "x/y", fqdn: "z", want: true},
		{name: "chain", filter: filter, chain: "base", ref: "base/sepolia", fqdn: "a", want: true},
		{name: "network", filter: filter, chain: "ethereum", ref: "ethereum/mainnet", fqdn: "a", want: true},
		{name: "other network on a named chain's sibling", filter: filter, chain: "ethereum", ref: "ethereum/sepolia", fqdn: "a", want: false},
		{name: "endpoint", filter: filter, chain: "polygon", ref: "polygon/mainnet", fqdn: "api-polygon-mainnet.n.dwellir.com", want: true},
		{name: "no match", filter: filter, chain: "lisk", ref: "lisk/mainnet", fqdn: "api-lisk-mainnet.n.dwellir.com", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Matches(tt.chain, tt.ref, tt.fqdn); got != tt.want {
				t.Fatalf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFeedURL(t *testing.T) {
	tests := []struct {
		name   string
		base   string
		format string
		filter StatusFilter
		want   string
	}{
		{name: "no filter", base: "https://status.dwellir.com", format: "atom", want: "https://status.dwellir.com/history.atom"},
		{name: "trailing slash", base: "https://status.dwellir.com/", format: "rss", want: "https://status.dwellir.com/history.rss"},
		{
			name:   "filters in chain, network, endpoint order",
			base:   "https://status.dwellir.com",
			format: "atom",
			filter: NewStatusFilter([]string{"base"}, []string{"ethereum/mainnet"}, []string{"api-polygon-mainnet.n.dwellir.com"}),
			want:   "https://status.dwellir.com/history.atom?chain=base&network=ethereum%2Fmainnet&endpoint=api-polygon-mainnet.n.dwellir.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FeedURL(tt.base, tt.format, tt.filter); got != tt.want {
				t.Fatalf("FeedURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseUnsubscribeToken(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "bare token", input: " abc123 ", want: "abc123"},
		{name: "unsubscribe URL", input: "https://status.dwellir.com/unsubscribe?token=abc123", want: "abc123"},
		{name: "escaped token", input: "https://status.dwellir.com/unsubscribe?token=a%2Bb", want: "a+b"},
		{name: "URL without token", input: "https://status.dwellir.com/unsubscribe", wantErr: true},
		{name: "empty", input: "  ", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUnsubscribeToken(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseUnsubscribeToken() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("ParseUnsubscribeToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUptimeChainParams(t *testing.T) {
	tests := []struct {
		name   string
		filter StatusFilter
		want   []string
	}{
		{name: "no filter fetches every chain", filter: StatusFilter{}, want: nil},
		{name: "chains pass through", filter: NewStatusFilter([]string{"base"}, nil, nil), want: []string{"base"}},
		{
			name:   "networks add their chain once",
			filter: NewStatusFilter([]string{"base"}, []string{"ethereum/mainnet", "ethereum/sepolia", "base/sepolia"}, nil),
			want:   []string{"base", "ethereum"},
		},
		{
			name:   "an endpoint filter needs every chain",
			filter: NewStatusFilter([]string{"base"}, nil, []string{"api-polygon-mainnet.n.dwellir.com"}),
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UptimeChainParams(tt.filter); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("UptimeChainParams() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNewStatusOverview(t *testing.T) {
	current := &StatusCurrent{
		Summary: StatusSummary{Operational: 1, Degraded: 1},
		Endpoints: []EndpointStatus{
			{Endpoint: StatusEndpointRef{FQDN: "bad"}, Status: "degraded"},
			{Endpoint: StatusEndpointRef{FQDN: "good"}, Status: "operational"},
		},
	}
	tests := []struct {
		name       string
		filtered   bool
		listAll    bool
		wantListed string
		wantFQDNs  []string
	}{
		{name: "default lists non-operational only", wantListed: StatusListedNonOperational, wantFQDNs: []string{"bad"}},
		{name: "--all lists every endpoint", listAll: true, wantListed: StatusListedAll, wantFQDNs: []string{"bad", "good"}},
		{name: "filters list every endpoint", filtered: true, wantListed: StatusListedAll, wantFQDNs: []string{"bad", "good"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewStatusOverview(current, tt.filtered, tt.listAll)
			if got.Listed != tt.wantListed {
				t.Fatalf("Listed = %q, want %q", got.Listed, tt.wantListed)
			}
			fqdns := []string{}
			for _, endpoint := range got.Endpoints {
				fqdns = append(fqdns, endpoint.Endpoint.FQDN)
			}
			if !reflect.DeepEqual(fqdns, tt.wantFQDNs) {
				t.Fatalf("endpoints = %v, want %v", fqdns, tt.wantFQDNs)
			}
			if got.Summary != current.Summary {
				t.Fatalf("summary = %#v, want the API summary", got.Summary)
			}
		})
	}
}

const uptimeFixture = `{
  "range": "7d",
  "generatedAt": "2026-10-12T12:00:00.000Z",
  "ecosystems": [{
    "ecosystem": "EVM",
    "chains": [{
      "slug": "ethereum",
      "title": "Ethereum",
      "networkTypes": [
        {"networkType": "mainnet", "displayName": "Mainnet", "endpoints": [
          {"fqdn": "api-ethereum-mainnet.n.dwellir.com", "displayName": "a",
           "aggregate": {"uptime": 99.5, "sampleCount": 1000, "laggingSampleCount": 25}}
        ]},
        {"networkType": "sepolia", "displayName": "Sepolia", "endpoints": [
          {"fqdn": "api-ethereum-sepolia.n.dwellir.com", "displayName": "b",
           "aggregate": {"uptime": 0, "sampleCount": 0}}
        ]}
      ]
    }]
  }]
}`

func TestFlattenUptime(t *testing.T) {
	var raw StatusUptimeResponse
	if err := json.Unmarshal([]byte(uptimeFixture), &raw); err != nil {
		t.Fatal(err)
	}

	all := FlattenUptime(raw, StatusFilter{})
	if len(all.Endpoints) != 2 || all.Range != "7d" {
		t.Fatalf("FlattenUptime() = %#v", all)
	}
	mainnet := all.Endpoints[0]
	if mainnet.Network != "ethereum/mainnet" || mainnet.Chain != "ethereum" {
		t.Fatalf("mainnet row = %#v", mainnet)
	}
	if mainnet.Uptime == nil || *mainnet.Uptime != 99.5 {
		t.Fatalf("uptime = %v, want 99.5", mainnet.Uptime)
	}
	if mainnet.LaggingShare == nil || *mainnet.LaggingShare != 2.5 {
		t.Fatalf("lagging share = %v, want 2.5", mainnet.LaggingShare)
	}
	// Without samples, 0% uptime would be a lie.
	if sepolia := all.Endpoints[1]; sepolia.Uptime != nil || sepolia.LaggingShare != nil {
		t.Fatalf("an endpoint without samples must report nil uptime: %#v", sepolia)
	}

	filtered := FlattenUptime(raw, NewStatusFilter(nil, []string{"ethereum/sepolia"}, nil))
	if len(filtered.Endpoints) != 1 || filtered.Endpoints[0].Endpoint != "api-ethereum-sepolia.n.dwellir.com" {
		t.Fatalf("network filter = %#v", filtered.Endpoints)
	}
}

type recordedRequest struct {
	method string
	path   string
	query  map[string][]string
	header http.Header
	body   string
}

func statusTestServer(t *testing.T, status int, body string) (*StatusAPI, *recordedRequest) {
	t.Helper()
	recorded := &recordedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		*recorded = recordedRequest{method: r.Method, path: r.URL.Path, query: r.URL.Query(), header: r.Header.Clone(), body: string(data)}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return NewStatusAPI(server.URL+"/", nil), recorded
}

func TestStatusAPIRequests(t *testing.T) {
	filter := NewStatusFilter([]string{"base"}, []string{"ethereum/mainnet"}, []string{"https://api-polygon-mainnet.n.dwellir.com/key"})

	tests := []struct {
		name       string
		call       func(*StatusAPI) error
		body       string
		wantMethod string
		wantPath   string
		wantQuery  map[string][]string
		wantBody   string
	}{
		{
			name:       "components",
			call:       func(s *StatusAPI) error { _, err := s.Components(); return err },
			body:       `{"chains":[]}`,
			wantMethod: http.MethodGet,
			wantPath:   "/api/status/components",
			wantQuery:  map[string][]string{},
		},
		{
			name:       "current sends filters",
			call:       func(s *StatusAPI) error { _, err := s.Current(filter); return err },
			body:       `{"summary":{},"endpoints":[]}`,
			wantMethod: http.MethodGet,
			wantPath:   "/api/status/current",
			wantQuery: map[string][]string{
				"chain":    {"base"},
				"network":  {"ethereum/mainnet"},
				"endpoint": {"api-polygon-mainnet.n.dwellir.com"},
			},
		},
		{
			name: "incidents sends filters and window",
			call: func(s *StatusAPI) error {
				_, err := s.Incidents(NewStatusFilter([]string{"base"}, nil, nil), IncidentQuery{Since: "24h", Status: "open", Limit: 5})
				return err
			},
			body:       `{"incidents":[]}`,
			wantMethod: http.MethodGet,
			wantPath:   "/api/status/incidents",
			wantQuery:  map[string][]string{"chain": {"base"}, "since": {"24h"}, "status": {"open"}, "limit": {"5"}},
		},
		{
			name: "uptime sends range and derived chains",
			call: func(s *StatusAPI) error {
				_, err := s.Uptime("30d", NewStatusFilter(nil, []string{"ethereum/mainnet"}, nil))
				return err
			},
			body:       `{"ecosystems":[]}`,
			wantMethod: http.MethodGet,
			wantPath:   "/api/status/uptime",
			wantQuery:  map[string][]string{"range": {"30d"}, "chain": {"ethereum"}},
		},
		{
			name: "subscribe posts the body",
			call: func(s *StatusAPI) error {
				_, err := s.Subscribe(SubscriptionRequest{URL: "https://example.com/hook", Chains: []string{"base"}, MinSeverity: "outage"})
				return err
			},
			body:       `{"id":"1"}`,
			wantMethod: http.MethodPost,
			wantPath:   "/api/subscriptions",
			wantQuery:  map[string][]string{},
			wantBody:   `{"url":"https://example.com/hook","chains":["base"],"minSeverity":"outage"}`,
		},
		{
			name:       "unsubscribe posts the token",
			call:       func(s *StatusAPI) error { _, err := s.Unsubscribe("tok"); return err },
			body:       `{"deleted":true}`,
			wantMethod: http.MethodPost,
			wantPath:   "/api/subscriptions/unsubscribe",
			wantQuery:  map[string][]string{},
			wantBody:   `{"token":"tok"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, recorded := statusTestServer(t, http.StatusOK, tt.body)
			if err := tt.call(client); err != nil {
				t.Fatalf("call failed: %v", err)
			}
			if recorded.method != tt.wantMethod || recorded.path != tt.wantPath {
				t.Fatalf("request = %s %s, want %s %s", recorded.method, recorded.path, tt.wantMethod, tt.wantPath)
			}
			if !reflect.DeepEqual(recorded.query, tt.wantQuery) {
				t.Fatalf("query = %#v, want %#v", recorded.query, tt.wantQuery)
			}
			if recorded.body != tt.wantBody {
				t.Fatalf("body = %s, want %s", recorded.body, tt.wantBody)
			}
			if got := recorded.header.Get("User-Agent"); got != "dwellir-cli" {
				t.Fatalf("User-Agent = %q", got)
			}
			// The status API is public. Never leak a Dwellir token to it.
			if got := recorded.header.Get("Authorization"); got != "" {
				t.Fatalf("Authorization header must be empty, got %q", got)
			}
		})
	}
}

func TestStatusAPIErrors(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		wantMessage string
	}{
		{name: "string error", status: 400, body: `{"error":"Unknown chain: nope"}`, wantMessage: "Unknown chain: nope"},
		{name: "object error", status: 400, body: `{"error":{"fieldErrors":{"range":["bad"]}}}`, wantMessage: `{"fieldErrors":{"range":["bad"]}}`},
		{name: "html 404", status: 404, body: `<!DOCTYPE html><html></html>`, wantMessage: ""},
		{name: "plain text 502", status: 502, body: "Bad Gateway", wantMessage: "Bad Gateway"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := statusTestServer(t, tt.status, tt.body)
			_, err := client.Components()
			var apiErr *StatusAPIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *StatusAPIError, got %T: %v", err, err)
			}
			if apiErr.StatusCode != tt.status || apiErr.Message != tt.wantMessage {
				t.Fatalf("error = %#v, want status %d message %q", apiErr, tt.status, tt.wantMessage)
			}
		})
	}
}

func TestStatusAPINetworkError(t *testing.T) {
	client := NewStatusAPI("http://127.0.0.1:1", nil)
	_, err := client.Components()
	if err == nil || !strings.HasPrefix(err.Error(), "request failed") {
		t.Fatalf("expected a request failed error, got %v", err)
	}
}
