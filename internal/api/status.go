package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// DefaultStatusBaseURL is the public status site. Its API needs no auth.
const DefaultStatusBaseURL = "https://status.dwellir.com"

// StatusAPIError is a non-2xx reply from the status API. The API reports
// failures as {"error": "<message>"}; Message holds that text when present.
type StatusAPIError struct {
	StatusCode int
	Message    string
}

func (e *StatusAPIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("status API error (HTTP %d)", e.StatusCode)
	}
	return fmt.Sprintf("status API error (HTTP %d): %s", e.StatusCode, e.Message)
}

// StatusFilter selects components. A component matches when any value matches
// (union). An empty filter matches every component.
type StatusFilter struct {
	Chains    []string `json:"chains,omitempty"`
	Networks  []string `json:"networks,omitempty"`
	Endpoints []string `json:"endpoints,omitempty"`
}

// NewStatusFilter normalizes raw flag values: it trims and lowercases values,
// drops empty and duplicate values, and reduces endpoint URLs to their host.
func NewStatusFilter(chains, networks, endpoints []string) StatusFilter {
	return StatusFilter{
		Chains:    normalizeFilterValues(chains, strings.ToLower),
		Networks:  normalizeFilterValues(networks, strings.ToLower),
		Endpoints: normalizeFilterValues(endpoints, NormalizeEndpointHost),
	}
}

// IsEmpty reports whether the filter matches every component.
func (f StatusFilter) IsEmpty() bool {
	return len(f.Chains) == 0 && len(f.Networks) == 0 && len(f.Endpoints) == 0
}

// Matches reports whether a component passes the filter.
func (f StatusFilter) Matches(chain, networkRef, fqdn string) bool {
	if f.IsEmpty() {
		return true
	}
	return containsFold(f.Chains, chain) || containsFold(f.Networks, networkRef) || containsFold(f.Endpoints, fqdn)
}

// Encode returns the query string in chain, network, endpoint order, the same
// order the status site uses for its feed links.
func (f StatusFilter) Encode() string {
	parts := make([]string, 0, len(f.Chains)+len(f.Networks)+len(f.Endpoints))
	appendParam := func(key string, values []string) {
		for _, value := range values {
			parts = append(parts, key+"="+url.QueryEscape(value))
		}
	}
	appendParam("chain", f.Chains)
	appendParam("network", f.Networks)
	appendParam("endpoint", f.Endpoints)
	return strings.Join(parts, "&")
}

// NormalizeEndpointHost reduces an endpoint URL to its lowercase host. Plain
// FQDNs pass through, lowercased.
func NormalizeEndpointHost(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if strings.Contains(value, "://") {
		if u, err := url.Parse(value); err == nil && u.Hostname() != "" {
			return u.Hostname()
		}
	}
	if host, _, found := strings.Cut(value, "/"); found {
		value = host
	}
	return strings.TrimSuffix(value, ".")
}

func normalizeFilterValues(values []string, normalize func(string) string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			value := normalize(strings.TrimSpace(part))
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

// StatusChainRef, StatusNetworkRef and StatusEndpointRef name a component in
// current-status and incident payloads.
type StatusChainRef struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type StatusNetworkRef struct {
	Ref  string `json:"ref"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type StatusEndpointRef struct {
	FQDN string `json:"fqdn"`
	Name string `json:"name"`
}

// StatusComponents is the filterable catalog from /api/status/components.
type StatusComponents struct {
	GeneratedAt string                 `json:"generatedAt"`
	Chains      []StatusComponentChain `json:"chains"`
}

type StatusComponentChain struct {
	Slug      string                   `json:"slug"`
	Title     string                   `json:"title"`
	Ecosystem string                   `json:"ecosystem"`
	Networks  []StatusComponentNetwork `json:"networks"`
}

type StatusComponentNetwork struct {
	Ref       string              `json:"ref"`
	Key       string              `json:"key"`
	Name      string              `json:"name"`
	Endpoints []StatusEndpointRef `json:"endpoints"`
}

// StatusSummary counts endpoints per status.
type StatusSummary struct {
	Operational int `json:"operational"`
	Degraded    int `json:"degraded"`
	Outage      int `json:"outage"`
}

// StatusCurrent is the reply from /api/status/current.
type StatusCurrent struct {
	GeneratedAt string           `json:"generatedAt"`
	Summary     StatusSummary    `json:"summary"`
	Endpoints   []EndpointStatus `json:"endpoints"`
}

// EndpointStatus is the current state of one endpoint.
type EndpointStatus struct {
	Chain             StatusChainRef    `json:"chain"`
	Network           StatusNetworkRef  `json:"network"`
	Endpoint          StatusEndpointRef `json:"endpoint"`
	Status            string            `json:"status"`
	Cause             *string           `json:"cause"`
	Since             *string           `json:"since"`
	IncidentID        *string           `json:"incidentId"`
	Uptime24h         *float64          `json:"uptime24h"`
	LaggingSamples24h int64             `json:"laggingSamples24h"`
	Samples24h        int64             `json:"samples24h"`
}

// StatusIncidents is the reply from /api/status/incidents.
type StatusIncidents struct {
	GeneratedAt string           `json:"generatedAt"`
	Since       string           `json:"since"`
	Incidents   []StatusIncident `json:"incidents"`
}

type StatusIncident struct {
	ID              string            `json:"id"`
	Status          string            `json:"status"`
	Severity        string            `json:"severity"`
	PeakSeverity    string            `json:"peakSeverity"`
	Cause           *string           `json:"cause"`
	StartedAt       string            `json:"startedAt"`
	ResolvedAt      *string           `json:"resolvedAt"`
	DurationSeconds *int64            `json:"durationSeconds"`
	Chain           StatusChainRef    `json:"chain"`
	Network         StatusNetworkRef  `json:"network"`
	Endpoint        StatusEndpointRef `json:"endpoint"`
}

// IncidentQuery holds the /api/status/incidents parameters besides filters.
type IncidentQuery struct {
	Since  string
	Status string
	Limit  int
}

// StatusUptimeResponse is the raw /api/status/uptime payload, trimmed to the
// fields the CLI reads.
type StatusUptimeResponse struct {
	Range       string                  `json:"range"`
	GeneratedAt string                  `json:"generatedAt"`
	Ecosystems  []StatusUptimeEcosystem `json:"ecosystems"`
}

type StatusUptimeEcosystem struct {
	Ecosystem string              `json:"ecosystem"`
	Chains    []StatusUptimeChain `json:"chains"`
}

type StatusUptimeChain struct {
	Slug         string                `json:"slug"`
	Title        string                `json:"title"`
	NetworkTypes []StatusUptimeNetwork `json:"networkTypes"`
}

type StatusUptimeNetwork struct {
	NetworkType string                 `json:"networkType"`
	DisplayName string                 `json:"displayName"`
	Endpoints   []StatusUptimeEndpoint `json:"endpoints"`
}

type StatusUptimeEndpoint struct {
	FQDN        string                `json:"fqdn"`
	DisplayName string                `json:"displayName"`
	Aggregate   StatusUptimeAggregate `json:"aggregate"`
}

type StatusUptimeAggregate struct {
	Uptime             float64 `json:"uptime"`
	SampleCount        int64   `json:"sampleCount"`
	LaggingSampleCount int64   `json:"laggingSampleCount"`
}

// StatusUptime is per-endpoint uptime for one range.
type StatusUptime struct {
	Range       string               `json:"range"`
	GeneratedAt string               `json:"generatedAt"`
	Endpoints   []EndpointUptimeStat `json:"endpoints"`
}

// EndpointUptimeStat is one endpoint's uptime aggregate. LaggingShare is the
// percentage of samples that trailed the chain head, or nil without samples.
type EndpointUptimeStat struct {
	Chain              string   `json:"chain"`
	Network            string   `json:"network"`
	Endpoint           string   `json:"endpoint"`
	Uptime             *float64 `json:"uptime"`
	SampleCount        int64    `json:"sampleCount"`
	LaggingSampleCount int64    `json:"laggingSampleCount"`
	LaggingShare       *float64 `json:"laggingShare"`
}

// FlattenUptime lists every endpoint in the payload that passes the filter.
// Uptime is nil when the endpoint has no samples in the range.
func FlattenUptime(resp StatusUptimeResponse, filter StatusFilter) StatusUptime {
	out := StatusUptime{Range: resp.Range, GeneratedAt: resp.GeneratedAt, Endpoints: []EndpointUptimeStat{}}
	seen := map[string]bool{}
	for _, ecosystem := range resp.Ecosystems {
		for _, chain := range ecosystem.Chains {
			for _, network := range chain.NetworkTypes {
				ref := chain.Slug + "/" + network.NetworkType
				for _, endpoint := range network.Endpoints {
					key := ref + "|" + endpoint.FQDN
					if seen[key] || !filter.Matches(chain.Slug, ref, endpoint.FQDN) {
						continue
					}
					seen[key] = true
					out.Endpoints = append(out.Endpoints, newEndpointUptimeStat(chain.Slug, ref, endpoint))
				}
			}
		}
	}
	return out
}

func newEndpointUptimeStat(chain, ref string, endpoint StatusUptimeEndpoint) EndpointUptimeStat {
	stat := EndpointUptimeStat{
		Chain:              chain,
		Network:            ref,
		Endpoint:           endpoint.FQDN,
		SampleCount:        endpoint.Aggregate.SampleCount,
		LaggingSampleCount: endpoint.Aggregate.LaggingSampleCount,
	}
	if stat.SampleCount > 0 {
		uptime := endpoint.Aggregate.Uptime
		share := float64(stat.LaggingSampleCount) / float64(stat.SampleCount) * 100
		stat.Uptime = &uptime
		stat.LaggingShare = &share
	}
	return stat
}

// UptimeChainParams returns the chain slugs to send to /api/status/uptime. That
// route filters by chain only, so the CLI narrows the request by the chains it
// can derive and filters networks and endpoints itself. An endpoint filter
// names no chain, so it needs every chain: the result is then nil.
func UptimeChainParams(filter StatusFilter) []string {
	if filter.IsEmpty() || len(filter.Endpoints) > 0 {
		return nil
	}
	chains := append([]string{}, filter.Chains...)
	for _, network := range filter.Networks {
		chain, _, _ := strings.Cut(network, "/")
		if chain != "" && !containsFold(chains, chain) {
			chains = append(chains, chain)
		}
	}
	return chains
}

// SubscriptionRequest is the POST /api/subscriptions body.
type SubscriptionRequest struct {
	URL         string   `json:"url"`
	Chains      []string `json:"chains,omitempty"`
	Networks    []string `json:"networks,omitempty"`
	Endpoints   []string `json:"endpoints,omitempty"`
	MinSeverity string   `json:"minSeverity"`
}

// Subscription is the reply to a created subscription. The API returns the
// signing secret and the unsubscribe token only once.
type Subscription struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Status         string `json:"status"`
	SigningSecret  string `json:"signingSecret"`
	UnsubscribeURL string `json:"unsubscribeUrl"`
	Message        string `json:"message"`
}

// UnsubscribeResult reports whether a subscription matched the token.
type UnsubscribeResult struct {
	Deleted bool `json:"deleted"`
}

// ParseUnsubscribeToken accepts a bare token or an unsubscribe URL and returns
// the token.
func ParseUnsubscribeToken(input string) (string, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", fmt.Errorf("the token is empty")
	}
	if !strings.Contains(value, "://") {
		return value, nil
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("the unsubscribe URL is not valid: %w", err)
	}
	token := strings.TrimSpace(u.Query().Get("token"))
	if token == "" {
		return "", fmt.Errorf("the unsubscribe URL has no token parameter")
	}
	return token, nil
}

// FeedURL builds an Atom or RSS feed URL for the filter.
func FeedURL(baseURL, format string, filter StatusFilter) string {
	feed := strings.TrimSuffix(baseURL, "/") + "/history." + format
	if query := filter.Encode(); query != "" {
		return feed + "?" + query
	}
	return feed
}

// StatusAPI calls the public status API. It sends no credentials.
type StatusAPI struct {
	baseURL    string
	httpClient *http.Client
}

// NewStatusAPI returns a client for baseURL. A nil client gets the default
// timeout.
func NewStatusAPI(baseURL string, client *http.Client) *StatusAPI {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	return &StatusAPI{baseURL: strings.TrimSuffix(strings.TrimSpace(baseURL), "/"), httpClient: client}
}

// BaseURL returns the base URL without a trailing slash.
func (s *StatusAPI) BaseURL() string {
	return s.baseURL
}

func (s *StatusAPI) Components() (*StatusComponents, error) {
	var out StatusComponents
	if err := s.get("/api/status/components", "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *StatusAPI) Current(filter StatusFilter) (*StatusCurrent, error) {
	var out StatusCurrent
	if err := s.get("/api/status/current", filter.Encode(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *StatusAPI) Incidents(filter StatusFilter, query IncidentQuery) (*StatusIncidents, error) {
	params := []string{}
	if encoded := filter.Encode(); encoded != "" {
		params = append(params, encoded)
	}
	if query.Since != "" {
		params = append(params, "since="+url.QueryEscape(query.Since))
	}
	if query.Status != "" {
		params = append(params, "status="+url.QueryEscape(query.Status))
	}
	if query.Limit > 0 {
		params = append(params, "limit="+strconv.Itoa(query.Limit))
	}
	var out StatusIncidents
	if err := s.get("/api/status/incidents", strings.Join(params, "&"), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Uptime fetches uptime for the range, narrowed to the chains the filter
// names, and keeps the endpoints that pass the filter.
func (s *StatusAPI) Uptime(rangeKey string, filter StatusFilter) (*StatusUptime, error) {
	params := []string{"range=" + url.QueryEscape(rangeKey)}
	for _, chain := range UptimeChainParams(filter) {
		params = append(params, "chain="+url.QueryEscape(chain))
	}
	var raw StatusUptimeResponse
	if err := s.get("/api/status/uptime", strings.Join(params, "&"), &raw); err != nil {
		return nil, err
	}
	out := FlattenUptime(raw, filter)
	return &out, nil
}

func (s *StatusAPI) Subscribe(req SubscriptionRequest) (*Subscription, error) {
	var out Subscription
	if err := s.post("/api/subscriptions", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *StatusAPI) Unsubscribe(token string) (*UnsubscribeResult, error) {
	var out UnsubscribeResult
	if err := s.post("/api/subscriptions/unsubscribe", map[string]string{"token": token}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *StatusAPI) get(path, rawQuery string, result interface{}) error {
	target := s.baseURL + path
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	return s.do(req, result)
}

func (s *StatusAPI) post(path string, body, result interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling body: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, s.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return s.do(req, result)
}

func (s *StatusAPI) do(req *http.Request, result interface{}) error {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "dwellir-cli")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return &StatusAPIError{StatusCode: resp.StatusCode, Message: statusErrorMessage(body)}
	}
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}
	return nil
}

// statusErrorMessage reads {"error": "..."}. The uptime route returns an
// object under "error", so anything that is not a string falls back to the
// raw body, trimmed.
func statusErrorMessage(body []byte) string {
	var payload struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && len(payload.Error) > 0 {
		var text string
		if json.Unmarshal(payload.Error, &text) == nil {
			return strings.TrimSpace(text)
		}
		return strings.TrimSpace(string(payload.Error))
	}
	raw := strings.TrimSpace(string(body))
	if len(raw) > 300 {
		raw = raw[:300] + "…"
	}
	if strings.HasPrefix(raw, "<") {
		return ""
	}
	return raw
}

// StatusOverview is what `dwellir status` reports: the summary, plus either
// every matching endpoint or only those that are not operational.
type StatusOverview struct {
	GeneratedAt string           `json:"generatedAt"`
	Summary     StatusSummary    `json:"summary"`
	Filtered    bool             `json:"filtered"`
	Listed      string           `json:"listed"`
	Endpoints   []EndpointStatus `json:"endpoints"`
}

// Values for StatusOverview.Listed.
const (
	StatusListedAll            = "all"
	StatusListedNonOperational = "non_operational"
)

// NewStatusOverview lists every endpoint when filters are set or listAll is
// true. Otherwise it lists only endpoints that are not operational.
func NewStatusOverview(current *StatusCurrent, filtered, listAll bool) StatusOverview {
	overview := StatusOverview{
		GeneratedAt: current.GeneratedAt,
		Summary:     current.Summary,
		Filtered:    filtered,
		Listed:      StatusListedAll,
		Endpoints:   []EndpointStatus{},
	}
	if !filtered && !listAll {
		overview.Listed = StatusListedNonOperational
	}
	for _, endpoint := range current.Endpoints {
		if overview.Listed == StatusListedNonOperational && endpoint.Status == "operational" {
			continue
		}
		overview.Endpoints = append(overview.Endpoints, endpoint)
	}
	return overview
}

// StatusFeed is a feed URL for a filter.
type StatusFeed struct {
	Format       string       `json:"format"`
	URL          string       `json:"url"`
	SlackCommand string       `json:"slackCommand,omitempty"`
	Filter       StatusFilter `json:"filter"`
	Validated    bool         `json:"validated"`
}
