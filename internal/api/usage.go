package api

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

func NewUsageAPI(client *Client) *UsageAPI {
	return &UsageAPI{client: client}
}

func (u *UsageAPI) Summary() (*UsageSummary, error) {
	var summary UsageSummary
	err := u.client.Get("/v4/organization/analytics/monthly_summary", nil, &summary)
	return &summary, err
}

func (u *UsageAPI) History(interval string, from string, to string) ([]UsageHistory, error) {
	body := map[string]string{"interval": interval}
	if from != "" {
		body["from"] = from
	}
	if to != "" {
		body["to"] = to
	}
	var history []UsageHistory
	err := u.client.Post("/v4/organization/analytics", body, &history)
	return history, err
}

func (u *UsageAPI) RPS() ([]RPSData, error) {
	var rps []RPSData
	err := u.client.Post("/v4/organization/analytics/rps", nil, &rps)
	return rps, err
}
