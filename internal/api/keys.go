package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
)

const apiKeysBasePath = "/v4/organization/apikeys"

type APIKey struct {
	APIKey       Secret `json:"api_key"`
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	DailyQuota   *int   `json:"daily_quota,omitempty"`
	MonthlyQuota *int   `json:"monthly_quota,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type DeleteKeyResult struct {
	CleanupPending bool `json:"cleanup_pending"`
}

type deleteKeyResponse struct {
	CleanupPending       bool     `json:"cleanup_pending"`
	UnreachableEndpoints []string `json:"unreachable_endpoints,omitempty"`
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
	err := k.client.Get(apiKeysBasePath, nil, &keys)
	return keys, err
}

func (k *KeysAPI) Create(input CreateKeyInput) (*APIKey, error) {
	var key APIKey
	err := k.client.Post(apiKeysBasePath, input, &key)
	return &key, err
}

// Update sends the full key settings, starting from current, because the
// API replaces every field.
func (k *KeysAPI) Update(current APIKey, input UpdateKeyInput) (*APIKey, error) {
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

	path := k.keyPath(current.APIKey)
	var key APIKey
	err := k.client.Post(path, payload, &key)
	if err != nil && isTimeoutError(err) {
		err = k.client.Post(path, payload, &key)
	}
	return &key, err
}

func (k *KeysAPI) Delete(key Secret) (*DeleteKeyResult, error) {
	result := &deleteKeyResponse{}
	statusCode, err := k.client.DeleteWithStatus(k.keyPath(key), result)
	if err != nil {
		return nil, err
	}
	return &DeleteKeyResult{
		CleanupPending: result.CleanupPending || statusCode == 202,
	}, nil
}

func (k *KeysAPI) Enable(current APIKey) (*APIKey, error) {
	enabled := true
	return k.Update(current, UpdateKeyInput{Enabled: &enabled})
}

func (k *KeysAPI) Disable(current APIKey) (*APIKey, error) {
	enabled := false
	return k.Update(current, UpdateKeyInput{Enabled: &enabled})
}

// Find returns the one key that matches selector.
func (k *KeysAPI) Find(selector string) (APIKey, error) {
	keys, err := k.List()
	if err != nil {
		return APIKey{}, err
	}
	return FindKey(keys, selector)
}

// KeyLookupError reports a selector that matches no key or more than one.
type KeyLookupError struct {
	Ambiguous  bool
	Candidates []string
}

func (e *KeyLookupError) Error() string {
	if e.Ambiguous {
		return "more than one API key matches; use a unique key name"
	}
	return "no API key matches that name or value"
}

// FindKey matches selector against a key name (case-insensitive), the masked
// value the CLI prints, or the full value.
func FindKey(keys []APIKey, selector string) (APIKey, error) {
	selector = strings.TrimSpace(selector)
	var matches []APIKey
	for _, key := range keys {
		if selector != "" && (strings.EqualFold(strings.TrimSpace(key.Name), selector) ||
			key.APIKey.Masked() == selector || key.APIKey.Reveal() == selector) {
			matches = append(matches, key)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return APIKey{}, &KeyLookupError{}
	default:
		candidates := make([]string, 0, len(matches))
		for _, key := range matches {
			candidates = append(candidates, fmt.Sprintf("%s (%s)", key.Name, key.APIKey.Masked()))
		}
		return APIKey{}, &KeyLookupError{Ambiguous: true, Candidates: candidates}
	}
}

func (k *KeysAPI) keyPath(key Secret) string {
	k.client.MaskInErrors(key)
	return fmt.Sprintf("%s/%s", apiKeysBasePath, key.Reveal())
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
