package api

import "encoding/json"

type AccountInfo struct {
	Name                string                     `json:"name"`
	ServerLocation      string                     `json:"idealServerLocation,omitempty"`
	TaxID               string                     `json:"tax_id,omitempty"`
	UsageLimits         *SubscriptionInfo          `json:"usageLimits,omitempty"`
	CurrentSubscription *CurrentSubscriptionWindow `json:"currentSubscription,omitempty"`
	Subscription        *SubscriptionInfo          `json:"-"`
}

type CurrentSubscriptionWindow struct {
	StartDate   string `json:"startDate,omitempty"`
	RenewalDate string `json:"renewalDate,omitempty"`
}

type DiscountInfo struct {
	Applied    bool    `json:"applied"`
	Type       string  `json:"type"`
	Percentage float64 `json:"percentage,omitempty"`
	FlatAmount float64 `json:"flatAmount,omitempty"`
	Meta       struct {
		ApplyToAddOns bool `json:"applyToAddOns"`
	} `json:"meta,omitempty"`
}

type SubscriptionInfo struct {
	ID           int    `json:"id"`
	Name         string `json:"name,omitempty"`
	PlanName     string `json:"plan_name"`
	RateLimit    int    `json:"rate_limit"`
	BurstLimit   int    `json:"burst_limit"`
	MonthlyQuota *int   `json:"monthly_quota,omitempty"`
	DailyQuota   *int   `json:"daily_quota,omitempty"`
	APIKeysLimit int    `json:"api_keys_limit"`
}

func (s *SubscriptionInfo) UnmarshalJSON(data []byte) error {
	type alias SubscriptionInfo
	var raw struct {
		alias
		NameCamel         string `json:"name"`
		PlanNameCamel     string `json:"planName"`
		RateLimitCamel    int    `json:"rateLimit"`
		BurstLimitCamel   int    `json:"burstLimit"`
		MonthlyQuotaCamel *int   `json:"monthlyQuota"`
		DailyQuotaCamel   *int   `json:"dailyQuota"`
		APIKeysLimitCamel int    `json:"apiKeysLimit"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*s = SubscriptionInfo(raw.alias)
	if s.Name == "" {
		s.Name = raw.NameCamel
	}
	if s.PlanName == "" {
		s.PlanName = raw.PlanNameCamel
	}
	if s.RateLimit == 0 {
		s.RateLimit = raw.RateLimitCamel
	}
	if s.BurstLimit == 0 {
		s.BurstLimit = raw.BurstLimitCamel
	}
	if s.MonthlyQuota == nil {
		s.MonthlyQuota = raw.MonthlyQuotaCamel
	}
	if s.DailyQuota == nil {
		s.DailyQuota = raw.DailyQuotaCamel
	}
	if s.APIKeysLimit == 0 {
		s.APIKeysLimit = raw.APIKeysLimitCamel
	}
	return nil
}

func (s SubscriptionInfo) EffectivePlanName() string {
	if s.PlanName != "" {
		return s.PlanName
	}
	if s.Name != "" {
		return s.Name
	}
	return "Unknown"
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
	if info.UsageLimits != nil {
		info.Subscription = info.UsageLimits
		if info.Subscription.APIKeysLimit == 0 {
			if sub, subErr := a.Subscription(); subErr == nil && sub != nil {
				info.Subscription.APIKeysLimit = sub.APIKeysLimit
				if info.Subscription.PlanName == "" {
					info.Subscription.PlanName = sub.EffectivePlanName()
				}
			}
		}
	}
	return &info, err
}

func (a *AccountAPI) Subscription() (*SubscriptionInfo, error) {
	var sub SubscriptionInfo
	err := a.client.Get("/v3/user/subscription", nil, &sub)
	return &sub, err
}

func (a *AccountAPI) Discount() (*DiscountInfo, error) {
	var discount DiscountInfo
	err := a.client.Get("/v4/organization/information/outseta/discount", nil, &discount)
	return &discount, err
}
