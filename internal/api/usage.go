package api

import "time"

type UsageSummary struct {
	TotalRequests  int    `json:"total_requests"`
	TotalResponses int    `json:"total_responses"`
	RateLimited    int    `json:"rate_limited"`
	BillingStart   string `json:"billing_start,omitempty"`
	BillingEnd     string `json:"billing_end,omitempty"`
}

type UsageHistory struct {
	Timestamp string `json:"timestamp"`
	Requests  int    `json:"requests"`
	Responses int    `json:"responses"`
}

type RPSData struct {
	Timestamp string  `json:"timestamp"`
	RPS       float64 `json:"rps"`
}

type UsageAPI struct {
	client *Client
}

type analyticsRequest struct {
	Interval  string `json:"interval,omitempty"`
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}

func NewUsageAPI(client *Client) *UsageAPI {
	return &UsageAPI{client: client}
}

func (u *UsageAPI) Summary() (*UsageSummary, error) {
	var summary UsageSummary
	err := u.client.Get("/v4/organization/analytics/monthly_summary", nil, &summary)
	return &summary, err
}

func (u *UsageAPI) History(interval string, from string, to string) ([]UsageHistory, error) {
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
	var history []UsageHistory
	err := u.client.Post("/v4/organization/analytics", body, &history)
	return history, err
}

func (u *UsageAPI) RPS() ([]RPSData, error) {
	body := analyticsRequest{
		Interval: "hour",
		Limit:    1000,
		Offset:   0,
	}
	var payload struct {
		RPS float64 `json:"rps"`
	}
	err := u.client.Post("/v4/organization/analytics/rps", body, &payload)
	if err != nil {
		return nil, err
	}
	rps := []RPSData{
		{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			RPS:       payload.RPS,
		},
	}
	return rps, err
}
