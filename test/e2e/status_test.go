//go:build e2e

package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

const (
	statusComponentsBody = `{"generatedAt":"2026-10-12T12:00:00.000Z","chains":[
  {"slug":"base","title":"Base","ecosystem":"EVM","networks":[
    {"ref":"base/mainnet","key":"mainnet","name":"Mainnet","endpoints":[{"fqdn":"api-base-mainnet.n.dwellir.com","name":"api-base-mainnet.n.dwellir.com"}]}
  ]},
  {"slug":"ethereum","title":"Ethereum","ecosystem":"EVM","networks":[
    {"ref":"ethereum/mainnet","key":"mainnet","name":"Mainnet","endpoints":[{"fqdn":"api-ethereum-mainnet.n.dwellir.com","name":"api-ethereum-mainnet.n.dwellir.com"}]},
    {"ref":"ethereum/sepolia","key":"sepolia","name":"Sepolia","endpoints":[{"fqdn":"api-ethereum-sepolia.n.dwellir.com","name":"api-ethereum-sepolia.n.dwellir.com"}]}
  ]}
]}`

	statusCurrentBody = `{"generatedAt":"2026-10-12T12:00:00.000Z","summary":{"operational":2,"degraded":1,"outage":0},"endpoints":[
  {"chain":{"slug":"ethereum","title":"Ethereum"},"network":{"ref":"ethereum/sepolia","key":"sepolia","name":"Sepolia"},
   "endpoint":{"fqdn":"api-ethereum-sepolia.n.dwellir.com","name":"api-ethereum-sepolia.n.dwellir.com"},
   "status":"degraded","cause":"lag","since":"2026-10-12T09:41:00.000Z","incidentId":"inc-1","uptime24h":100,"laggingSamples24h":720,"samples24h":1440},
  {"chain":{"slug":"base","title":"Base"},"network":{"ref":"base/mainnet","key":"mainnet","name":"Mainnet"},
   "endpoint":{"fqdn":"api-base-mainnet.n.dwellir.com","name":"api-base-mainnet.n.dwellir.com"},
   "status":"operational","cause":null,"since":null,"incidentId":null,"uptime24h":99.9,"laggingSamples24h":0,"samples24h":1440},
  {"chain":{"slug":"ethereum","title":"Ethereum"},"network":{"ref":"ethereum/mainnet","key":"mainnet","name":"Mainnet"},
   "endpoint":{"fqdn":"api-ethereum-mainnet.n.dwellir.com","name":"api-ethereum-mainnet.n.dwellir.com"},
   "status":"operational","cause":null,"since":null,"incidentId":null,"uptime24h":100,"laggingSamples24h":0,"samples24h":1440}
]}`

	statusIncidentsBody = `{"generatedAt":"2026-10-12T12:00:00.000Z","since":"2026-10-05T12:00:00.000Z","incidents":[
  {"id":"inc-2","status":"resolved","severity":"degraded","peakSeverity":"outage","cause":"errors",
   "startedAt":"2026-10-11T08:00:00.000Z","resolvedAt":"2026-10-11T08:12:00.000Z","durationSeconds":720,
   "chain":{"slug":"base","title":"Base"},"network":{"ref":"base/mainnet","key":"mainnet","name":"Mainnet"},
   "endpoint":{"fqdn":"api-base-mainnet.n.dwellir.com","name":"api-base-mainnet.n.dwellir.com"}}
]}`

	statusUptimeBody = `{"range":"30d","generatedAt":"2026-10-12T12:00:00.000Z","ecosystems":[{"ecosystem":"EVM","chains":[
  {"slug":"ethereum","title":"Ethereum","networkTypes":[
    {"networkType":"mainnet","displayName":"Mainnet","aggregate":{"uptime":99.5,"sampleCount":1000},"endpoints":[
      {"fqdn":"api-ethereum-mainnet.n.dwellir.com","displayName":"Mainnet","aggregate":{"uptime":99.5,"sampleCount":1000,"laggingSampleCount":50},"buckets":[]}]},
    {"networkType":"sepolia","displayName":"Sepolia","aggregate":{"uptime":98,"sampleCount":1000},"endpoints":[
      {"fqdn":"api-ethereum-sepolia.n.dwellir.com","displayName":"Sepolia","aggregate":{"uptime":98,"sampleCount":1000,"laggingSampleCount":0},"buckets":[]}]}
  ]}
]}]}`

	statusSubscriptionBody = `{"id":"sub-1","kind":"slack","status":"pending","signingSecret":"whsec_test",
  "unsubscribeUrl":"https://status.dwellir.com/unsubscribe?token=tok-1","message":"We sent a test event to your URL."}`
)

// statusStub serves the status API. Override a path's reply with set().
type statusStub struct {
	*httptest.Server
	mu       sync.Mutex
	replies  map[string]statusReply
	requests map[string]*http.Request
	bodies   map[string]string
}

type statusReply struct {
	status int
	body   string
}

func newStatusStub(t *testing.T) *statusStub {
	t.Helper()
	stub := &statusStub{
		replies: map[string]statusReply{
			"/api/status/components":         {200, statusComponentsBody},
			"/api/status/current":            {200, statusCurrentBody},
			"/api/status/incidents":          {200, statusIncidentsBody},
			"/api/status/uptime":             {200, statusUptimeBody},
			"/api/subscriptions":             {201, statusSubscriptionBody},
			"/api/subscriptions/unsubscribe": {200, `{"deleted":true}`},
		},
		requests: map[string]*http.Request{},
		bodies:   map[string]string{},
	}
	stub.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		stub.mu.Lock()
		reply, ok := stub.replies[r.URL.Path]
		stub.requests[r.URL.Path] = r
		stub.bodies[r.URL.Path] = string(body)
		stub.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(reply.status)
		_, _ = w.Write([]byte(reply.body))
	}))
	t.Cleanup(stub.Close)
	return stub
}

func (s *statusStub) set(path string, status int, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.replies[path] = statusReply{status, body}
}

func (s *statusStub) request(t *testing.T, path string) (*http.Request, string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[path]
	if !ok {
		t.Fatalf("expected a request to %s", path)
	}
	return req, s.bodies[path]
}

func (s *statusStub) called(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.requests[path]
	return ok
}

func statusEnv(serverURL string) map[string]string {
	return map[string]string{"DWELLIR_STATUS": "1", "DWELLIR_STATUS_URL": serverURL}
}

func runStatus(t *testing.T, stub *statusStub, args ...string) cliResult {
	t.Helper()
	return runCLIWithEnv(t, statusEnv(stub.URL), append([]string{"status"}, args...)...)
}

func expectExit(t *testing.T, res cliResult, want int) {
	t.Helper()
	if res.exitCode != want {
		t.Fatalf("exit code = %d, want %d\nstdout: %s\nstderr: %s", res.exitCode, want, res.stdout, res.stderr)
	}
}

func expectErrorCode(t *testing.T, res cliResult, want string) map[string]interface{} {
	t.Helper()
	parsed := parseJSON(t, res.stdout)
	if parsed["ok"] != false {
		t.Fatalf("expected ok:false, got %#v", parsed)
	}
	errBody, _ := parsed["error"].(map[string]interface{})
	if errBody["code"] != want {
		t.Fatalf("error code = %#v, want %q: %#v", errBody["code"], want, errBody)
	}
	return errBody
}

func statusData(t *testing.T, res cliResult) map[string]interface{} {
	t.Helper()
	parsed := parseJSON(t, res.stdout)
	if parsed["ok"] != true {
		t.Fatalf("expected ok:true, got %#v", parsed)
	}
	data, ok := parsed["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected an object in data, got %#v", parsed["data"])
	}
	return data
}

// The gate keeps a released binary from shipping commands for an API that is
// not deployed yet.
func TestStatusIsUnknownCommandWhenTheGateIsOff(t *testing.T) {
	for _, value := range []string{"", "0", "false"} {
		t.Run("DWELLIR_STATUS="+value, func(t *testing.T) {
			for _, args := range [][]string{{"status", "--json"}, {"status", "components", "--json"}} {
				res := runCLIWithEnv(t, map[string]string{"DWELLIR_STATUS": value}, args...)
				if res.exitCode == 0 {
					t.Fatalf("%v: expected a non-zero exit, got 0\nstdout: %s", args, res.stdout)
				}
				if combined := res.stdout + res.stderr; !strings.Contains(combined, "unknown command") {
					t.Fatalf("%v: expected an unknown command error, got:\n%s", args, combined)
				}
			}
		})
	}
}

func TestStatusHelpFollowsTheGate(t *testing.T) {
	off := runCLIWithEnv(t, map[string]string{"DWELLIR_STATUS": "0"}, "--help")
	expectExit(t, off, 0)
	if strings.Contains(off.stdout, "status") {
		t.Fatalf("--help must not mention status with the gate off:\n%s", off.stdout)
	}

	on := runCLIWithEnv(t, map[string]string{"DWELLIR_STATUS": "1"}, "--help")
	expectExit(t, on, 0)
	if !strings.Contains(on.stdout, "status") {
		t.Fatalf("expected status in --help with the gate on:\n%s", on.stdout)
	}
}

func TestDoctorReportsTheStatusGate(t *testing.T) {
	for _, tt := range []struct {
		env  string
		want bool
	}{{"0", false}, {"1", true}} {
		t.Run("DWELLIR_STATUS="+tt.env, func(t *testing.T) {
			res := runCLIWithEnv(t, map[string]string{"DWELLIR_STATUS": tt.env}, "doctor", "--json")
			expectExit(t, res, 0)
			check := findDoctorCheck(t, parseJSON(t, res.stdout), "feature_gates")
			details, _ := check["details"].(map[string]interface{})
			if details["status_enabled"] != tt.want || details["status_env"] != tt.env {
				t.Fatalf("feature_gates details = %#v", details)
			}
		})
	}
}

func TestStatusCurrentListsNonOperationalEndpointsByDefault(t *testing.T) {
	stub := newStatusStub(t)

	res := runStatus(t, stub, "--json")
	expectExit(t, res, 0)
	data := statusData(t, res)
	if data["listed"] != "non_operational" {
		t.Fatalf("listed = %#v", data["listed"])
	}
	endpoints, _ := data["endpoints"].([]interface{})
	if len(endpoints) != 1 {
		t.Fatalf("expected only the degraded endpoint, got %#v", endpoints)
	}
	summary, _ := data["summary"].(map[string]interface{})
	if summary["operational"] != float64(2) || summary["degraded"] != float64(1) {
		t.Fatalf("summary = %#v", summary)
	}

	all := runStatus(t, stub, "--all", "--json")
	expectExit(t, all, 0)
	if endpoints, _ := statusData(t, all)["endpoints"].([]interface{}); len(endpoints) != 3 {
		t.Fatalf("--all must list every endpoint, got %d", len(endpoints))
	}

	human := runStatus(t, stub, "--human")
	expectExit(t, human, 0)
	for _, want := range []string{"Some endpoints are degraded: 2 operational, 1 degraded, 0 outage.", "api-ethereum-sepolia.n.dwellir.com", "50.00%"} {
		if !strings.Contains(human.stdout, want) {
			t.Fatalf("expected %q in human output:\n%s", want, human.stdout)
		}
	}
	if strings.Contains(human.stdout, "api-base-mainnet") {
		t.Fatalf("operational endpoints must stay hidden without --all:\n%s", human.stdout)
	}
}

func TestStatusCurrentForwardsFiltersAndListsAllMatches(t *testing.T) {
	stub := newStatusStub(t)

	res := runStatus(t, stub,
		"--chain", "Base,ethereum", "--network", "ethereum/mainnet",
		"--endpoint", "https://api-polygon-mainnet.n.dwellir.com/some-key", "--json")
	expectExit(t, res, 0)
	if data := statusData(t, res); data["listed"] != "all" || data["filtered"] != true {
		t.Fatalf("filters must list every match: %#v", data)
	}

	req, _ := stub.request(t, "/api/status/current")
	query := req.URL.Query()
	if got := query["chain"]; strings.Join(got, ",") != "base,ethereum" {
		t.Fatalf("chain params = %v", got)
	}
	if got := query.Get("network"); got != "ethereum/mainnet" {
		t.Fatalf("network param = %q", got)
	}
	// The key in the URL path must never leave the machine.
	if got := query.Get("endpoint"); got != "api-polygon-mainnet.n.dwellir.com" {
		t.Fatalf("endpoint param = %q", got)
	}
}

func TestStatusNeedsNoLoginAndSendsNoToken(t *testing.T) {
	stub := newStatusStub(t)
	env := statusEnv(stub.URL)
	env["DWELLIR_TOKEN"] = "secret-token"

	res := runCLIWithEnv(t, env, "status", "--json")
	expectExit(t, res, 0)
	req, _ := stub.request(t, "/api/status/current")
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("the status API must not receive credentials, got %q", got)
	}
}

func TestStatusURLFlagOverridesTheEnvironment(t *testing.T) {
	stub := newStatusStub(t)
	env := statusEnv("http://127.0.0.1:1")

	res := runCLIWithEnv(t, env, "status", "--status-url", stub.URL, "--json")
	expectExit(t, res, 0)

	bad := runCLIWithEnv(t, env, "status", "--status-url", "status.dwellir.com", "--json")
	expectExit(t, bad, 4)
	expectErrorCode(t, bad, "validation_error")
}

func TestStatusMapsAPIFailuresToExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantExit int
		wantCode string
	}{
		{name: "404", status: 404, body: `not found`, wantExit: 3, wantCode: "not_found"},
		{name: "400", status: 400, body: `{"error":"Too many filter values."}`, wantExit: 4, wantCode: "validation_error"},
		{name: "500", status: 500, body: `{"error":"boom"}`, wantExit: 1, wantCode: "status_unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := newStatusStub(t)
			stub.set("/api/status/current", tt.status, tt.body)
			res := runStatus(t, stub, "--json")
			expectExit(t, res, tt.wantExit)
			expectErrorCode(t, res, tt.wantCode)
		})
	}

	unreachable := runCLIWithEnv(t, statusEnv("http://127.0.0.1:1"), "status", "--json")
	expectExit(t, unreachable, 1)
	expectErrorCode(t, unreachable, "network_error")
}

func TestStatusComponents(t *testing.T) {
	stub := newStatusStub(t)

	res := runStatus(t, stub, "components", "--chain", "ethereum", "--json")
	expectExit(t, res, 0)
	chains, _ := statusData(t, res)["chains"].([]interface{})
	if len(chains) != 1 {
		t.Fatalf("expected only ethereum, got %#v", chains)
	}

	human := runStatus(t, stub, "components", "--human")
	expectExit(t, human, 0)
	for _, want := range []string{"base/mainnet", "ethereum/sepolia", "api-ethereum-mainnet.n.dwellir.com"} {
		if !strings.Contains(human.stdout, want) {
			t.Fatalf("expected %q in:\n%s", want, human.stdout)
		}
	}

	unknown := runStatus(t, stub, "components", "--chain", "etherum", "--json")
	expectExit(t, unknown, 4)
	errBody := expectErrorCode(t, unknown, "validation_error")
	if help, _ := errBody["help"].(string); !strings.Contains(help, "Did you mean chain ethereum?") {
		t.Fatalf("expected a suggestion in help, got %q", help)
	}
}

func TestStatusHistory(t *testing.T) {
	stub := newStatusStub(t)

	res := runStatus(t, stub, "history", "--chain", "base", "--since", "30d", "--status", "resolved", "--limit", "10", "--json")
	expectExit(t, res, 0)
	incidents, _ := statusData(t, res)["incidents"].([]interface{})
	if len(incidents) != 1 {
		t.Fatalf("incidents = %#v", incidents)
	}
	req, _ := stub.request(t, "/api/status/incidents")
	want := url.Values{"chain": {"base"}, "since": {"30d"}, "status": {"resolved"}, "limit": {"10"}}
	if got := req.URL.Query(); got.Encode() != want.Encode() {
		t.Fatalf("query = %v, want %v", got, want)
	}

	defaults := runStatus(t, stub, "history", "--json")
	expectExit(t, defaults, 0)
	req, _ = stub.request(t, "/api/status/incidents")
	if q := req.URL.Query(); q.Get("since") != "7d" || q.Get("status") != "all" || q.Get("limit") != "50" {
		t.Fatalf("default query = %v", q)
	}

	human := runStatus(t, stub, "history", "--human")
	expectExit(t, human, 0)
	for _, want := range []string{"2026-10-11 08:00 UTC", "resolved", "errors", "12m"} {
		if !strings.Contains(human.stdout, want) {
			t.Fatalf("expected %q in:\n%s", want, human.stdout)
		}
	}

	for _, args := range [][]string{{"--status", "closed"}, {"--limit", "0"}, {"--limit", "201"}} {
		bad := runStatus(t, stub, append([]string{"history", "--json"}, args...)...)
		expectExit(t, bad, 4)
		expectErrorCode(t, bad, "validation_error")
	}
}

func TestStatusUptime(t *testing.T) {
	stub := newStatusStub(t)

	res := runStatus(t, stub, "uptime", "--network", "ethereum/mainnet", "--range", "30d", "--json")
	expectExit(t, res, 0)
	data := statusData(t, res)
	endpoints, _ := data["endpoints"].([]interface{})
	if len(endpoints) != 1 {
		t.Fatalf("expected the network filter to keep one endpoint, got %#v", endpoints)
	}
	row, _ := endpoints[0].(map[string]interface{})
	if row["endpoint"] != "api-ethereum-mainnet.n.dwellir.com" || row["uptime"] != 99.5 || row["laggingShare"] != float64(5) {
		t.Fatalf("row = %#v", row)
	}
	req, _ := stub.request(t, "/api/status/uptime")
	if q := req.URL.Query(); q.Get("range") != "30d" || q.Get("chain") != "ethereum" {
		t.Fatalf("query = %v", q)
	}

	human := runStatus(t, stub, "uptime", "--chain", "ethereum", "--human")
	expectExit(t, human, 0)
	for _, want := range []string{"99.50%", "5.00%", "98.00%", "ethereum/sepolia"} {
		if !strings.Contains(human.stdout, want) {
			t.Fatalf("expected %q in:\n%s", want, human.stdout)
		}
	}

	bad := runStatus(t, stub, "uptime", "--range", "1y", "--json")
	expectExit(t, bad, 4)
	expectErrorCode(t, bad, "validation_error")
}

func TestStatusFeed(t *testing.T) {
	stub := newStatusStub(t)

	res := runStatus(t, stub, "feed", "--chain", "base", "--network", "ethereum/mainnet", "--format", "rss", "--json")
	expectExit(t, res, 0)
	data := statusData(t, res)
	wantURL := stub.URL + "/history.rss?chain=base&network=ethereum%2Fmainnet"
	if data["url"] != wantURL || data["validated"] != true {
		t.Fatalf("feed = %#v, want url %s", data, wantURL)
	}

	slack := runStatus(t, stub, "feed", "--chain", "base", "--slack", "--human")
	expectExit(t, slack, 0)
	if !strings.Contains(slack.stdout, "/feed subscribe "+stub.URL+"/history.atom?chain=base") {
		t.Fatalf("expected the Slack command:\n%s", slack.stdout)
	}

	unknown := runStatus(t, stub, "feed", "--chain", "etherum", "--json")
	expectExit(t, unknown, 4)
	errBody := expectErrorCode(t, unknown, "validation_error")
	help, _ := errBody["help"].(string)
	if !strings.Contains(help, "ethereum") || !strings.Contains(help, "--no-validate") {
		t.Fatalf("expected a suggestion and the --no-validate hint, got %q", help)
	}
}

func TestStatusFeedNoValidateSkipsTheComponentFetch(t *testing.T) {
	stub := newStatusStub(t)
	stub.set("/api/status/components", 404, "not found")

	failing := runStatus(t, stub, "feed", "--chain", "base", "--json")
	expectExit(t, failing, 3)
	errBody := expectErrorCode(t, failing, "not_found")
	if help, _ := errBody["help"].(string); !strings.Contains(help, "--no-validate") {
		t.Fatalf("expected the --no-validate hint, got %q", help)
	}

	fresh := newStatusStub(t)
	res := runStatus(t, fresh, "feed", "--chain", "anything", "--no-validate", "--json")
	expectExit(t, res, 0)
	if fresh.called("/api/status/components") {
		t.Fatal("--no-validate must not fetch components")
	}
	if data := statusData(t, res); data["validated"] != false {
		t.Fatalf("validated = %#v", data["validated"])
	}
}

func TestStatusSubscribe(t *testing.T) {
	stub := newStatusStub(t)

	res := runStatus(t, stub, "subscribe", "--url", "https://hooks.slack.com/services/T/B/X",
		"--chain", "base", "--endpoint", "https://api-ethereum-mainnet.n.dwellir.com/key", "--min-severity", "outage", "--json")
	expectExit(t, res, 0)
	data := statusData(t, res)
	if data["kind"] != "slack" || data["signingSecret"] != "whsec_test" {
		t.Fatalf("subscription = %#v", data)
	}
	req, body := stub.request(t, "/api/subscriptions")
	if req.Method != http.MethodPost {
		t.Fatalf("method = %s", req.Method)
	}
	var sent map[string]interface{}
	if err := json.Unmarshal([]byte(body), &sent); err != nil {
		t.Fatalf("body is not JSON: %s", body)
	}
	if sent["minSeverity"] != "outage" || sent["url"] != "https://hooks.slack.com/services/T/B/X" {
		t.Fatalf("body = %s", body)
	}
	if endpoints, _ := sent["endpoints"].([]interface{}); len(endpoints) != 1 || endpoints[0] != "api-ethereum-mainnet.n.dwellir.com" {
		t.Fatalf("endpoints = %#v", sent["endpoints"])
	}
	if _, present := sent["networks"]; present {
		t.Fatalf("empty filters must be omitted: %s", body)
	}

	human := runStatus(t, stub, "subscribe", "--url", "https://example.com/hook", "--human")
	expectExit(t, human, 0)
	for _, want := range []string{"whsec_test", "unsubscribe?token=tok-1", "only once"} {
		if !strings.Contains(human.stdout, want) {
			t.Fatalf("expected %q in:\n%s", want, human.stdout)
		}
	}
}

func TestStatusSubscribeErrors(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		reply    *statusReply
		wantExit int
		wantCode string
	}{
		{name: "missing url", args: nil, wantExit: 4, wantCode: "validation_error"},
		{name: "http url", args: []string{"--url", "http://example.com/hook"}, wantExit: 4, wantCode: "validation_error"},
		{name: "bad severity", args: []string{"--url", "https://example.com/hook", "--min-severity", "minor"}, wantExit: 4, wantCode: "validation_error"},
		{
			name: "API rejects a filter", args: []string{"--url", "https://example.com/hook", "--chain", "nope"},
			reply: &statusReply{400, `{"error":"Unknown chain: nope"}`}, wantExit: 4, wantCode: "validation_error",
		},
		{
			name: "duplicate", args: []string{"--url", "https://example.com/hook"},
			reply: &statusReply{409, `{"error":"This URL already has a live subscription."}`}, wantExit: 1, wantCode: "conflict",
		},
		{
			name: "unavailable", args: []string{"--url", "https://example.com/hook"},
			reply: &statusReply{503, `{"error":"Subscriptions are unavailable."}`}, wantExit: 1, wantCode: "status_unavailable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := newStatusStub(t)
			if tt.reply != nil {
				stub.set("/api/subscriptions", tt.reply.status, tt.reply.body)
			}
			res := runStatus(t, stub, append([]string{"subscribe", "--json"}, tt.args...)...)
			expectExit(t, res, tt.wantExit)
			errBody := expectErrorCode(t, res, tt.wantCode)
			if tt.reply != nil {
				var apiBody map[string]string
				_ = json.Unmarshal([]byte(tt.reply.body), &apiBody)
				if errBody["message"] != apiBody["error"] {
					t.Fatalf("message = %#v, want the API message %q", errBody["message"], apiBody["error"])
				}
			}
		})
	}
}

func TestStatusUnsubscribe(t *testing.T) {
	stub := newStatusStub(t)

	res := runStatus(t, stub, "unsubscribe", "https://status.dwellir.com/unsubscribe?token=tok-1", "--json")
	expectExit(t, res, 0)
	if data := statusData(t, res); data["deleted"] != true {
		t.Fatalf("data = %#v", data)
	}
	if _, body := stub.request(t, "/api/subscriptions/unsubscribe"); body != `{"token":"tok-1"}` {
		t.Fatalf("body = %s", body)
	}

	bare := runStatus(t, stub, "unsubscribe", "tok-2", "--human")
	expectExit(t, bare, 0)
	if !strings.Contains(bare.stdout, "Subscription deleted.") {
		t.Fatalf("human output:\n%s", bare.stdout)
	}

	stub.set("/api/subscriptions/unsubscribe", 200, `{"deleted":false}`)
	missing := runStatus(t, stub, "unsubscribe", "tok-3", "--json")
	expectExit(t, missing, 3)
	expectErrorCode(t, missing, "not_found")

	noToken := runStatus(t, stub, "unsubscribe", "https://status.dwellir.com/unsubscribe", "--json")
	expectExit(t, noToken, 4)
	expectErrorCode(t, noToken, "validation_error")
}
