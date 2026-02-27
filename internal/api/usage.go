package api

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type UsageSummary struct {
	TotalRequests  int    `json:"total_requests"`
	TotalResponses int    `json:"total_responses"`
	RateLimited    int    `json:"rate_limited"`
	BillingStart   string `json:"billing_start,omitempty"`
	BillingEnd     string `json:"billing_end,omitempty"`
}

func (u *UsageSummary) UnmarshalJSON(data []byte) error {
	type alias UsageSummary
	var raw struct {
		alias
		RateLimitedCamel int `json:"rateLimited"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*u = UsageSummary(raw.alias)
	if u.RateLimited == 0 {
		u.RateLimited = raw.RateLimitedCamel
	}
	return nil
}

type UsageHistory struct {
	Timestamp  string `json:"timestamp"`
	APIKey     string `json:"api_key,omitempty"`
	APIKeyName string `json:"api_key_name,omitempty"`
	Domain     string `json:"domain,omitempty"`
	Method     string `json:"method,omitempty"`
	Requests   int    `json:"requests"`
	Responses  int    `json:"responses"`
}

func (u *UsageHistory) UnmarshalJSON(data []byte) error {
	type alias UsageHistory
	var raw struct {
		alias
		StartTime string `json:"start_time"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*u = UsageHistory(raw.alias)
	if u.Timestamp == "" {
		u.Timestamp = raw.StartTime
	}
	return nil
}

type RPSData struct {
	Timestamp string  `json:"timestamp"`
	RPS       float64 `json:"rps"`
}

type OrganizationRPS struct {
	RPS             float64 `json:"rps"`
	PeakRPS         float64 `json:"peak_rps"`
	LimitedRequests float64 `json:"limited_requests"`
}

func (r *OrganizationRPS) UnmarshalJSON(data []byte) error {
	type alias OrganizationRPS
	var raw struct {
		alias
		PeakRPSCamel         float64 `json:"peakRps"`
		LimitedRequestsCamel float64 `json:"limitedRequests"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*r = OrganizationRPS(raw.alias)
	if r.PeakRPS == 0 {
		r.PeakRPS = raw.PeakRPSCamel
	}
	if r.LimitedRequests == 0 {
		r.LimitedRequests = raw.LimitedRequestsCamel
	}
	return nil
}

type UsageBreakdown struct {
	Group       string `json:"group"`
	Requests    int    `json:"requests"`
	Responses   int    `json:"responses"`
	RateLimited int    `json:"rate_limited"`
}

type UsageAPI struct {
	client *Client
}

type analyticsRequest struct {
	Interval  string           `json:"interval,omitempty"`
	StartTime string           `json:"start_time,omitempty"`
	EndTime   string           `json:"end_time,omitempty"`
	Limit     int              `json:"limit,omitempty"`
	Offset    int              `json:"offset,omitempty"`
	Filter    *analyticsFilter `json:"filter,omitempty"`
}

type analyticsFilter struct {
	APIKeys []string `json:"api_keys,omitempty"`
	Domains []string `json:"domains,omitempty"`
}

func NewUsageAPI(client *Client) *UsageAPI {
	return &UsageAPI{client: client}
}

func (u *UsageAPI) Summary() (*UsageSummary, error) {
	var summary UsageSummary
	err := u.client.Get("/v4/organization/analytics/monthly_summary", nil, &summary)
	return &summary, err
}

func (u *UsageAPI) History(interval string, from string, to string, apiKey string, fqdn string, method string) ([]UsageHistory, error) {
	body := analyticsRequest{
		Interval: interval,
		Limit:    1000,
		Offset:   0,
	}
	if from != "" {
		body.StartTime = from
	}
	if to != "" {
		body.EndTime = to
	}
	filter := &analyticsFilter{}
	if trimmed := strings.TrimSpace(apiKey); trimmed != "" {
		filter.APIKeys = []string{trimmed}
	}
	if trimmed := strings.TrimSpace(fqdn); trimmed != "" {
		filter.Domains = []string{trimmed}
	}
	if len(filter.APIKeys) > 0 || len(filter.Domains) > 0 {
		body.Filter = filter
	}

	history := make([]UsageHistory, 0, body.Limit)
	maxRows := 50000
	for {
		var page []UsageHistory
		if err := u.client.Post("/v4/user/analytics", body, &page); err != nil {
			return nil, err
		}
		history = append(history, page...)
		if len(page) < body.Limit || len(history) >= maxRows {
			break
		}
		body.Offset += body.Limit
	}

	method = strings.TrimSpace(method)
	if method == "" {
		return history, nil
	}
	filtered := make([]UsageHistory, 0, len(history))
	for _, row := range history {
		if strings.EqualFold(row.Method, method) {
			filtered = append(filtered, row)
		}
	}
	return filtered, nil
}

func BuildUsageBreakdown(history []UsageHistory, keyFn func(UsageHistory) string) []UsageBreakdown {
	grouped := map[string]*UsageBreakdown{}
	for _, row := range history {
		key := strings.TrimSpace(keyFn(row))
		if key == "" {
			key = "unknown"
		}
		entry, ok := grouped[key]
		if !ok {
			entry = &UsageBreakdown{Group: key}
			grouped[key] = entry
		}
		entry.Requests += row.Requests
		entry.Responses += row.Responses
	}

	out := make([]UsageBreakdown, 0, len(grouped))
	for _, entry := range grouped {
		entry.RateLimited = max(0, entry.Requests-entry.Responses)
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Responses == out[j].Responses {
			return out[i].Group < out[j].Group
		}
		return out[i].Responses > out[j].Responses
	})
	return out
}

func BuildRPSTimeSeries(history []UsageHistory, interval string) []RPSData {
	if len(history) == 0 {
		return nil
	}

	type aggregate struct {
		timestamp string
		requests  int
	}

	byBucket := map[string]*aggregate{}
	for _, row := range history {
		if row.Timestamp == "" {
			continue
		}
		item, ok := byBucket[row.Timestamp]
		if !ok {
			item = &aggregate{timestamp: row.Timestamp}
			byBucket[row.Timestamp] = item
		}
		item.requests += row.Requests
	}

	seconds := intervalSeconds(interval)
	series := make([]RPSData, 0, len(byBucket))
	for _, item := range byBucket {
		series = append(series, RPSData{
			Timestamp: item.timestamp,
			RPS:       float64(item.requests) / seconds,
		})
	}
	sort.Slice(series, func(i, j int) bool {
		return series[i].Timestamp < series[j].Timestamp
	})
	return series
}

func intervalSeconds(interval string) float64 {
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "minute":
		return 60
	case "day":
		return 86400
	default:
		return 3600
	}
}

func (u *UsageAPI) RPS(interval string, from string, to string, apiKey string, fqdn string) ([]RPSData, error) {
	history, err := u.History(interval, from, to, apiKey, fqdn, "")
	if err != nil {
		return nil, err
	}
	return BuildRPSTimeSeries(history, interval), nil
}

func (u *UsageAPI) OrganizationRPS(interval string, from string, to string, apiKey string, fqdn string) (*OrganizationRPS, error) {
	body := analyticsRequest{
		Interval: interval,
	}
	if from != "" {
		body.StartTime = from
	}
	if to != "" {
		body.EndTime = to
	}
	filter := &analyticsFilter{}
	if trimmed := strings.TrimSpace(apiKey); trimmed != "" {
		filter.APIKeys = []string{trimmed}
	}
	if trimmed := strings.TrimSpace(fqdn); trimmed != "" {
		filter.Domains = []string{trimmed}
	}
	if len(filter.APIKeys) > 0 || len(filter.Domains) > 0 {
		body.Filter = filter
	}

	var stats OrganizationRPS
	if err := u.client.Post("/v4/organization/analytics/rps", body, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

func UsageDomain(row UsageHistory) string {
	return row.Domain
}

func UsageMethod(row UsageHistory) string {
	return row.Method
}

func UsageAPIKey(row UsageHistory) string {
	if strings.TrimSpace(row.APIKeyName) != "" {
		return row.APIKeyName
	}
	return row.APIKey
}

func UsageTimestamp(row UsageHistory) string {
	return row.Timestamp
}

func (u *UsageAPI) MethodBreakdown(interval string, from string, to string, apiKey string, fqdn string) ([]UsageBreakdown, error) {
	history, err := u.History(interval, from, to, apiKey, fqdn, "")
	if err != nil {
		return nil, err
	}
	return BuildUsageBreakdown(history, UsageMethod), nil
}

func (u *UsageAPI) EndpointBreakdown(interval string, from string, to string, apiKey string, fqdn string, method string) ([]UsageBreakdown, error) {
	history, err := u.History(interval, from, to, apiKey, fqdn, method)
	if err != nil {
		return nil, err
	}
	return BuildUsageBreakdown(history, UsageDomain), nil
}

func (u *UsageAPI) APIKeyBreakdown(interval string, from string, to string, fqdn string, method string) ([]UsageBreakdown, error) {
	history, err := u.History(interval, from, to, "", fqdn, method)
	if err != nil {
		return nil, err
	}
	return BuildUsageBreakdown(history, UsageAPIKey), nil
}

func (u *UsageAPI) TimeBreakdown(interval string, from string, to string, apiKey string, fqdn string, method string) ([]UsageBreakdown, error) {
	history, err := u.History(interval, from, to, apiKey, fqdn, method)
	if err != nil {
		return nil, err
	}
	return BuildUsageBreakdown(history, UsageTimestamp), nil
}

func (u *UsageAPI) RawHistory(interval string, from string, to string, apiKey string, fqdn string, method string) ([]UsageHistory, error) {
	return u.History(interval, from, to, apiKey, fqdn, method)
}

func CurrentBillingCycleRange(now time.Time, currentSub *CurrentSubscriptionWindow) (time.Time, time.Time) {
	start, _ := currentBillingCycleWindow(now, currentSub)
	return startOfUTCDay(start), startOfUTCDay(now.AddDate(0, 0, 1))
}

func currentBillingCycleWindow(now time.Time, currentSub *CurrentSubscriptionWindow) (time.Time, time.Time) {
	if currentSub != nil {
		if renewal := parseUsageDate(currentSub.GetRenewalDate()); renewal != nil {
			return usageCycleWindowFromAnchor(now, *renewal)
		}
		if start := parseUsageDate(currentSub.GetStartDate()); start != nil && !start.After(now) {
			return usageCycleWindowFromAnchor(now, *start)
		}
	}
	start := time.Date(now.UTC().Year(), now.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}

func usageCycleWindowFromAnchor(now time.Time, anchorInput time.Time) (time.Time, time.Time) {
	anchor := anchorInput.UTC()
	now = now.UTC()
	for !anchor.After(now) {
		anchor = anchor.AddDate(0, 1, 0)
	}
	start := anchor.AddDate(0, -1, 0)
	for now.Before(start) {
		anchor = anchor.AddDate(0, -1, 0)
		start = anchor.AddDate(0, -1, 0)
	}
	return start, anchor
}

func parseUsageDate(raw string) *time.Time {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}

func startOfUTCDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}
