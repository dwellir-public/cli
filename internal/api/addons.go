package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CatalogAddOn is one entry of the Outseta add-on catalog as marly serves it
// from GET /v4/billing/addons (pymarly/outseta/models.py, BillingAddOn).
//
// The rate fields are pointers on purpose: nil means Outseta does not offer the
// add-on at that term, which is not the same as a free add-on priced at 0.
type CatalogAddOn struct {
	UID                 string   `json:"uid"`
	Name                string   `json:"name,omitempty"`
	MonthlyRate         *float64 `json:"monthlyRate,omitempty"`
	AnnualRate          *float64 `json:"annualRate,omitempty"`
	QuarterlyRate       *float64 `json:"quarterlyRate,omitempty"`
	OneTimeRate         *float64 `json:"oneTimeRate,omitempty"`
	SetupFee            *float64 `json:"setupFee,omitempty"`
	BillingAddOnType    *int     `json:"billingAddOnType,omitempty"`
	UnitOfMeasure       string   `json:"unitOfMeasure,omitempty"`
	IsBilledDuringTrial *bool    `json:"isBilledDuringTrial,omitempty"`
}

// ActiveAddOn is one add-on instance on the caller's current subscription.
//
// InstanceUID identifies the instance, not the product, and it is the key the
// cancel route takes. Always show it.
type ActiveAddOn struct {
	InstanceUID string `json:"instanceUid"`
	AddOnUID    string `json:"addOnUid,omitempty"`
	Name        string `json:"name,omitempty"`
	Quantity    *int   `json:"quantity,omitempty"`
	StartDate   string `json:"startDate,omitempty"`
	// EndDate set means the instance is already scheduled to stop renewing.
	EndDate     string `json:"endDate,omitempty"`
	RenewalDate string `json:"renewalDate,omitempty"`
}

// PaymentMethod is the card on file for the organization.
type PaymentMethod struct {
	Brand    string     `json:"brand,omitempty"`
	Last4    string     `json:"last4,omitempty"`
	ExpMonth flexString `json:"expMonth,omitempty"`
	ExpYear  flexString `json:"expYear,omitempty"`
	Name     string     `json:"name,omitempty"`
}

// PaymentMethodResult wraps PaymentMethod with an explicit boolean, because
// marly signals "no card on file" with a bare JSON null body and exposes no
// hasPaymentMethod flag of its own.
type PaymentMethodResult struct {
	HasPaymentMethod bool           `json:"hasPaymentMethod"`
	PaymentMethod    *PaymentMethod `json:"paymentMethod,omitempty"`
}

// flexString accepts a JSON string, number, or null. Outseta returns card
// expiry fields inconsistently typed and marly passes them through untouched.
type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		*f = ""
		return nil
	}
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		*f = flexString(asString)
		return nil
	}
	var asNumber json.Number
	if err := json.Unmarshal(data, &asNumber); err != nil {
		return err
	}
	*f = flexString(asNumber.String())
	return nil
}

// AddonKind classifies a catalog entry for the CLI's purchase view.
type AddonKind string

const (
	// AddonKindPremium unlocks access to a restricted endpoint.
	AddonKindPremium AddonKind = "premium"
	// AddonKindUnlimited buys an unmetered RPS tier on one endpoint.
	AddonKindUnlimited AddonKind = "unlimited"
	// AddonKindOther is any other catalog product.
	AddonKindOther AddonKind = "other"
)

// Ownership states for a catalog entry. These deliberately differ from
// PremiumEndpointStatus: "locked" makes no sense in a list of things you can
// buy, so the locked case is reported as available instead.
const (
	AddonStatusActive       = "active"
	AddonStatusTrial        = "trial"
	AddonStatusTrialExpired = "trial-expired"
	AddonStatusAvailable    = "available"
)

// AddonCatalogEntry is one row of `dwellir addons list`.
type AddonCatalogEntry struct {
	AddOnUID    string    `json:"addOnUid"`
	Name        string    `json:"name,omitempty"`
	Kind        AddonKind `json:"kind"`
	Endpoint    string    `json:"endpoint,omitempty"`
	Chain       string    `json:"chain,omitempty"`
	RPS         *int      `json:"rps,omitempty"`
	MonthlyRate *float64  `json:"monthlyRate,omitempty"`
	AnnualRate  *float64  `json:"annualRate,omitempty"`
	Status      string    `json:"status"`
	TrialEndsAt string    `json:"trialEndsAt,omitempty"`
}

// AddonTrial is one premium-endpoint trial on the caller's organization.
type AddonTrial struct {
	Endpoint string `json:"endpoint"`
	Chain    string `json:"chain,omitempty"`
	Status   string `json:"status"`
	EndsAt   string `json:"endsAt,omitempty"`
}

// AddonNotice is a typed, non-fatal problem that changed what a command could
// report. It is rendered, never swallowed.
type AddonNotice struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Help    string `json:"help,omitempty"`
}

// Sources for the active add-on list in AddonsStatus.
const (
	// AddonsSourceBilling is GET /v4/billing/addons/active.
	AddonsSourceBilling = "billing"
	// AddonsSourceAccount is the currentSubscription block of
	// GET /v4/organization/information/outseta, used when the billing route
	// rejects the CLI token.
	AddonsSourceAccount = "account"
)

// AddonsStatus is the payload of `dwellir addons status`.
type AddonsStatus struct {
	AddOns       []ActiveAddOn `json:"addOns"`
	AddOnsSource string        `json:"addOnsSource"`
	Trials       []AddonTrial  `json:"trials"`
	Notices      []AddonNotice `json:"notices,omitempty"`
}

type AddonsAPI struct {
	client *Client
}

func NewAddonsAPI(client *Client) *AddonsAPI {
	return &AddonsAPI{client: client}
}

// Catalog returns the Outseta add-on catalog. This route already accepts CLI
// tokens.
func (a *AddonsAPI) Catalog() ([]CatalogAddOn, error) {
	var response struct {
		Addons []CatalogAddOn `json:"addons"`
	}
	if err := a.client.Get("/v4/billing/addons", nil, &response); err != nil {
		return nil, err
	}
	return response.Addons, nil
}

// Active returns the add-on instances on the caller's subscription.
//
// Marly guards this route with a cookie-only dependency today, so a CLI token
// gets 401. Callers must handle that: see ActiveAddOnsFromAccount for the
// fallback the CLI uses until the marly change lands.
func (a *AddonsAPI) Active() ([]ActiveAddOn, error) {
	var response struct {
		AddOns []ActiveAddOn `json:"addOns"`
	}
	if err := a.client.Get("/v4/billing/addons/active", nil, &response); err != nil {
		return nil, err
	}
	return response.AddOns, nil
}

// PaymentMethod returns nil with a nil error when no card is on file. Marly
// answers that case with a bare JSON `null` body.
//
// The body is read as raw JSON rather than straight into the struct so that an
// empty 200 stays distinguishable from `null`. The client skips unmarshalling a
// zero-length body, which would otherwise leave the pointer nil and report a
// truncated or proxy-mangled response as "no card on file".
func (a *AddonsAPI) PaymentMethod() (*PaymentMethod, error) {
	var raw json.RawMessage
	if err := a.client.Get("/v4/billing/payment-method", nil, &raw); err != nil {
		return nil, err
	}

	body := bytes.TrimSpace(raw)
	if len(body) == 0 {
		return nil, errors.New("payment method response was empty")
	}
	if string(body) == "null" {
		return nil, nil
	}

	var method PaymentMethod
	if err := json.Unmarshal(body, &method); err != nil {
		return nil, fmt.Errorf("parsing payment method: %w", err)
	}
	return &method, nil
}

// ActiveAddOnsFromAccount reads the same add-on instances out of the account
// payload, which does accept CLI tokens.
//
// Marly builds both responses from the same SubscriptionAddOn model
// (pymarly/outseta/models.py:78-90), so the fallback carries every field the
// billing route does, including quantity and renewal date.
func ActiveAddOnsFromAccount(account *AccountInfo) []ActiveAddOn {
	if account == nil || account.CurrentSubscription == nil {
		return nil
	}

	addOns := make([]ActiveAddOn, 0, len(account.CurrentSubscription.SubscriptionAddOns))
	for _, addOn := range account.CurrentSubscription.SubscriptionAddOns {
		instanceUID := strings.TrimSpace(addOn.UID)
		if instanceUID == "" {
			// Without the instance uid there is nothing to cancel against, so
			// the row would be actively misleading. Marly drops these too.
			continue
		}
		addOns = append(addOns, ActiveAddOn{
			InstanceUID: instanceUID,
			AddOnUID:    addOn.CanonicalAddOnUID(),
			Name:        strings.TrimSpace(addOn.Name),
			Quantity:    addOn.Quantity,
			StartDate:   strings.TrimSpace(addOn.StartDate),
			EndDate:     strings.TrimSpace(addOn.EndDate),
			RenewalDate: strings.TrimSpace(addOn.RenewalDate),
		})
	}
	return addOns
}

// unlimitedAddOnNamePattern matches the deterministic name brian gives an
// auto-provisioned unlimited-RPS product, "Unlimited {rps} RPS - {fqdn}", plus
// the optional "[token]" suffix that avoids Outseta name collisions.
var unlimitedAddOnNamePattern = regexp.MustCompile(`(?i)^unlimited\s+(\d+)\s+rps\s*-\s*(\S+?)(?:\s+\[[^\]]*\])?$`)

// ParseUnlimitedAddOnName reports whether name is an unlimited-RPS add-on, and
// if so returns its tier and the host slug of the endpoint it covers.
func ParseUnlimitedAddOnName(name string) (int, string, bool) {
	match := unlimitedAddOnNamePattern.FindStringSubmatch(strings.TrimSpace(name))
	if match == nil {
		return 0, "", false
	}
	rps, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, "", false
	}
	return rps, NormalizeEndpointSlug(match[2]), true
}

// BuildAddonCatalog classifies the Outseta catalog and folds in what the
// account already owns, so a caller can see price and ownership in one pass.
//
// Entries are sorted by kind, then endpoint, then name, so repeated runs and
// golden-output tests stay stable: Outseta does not promise an order.
func BuildAddonCatalog(catalog []CatalogAddOn, account *AccountInfo) []AddonCatalogEntry {
	rulesByUID := make(map[string]premiumEndpointRule, len(premiumEndpointRules))
	for _, rule := range premiumEndpointRules {
		uid := strings.ToLower(strings.TrimSpace(rule.addOnUID))
		if uid == "" {
			continue
		}
		if _, seen := rulesByUID[uid]; seen {
			// Alias slugs share one uid. The first rule is the canonical one.
			continue
		}
		rulesByUID[uid] = rule
	}

	stateByHost, activeAddOnUIDs := accountPremiumState(account)
	now := time.Now().UTC()

	entries := make([]AddonCatalogEntry, 0, len(catalog))
	for _, addOn := range catalog {
		uid := strings.TrimSpace(addOn.UID)
		if uid == "" {
			continue
		}

		entry := AddonCatalogEntry{
			AddOnUID:    uid,
			Name:        strings.TrimSpace(addOn.Name),
			Kind:        AddonKindOther,
			MonthlyRate: addOn.MonthlyRate,
			AnnualRate:  addOn.AnnualRate,
			Status:      AddonStatusAvailable,
		}

		if _, owned := activeAddOnUIDs[strings.ToLower(uid)]; owned {
			entry.Status = AddonStatusActive
		}

		switch rule, isPremium := rulesByUID[strings.ToLower(uid)]; {
		case isPremium:
			entry.Kind = AddonKindPremium
			entry.Endpoint = rule.hostSlug
			entry.Chain = rule.chainName
			status, trialEndsAt := resolvePremiumStatus(rule, stateByHost[rule.hostSlug], activeAddOnUIDs, now)
			entry.Status = addonStatusFromPremiumStatus(status)
			entry.TrialEndsAt = trialEndsAt
		default:
			if rps, hostSlug, isUnlimited := ParseUnlimitedAddOnName(entry.Name); isUnlimited {
				tier := rps
				entry.Kind = AddonKindUnlimited
				entry.Endpoint = hostSlug
				entry.RPS = &tier
			}
		}

		entries = append(entries, entry)
	}

	sortAddonCatalog(entries)
	return entries
}

// BuildAddonsStatus pairs the caller's active add-on instances with the
// premium-endpoint trials recorded on the account.
func BuildAddonsStatus(active []ActiveAddOn, account *AccountInfo, source string) AddonsStatus {
	status := AddonsStatus{
		AddOns:       active,
		AddOnsSource: source,
		Trials:       buildAddonTrials(account),
	}
	if status.AddOns == nil {
		status.AddOns = []ActiveAddOn{}
	}
	return status
}

func buildAddonTrials(account *AccountInfo) []AddonTrial {
	stateByHost, activeAddOnUIDs := accountPremiumState(account)
	if len(stateByHost) == 0 {
		return []AddonTrial{}
	}

	chainByHost := make(map[string]string, len(premiumEndpointRules))
	rulesByHost := make(map[string]premiumEndpointRule, len(premiumEndpointRules))
	for _, rule := range premiumEndpointRules {
		host := strings.ToLower(rule.hostSlug)
		chainByHost[host] = rule.chainName
		rulesByHost[host] = rule
	}

	now := time.Now().UTC()
	trials := make([]AddonTrial, 0, len(stateByHost))
	for host, entry := range stateByHost {
		status, endsAt := resolvePremiumStatus(rulesByHost[host], entry, activeAddOnUIDs, now)
		if status != PremiumStatusTrialActive && status != PremiumStatusTrialExpired {
			// An entry that resolved to locked or addon-active is not a trial.
			// The add-on table already covers the addon-active case.
			continue
		}
		trials = append(trials, AddonTrial{
			Endpoint: host,
			Chain:    chainByHost[host],
			Status:   string(status),
			EndsAt:   endsAt,
		})
	}

	sort.Slice(trials, func(i, j int) bool { return trials[i].Endpoint < trials[j].Endpoint })
	return trials
}

// accountPremiumState indexes the two account fields that say what an
// organization already has: recorded trial state, and live add-on uids.
func accountPremiumState(account *AccountInfo) (map[string]PremiumEndpointStateEntry, map[string]struct{}) {
	stateByHost := make(map[string]PremiumEndpointStateEntry)
	activeAddOnUIDs := make(map[string]struct{})
	if account == nil {
		return stateByHost, activeAddOnUIDs
	}

	for _, entry := range account.PremiumEndpointState {
		host := strings.ToLower(strings.TrimSpace(entry.HostSlug))
		if host == "" {
			continue
		}
		stateByHost[host] = entry
	}

	if account.CurrentSubscription != nil {
		now := time.Now().UTC()
		for _, addOn := range account.CurrentSubscription.SubscriptionAddOns {
			if !isSubscriptionAddOnActive(addOn.EndDate, now) {
				continue
			}
			uid := strings.ToLower(strings.TrimSpace(addOn.CanonicalAddOnUID()))
			if uid == "" {
				continue
			}
			activeAddOnUIDs[uid] = struct{}{}
		}
	}

	return stateByHost, activeAddOnUIDs
}

func addonStatusFromPremiumStatus(status PremiumEndpointStatus) string {
	switch status {
	case PremiumStatusAddonActive:
		return AddonStatusActive
	case PremiumStatusTrialActive:
		return AddonStatusTrial
	case PremiumStatusTrialExpired:
		return AddonStatusTrialExpired
	default:
		return AddonStatusAvailable
	}
}

// addonKindOrder keeps the buyable rows above the rest of the Outseta catalog.
var addonKindOrder = map[AddonKind]int{
	AddonKindPremium:   0,
	AddonKindUnlimited: 1,
	AddonKindOther:     2,
}

func sortAddonCatalog(entries []AddonCatalogEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		if addonKindOrder[left.Kind] != addonKindOrder[right.Kind] {
			return addonKindOrder[left.Kind] < addonKindOrder[right.Kind]
		}
		if left.Endpoint != right.Endpoint {
			return left.Endpoint < right.Endpoint
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		return left.AddOnUID < right.AddOnUID
	})
}

// FilterAddonCatalog narrows a catalog by kind and by a chain or endpoint
// substring. Passing both premium and unlimited keeps both kinds.
func FilterAddonCatalog(entries []AddonCatalogEntry, premium bool, unlimited bool, chain string) []AddonCatalogEntry {
	needle := strings.ToLower(strings.TrimSpace(chain))
	filtered := make([]AddonCatalogEntry, 0, len(entries))

	for _, entry := range entries {
		if premium || unlimited {
			wanted := (premium && entry.Kind == AddonKindPremium) ||
				(unlimited && entry.Kind == AddonKindUnlimited)
			if !wanted {
				continue
			}
		}
		if needle != "" && !addonEntryMatches(entry, needle) {
			continue
		}
		filtered = append(filtered, entry)
	}

	return filtered
}

func addonEntryMatches(entry AddonCatalogEntry, needle string) bool {
	// The needle is also matched against a normalized form of itself so that
	// `--chain https://api-kusama-sidecar.n.dwellir.com` works like the slug.
	candidates := []string{
		strings.ToLower(entry.Chain),
		strings.ToLower(entry.Endpoint),
		strings.ToLower(entry.Name),
	}
	normalized := NormalizeEndpointSlug(needle)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if strings.Contains(candidate, needle) {
			return true
		}
		if normalized != "" && normalized != needle && strings.Contains(candidate, normalized) {
			return true
		}
	}
	return false
}
