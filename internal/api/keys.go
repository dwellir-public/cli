package api

import (
	"context"
	"errors"
	"fmt"
	"net"
)

type APIKey struct {
	APIKey       string `json:"api_key"`
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	DailyQuota   *int   `json:"daily_quota,omitempty"`
	MonthlyQuota *int   `json:"monthly_quota,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type CreateKeyInput struct {
	Name         string `json:"name,omitempty"`
	DailyQuota   *int   `json:"daily_quota,omitempty"`
	MonthlyQuota *int   `json:"monthly_quota,omitempty"`
}

type UpdateKeyInput struct {
	Name         *string `json:"name,omitempty"`
	Enabled      *bool   `json:"enabled,omitempty"`
	DailyQuota   *int    `json:"daily_quota,omitempty"`
	MonthlyQuota *int    `json:"monthly_quota,omitempty"`
}

type KeysAPI struct {
	client *Client
}

func NewKeysAPI(client *Client) *KeysAPI {
	return &KeysAPI{client: client}
}

func (k *KeysAPI) List() ([]APIKey, error) {
	var keys []APIKey
	err := k.client.Get("/v3/user/apikeys", nil, &keys)
	return keys, err
}

func (k *KeysAPI) Create(input CreateKeyInput) (*APIKey, error) {
	var key APIKey
	err := k.client.Post("/v3/user/apikey", input, &key)
	return &key, err
}

func (k *KeysAPI) Update(apiKey string, input UpdateKeyInput) (*APIKey, error) {
	current, err := k.lookup(apiKey)
	if err != nil {
		return nil, err
	}

	payload := struct {
		Name         string `json:"name"`
		Enabled      bool   `json:"enabled"`
		DailyQuota   *int   `json:"daily_quota"`
		MonthlyQuota *int   `json:"monthly_quota"`
	}{
		Name:         current.Name,
		Enabled:      current.Enabled,
		DailyQuota:   current.DailyQuota,
		MonthlyQuota: current.MonthlyQuota,
	}

	if input.Name != nil {
		payload.Name = *input.Name
	}
	if input.Enabled != nil {
		payload.Enabled = *input.Enabled
	}
	if input.DailyQuota != nil {
		payload.DailyQuota = input.DailyQuota
	}
	if input.MonthlyQuota != nil {
		payload.MonthlyQuota = input.MonthlyQuota
	}

	var key APIKey
	err = k.client.Post(fmt.Sprintf("/user/apikey/%s", apiKey), payload, &key)
	if err != nil && isTimeoutError(err) {
		err = k.client.Post(fmt.Sprintf("/user/apikey/%s", apiKey), payload, &key)
	}
	return &key, err
}

func (k *KeysAPI) Delete(apiKey string) error {
	return k.client.Delete(fmt.Sprintf("/user/apikey/%s", apiKey), nil)
}

func (k *KeysAPI) Enable(apiKey string) (*APIKey, error) {
	enabled := true
	return k.Update(apiKey, UpdateKeyInput{Enabled: &enabled})
}

func (k *KeysAPI) Disable(apiKey string) (*APIKey, error) {
	enabled := false
	return k.Update(apiKey, UpdateKeyInput{Enabled: &enabled})
}

func (k *KeysAPI) lookup(apiKey string) (*APIKey, error) {
	keys, err := k.List()
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		if key.APIKey == apiKey {
			copy := key
			return &copy, nil
		}
	}
	return nil, fmt.Errorf("api key not found: %s", apiKey)
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
