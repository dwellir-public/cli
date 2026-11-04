package api

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

const whitelistsBasePath = "/v4/organization/ip-whitelists"

// Assignment kinds Marly reports. Brian adds an account_wide assignment for
// every key of an account-wide whitelist; manual ones come from assign.
const (
	AssignmentManual      = "manual"
	AssignmentAccountWide = "account_wide"
)

// Whitelist is an IP whitelist of the active organization. An enabled
// whitelist limits its API keys to the addresses in Entries.
type Whitelist struct {
	ID                int64                 `json:"id"`
	Name              string                `json:"name"`
	Description       *string               `json:"description"`
	Enabled           bool                  `json:"enabled"`
	ApplyToAllAPIKeys bool                  `json:"apply_to_all_api_keys"`
	Entries           []WhitelistEntry      `json:"entries"`
	AssignedAPIKeys   []WhitelistAssignment `json:"assigned_api_keys"`
}

type WhitelistEntry struct {
	ID          int64   `json:"id"`
	CIDR        string  `json:"cidr"`
	Description *string `json:"description,omitempty"`
	Enabled     bool    `json:"enabled"`
}

// WhitelistAssignment links a whitelist to one API key. Marly does not send
// the key name; the CLI fills Name from the key list.
type WhitelistAssignment struct {
	APIKeyID       int64  `json:"api_key_id"`
	APIKey         Secret `json:"api_key"`
	Name           string `json:"name,omitempty"`
	AssignmentKind string `json:"assignment_kind"`
	Enabled        bool   `json:"enabled"`
}

// CreateWhitelistInput is the POST body. Enabled has no omitempty: Marly
// defaults it to true, so false must be sent.
type CreateWhitelistInput struct {
	Name              string   `json:"name"`
	Description       *string  `json:"description,omitempty"`
	Entries           []string `json:"entries"`
	ApplyToAllAPIKeys bool     `json:"apply_to_all_api_keys"`
	Enabled           bool     `json:"enabled"`
}

// UpdateWhitelistInput is the PATCH body. Marly changes only the fields it
// receives and rejects null for every field except description.
type UpdateWhitelistInput struct {
	Name              *string  `json:"name,omitempty"`
	Description       *string  `json:"description,omitempty"`
	Entries           []string `json:"entries,omitempty"`
	ApplyToAllAPIKeys *bool    `json:"apply_to_all_api_keys,omitempty"`
	Enabled           *bool    `json:"enabled,omitempty"`
}

type WhitelistsAPI struct {
	client *Client
}

func NewWhitelistsAPI(client *Client) *WhitelistsAPI {
	return &WhitelistsAPI{client: client}
}

func (w *WhitelistsAPI) List() ([]Whitelist, error) {
	var whitelists []Whitelist
	err := w.client.Get(whitelistsBasePath, nil, &whitelists)
	return whitelists, err
}

func (w *WhitelistsAPI) Create(input CreateWhitelistInput) (*Whitelist, error) {
	var whitelist Whitelist
	if err := w.client.Post(whitelistsBasePath, input, &whitelist); err != nil {
		return nil, err
	}
	return &whitelist, nil
}

func (w *WhitelistsAPI) Update(id int64, input UpdateWhitelistInput) (*Whitelist, error) {
	var whitelist Whitelist
	if err := w.client.Patch(whitelistPath(id), input, &whitelist); err != nil {
		return nil, err
	}
	return &whitelist, nil
}

func (w *WhitelistsAPI) SetEnabled(id int64, enabled bool) (*Whitelist, error) {
	return w.Update(id, UpdateWhitelistInput{Enabled: &enabled})
}

func (w *WhitelistsAPI) Delete(id int64) error {
	return w.client.Delete(whitelistPath(id), nil)
}

// Assign adds a manual assignment of key to the whitelist.
func (w *WhitelistsAPI) Assign(id int64, key Secret) (*Whitelist, error) {
	var whitelist Whitelist
	if err := w.client.Post(whitelistKeyPath(id, key), nil, &whitelist); err != nil {
		return nil, err
	}
	return &whitelist, nil
}

// Unassign removes the manual assignment of key. Brian keeps account-wide
// assignments, so it cannot take a key off an account-wide whitelist.
func (w *WhitelistsAPI) Unassign(id int64, key Secret) (*Whitelist, error) {
	var whitelist Whitelist
	if err := w.client.Delete(whitelistKeyPath(id, key), &whitelist); err != nil {
		return nil, err
	}
	return &whitelist, nil
}

// Find returns the one whitelist that matches selector.
func (w *WhitelistsAPI) Find(selector string) (Whitelist, error) {
	whitelists, err := w.List()
	if err != nil {
		return Whitelist{}, err
	}
	return FindWhitelist(whitelists, selector)
}

// WhitelistLookupError reports a selector that matches no whitelist or more
// than one.
type WhitelistLookupError struct {
	Ambiguous  bool
	Candidates []string
}

func (e *WhitelistLookupError) Error() string {
	if e.Ambiguous {
		return "more than one IP whitelist matches; select it by ID"
	}
	return "no IP whitelist matches that ID or name"
}

// FindWhitelist matches selector against a whitelist ID or name
// (case-insensitive). Names are not unique, so a name can be ambiguous.
func FindWhitelist(whitelists []Whitelist, selector string) (Whitelist, error) {
	selector = strings.TrimSpace(selector)
	id, idErr := strconv.ParseInt(selector, 10, 64)
	var matches []Whitelist
	for _, whitelist := range whitelists {
		if selector == "" {
			break
		}
		if (idErr == nil && whitelist.ID == id) || strings.EqualFold(strings.TrimSpace(whitelist.Name), selector) {
			matches = append(matches, whitelist)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return Whitelist{}, &WhitelistLookupError{}
	default:
		candidates := make([]string, 0, len(matches))
		for _, whitelist := range matches {
			candidates = append(candidates, fmt.Sprintf("%s (ID %d)", whitelist.Name, whitelist.ID))
		}
		return Whitelist{}, &WhitelistLookupError{Ambiguous: true, Candidates: candidates}
	}
}

// NormalizeWhitelistAddress turns one IP address into the single-address
// CIDR the dashboard sends: /32 for IPv4, /128 for IPv6. Brian accepts any
// CIDR range, but the dashboard accepts single addresses only, and the CLI
// matches it. An explicit /32 or /128 is accepted; other prefixes are not.
func NormalizeWhitelistAddress(value string) (string, error) {
	value = strings.TrimSpace(value)
	address, prefix, hasPrefix := strings.Cut(value, "/")
	addr, err := netip.ParseAddr(address)
	if err != nil || addr.Zone() != "" {
		return "", fmt.Errorf("%q is not an IPv4 or IPv6 address", value)
	}
	hostPrefix := "32"
	if addr.Is6() {
		hostPrefix = "128"
	}
	if hasPrefix && prefix != hostPrefix {
		return "", fmt.Errorf("%q is a range; only single addresses are supported (/32 for IPv4, /128 for IPv6)", value)
	}
	return addr.String() + "/" + hostPrefix, nil
}

func whitelistPath(id int64) string {
	return fmt.Sprintf("%s/%d", whitelistsBasePath, id)
}

func whitelistKeyPath(id int64, key Secret) string {
	return fmt.Sprintf("%s/api-keys/%s", whitelistPath(id), key.Reveal())
}
