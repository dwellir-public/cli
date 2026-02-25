package api

type AccountInfo struct {
	Name           string            `json:"name"`
	ServerLocation string            `json:"ideal_server_location,omitempty"`
	TaxID          string            `json:"tax_id,omitempty"`
	Subscription   *SubscriptionInfo `json:"current_subscription,omitempty"`
}

type SubscriptionInfo struct {
	PlanName     string `json:"plan_name"`
	RateLimit    int    `json:"rate_limit"`
	BurstLimit   int    `json:"burst_limit"`
	MonthlyQuota *int   `json:"monthly_quota,omitempty"`
	DailyQuota   *int   `json:"daily_quota,omitempty"`
	APIKeysLimit int    `json:"api_keys_limit"`
}

type AccountAPI struct {
	client *Client
}

func NewAccountAPI(client *Client) *AccountAPI {
	return &AccountAPI{client: client}
}

func (a *AccountAPI) Info() (*AccountInfo, error) {
	var info AccountInfo
	err := a.client.Get("/v4/organization/information/outseta", nil, &info)
	return &info, err
}

func (a *AccountAPI) Subscription() (*SubscriptionInfo, error) {
	var sub SubscriptionInfo
	err := a.client.Get("/v3/user/subscription", nil, &sub)
	return &sub, err
}
