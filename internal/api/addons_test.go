package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNormalizeEndpointSlug(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "already a slug", input: "api-kusama-sidecar", want: "api-kusama-sidecar"},
		{name: "uppercase slug", input: "API-Kusama-Sidecar", want: "api-kusama-sidecar"},
		{name: "surrounding whitespace", input: "  api-kusama-sidecar  ", want: "api-kusama-sidecar"},
		{name: "fqdn", input: "api-kusama-sidecar.n.dwellir.com", want: "api-kusama-sidecar"},
		{name: "https url", input: "https://api-kusama-sidecar.n.dwellir.com/<key>", want: "api-kusama-sidecar"},
		{name: "wss url", input: "wss://api-kusama-sidecar.n.dwellir.com/<key>/ws", want: "api-kusama-sidecar"},
		{name: "url with port", input: "https://api-kusama-sidecar.n.dwellir.com:443/<key>", want: "api-kusama-sidecar"},
		{name: "host with port", input: "api-kusama-sidecar.n.dwellir.com:443", want: "api-kusama-sidecar"},
		{name: "host with path", input: "api-kusama-sidecar.n.dwellir.com/<key>", want: "api-kusama-sidecar"},
		{name: "trailing dot", input: "api-kusama-sidecar.", want: "api-kusama-sidecar"},
		{name: "empty", input: "", want: ""},
		{name: "whitespace only", input: "   ", want: ""},
		{name: "scheme with no host", input: "https://", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeEndpointSlug(tt.input); got != tt.want {
				t.Fatalf("NormalizeEndpointSlug(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseUnlimitedAddOnName(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantRPS      int
		wantEndpoint string
		wantOK       bool
	}{
		{
			name:         "canonical name",
			input:        "Unlimited 200 RPS - api-base-mainnet.n.dwellir.com",
			wantRPS:      200,
			wantEndpoint: "api-base-mainnet",
			wantOK:       true,
		},
		{
			name:         "collision token suffix",
			input:        "Unlimited 500 RPS - api-base-mainnet.n.dwellir.com [a1b2c3]",
			wantRPS:      500,
			wantEndpoint: "api-base-mainnet",
			wantOK:       true,
		},
		{
			name:         "lowercase",
			input:        "unlimited 50 rps - api-base-mainnet.n.dwellir.com",
			wantRPS:      50,
			wantEndpoint: "api-base-mainnet",
			wantOK:       true,
		},
		{name: "premium add-on name", input: "Hyperliquid Orderbook Service", wantOK: false},
		{name: "missing tier", input: "Unlimited RPS - api-base-mainnet.n.dwellir.com", wantOK: false},
		{name: "empty", input: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rps, endpoint, ok := ParseUnlimitedAddOnName(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if rps != tt.wantRPS {
				t.Fatalf("rps = %d, want %d", rps, tt.wantRPS)
			}
			if endpoint != tt.wantEndpoint {
				t.Fatalf("endpoint = %q, want %q", endpoint, tt.wantEndpoint)
			}
		})
	}
}

func TestBuildAddonCatalogClassifiesAndMergesOwnership(t *testing.T) {
	trialEnds := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)
	monthly := 199.0
	catalog := []CatalogAddOn{
		{UID: "zzz-unknown", Name: "Some Other Product", MonthlyRate: &monthly},
		{UID: "gWKew2Qp", Name: "Hyperliquid Orderbook Service", MonthlyRate: &monthly},
		{UID: "amRyMMWJ", Name: "Hyperliquid Orderbook Testnet"},
		{UID: "y9gpzRWM", Name: "Kusama Sidecar"},
		{UID: "auto-1", Name: "Unlimited 200 RPS - api-base-mainnet.n.dwellir.com"},
		{UID: "", Name: "Dropped: no uid"},
	}
	account := &AccountInfo{
		PremiumEndpointState: PremiumEndpointState{
			{HostSlug: "api-hyperliquid-testnet-orderbook", Status: PremiumStatusTrialActive, TrialEndsAt: trialEnds},
			{HostSlug: "api-kusama-sidecar", Status: PremiumStatusTrialExpired},
		},
		CurrentSubscription: &CurrentSubscriptionWindow{
			SubscriptionAddOns: []OutsetaSubscriptionAddOn{
				{UID: "inst-1", AddOnUID: "gWKew2Qp"},
				{UID: "inst-2", AddOnUID: "auto-1"},
			},
		},
	}

	entries := BuildAddonCatalog(catalog, account)
	if len(entries) != 5 {
		t.Fatalf("expected the uid-less entry to be dropped, got %d entries", len(entries))
	}

	byUID := make(map[string]AddonCatalogEntry, len(entries))
	for _, entry := range entries {
		byUID[entry.AddOnUID] = entry
	}

	owned := byUID["gWKew2Qp"]
	if owned.Kind != AddonKindPremium || owned.Status != AddonStatusActive {
		t.Fatalf("owned premium add-on = %+v", owned)
	}
	if owned.Endpoint != "api-hyperliquid-mainnet-orderbook" || owned.Chain != "Hyperliquid Orderbook" {
		t.Fatalf("premium entry not resolved from the rule table: %+v", owned)
	}
	if owned.MonthlyRate == nil || *owned.MonthlyRate != monthly {
		t.Fatalf("expected the catalog price to be carried through: %+v", owned)
	}

	onTrial := byUID["amRyMMWJ"]
	if onTrial.Status != AddonStatusTrial || onTrial.TrialEndsAt != trialEnds {
		t.Fatalf("trialing premium add-on = %+v", onTrial)
	}

	expired := byUID["y9gpzRWM"]
	if expired.Status != AddonStatusTrialExpired {
		t.Fatalf("expired trial add-on = %+v", expired)
	}

	unlimited := byUID["auto-1"]
	if unlimited.Kind != AddonKindUnlimited || unlimited.Endpoint != "api-base-mainnet" {
		t.Fatalf("unlimited add-on = %+v", unlimited)
	}
	if unlimited.RPS == nil || *unlimited.RPS != 200 {
		t.Fatalf("expected the 200 RPS tier, got %+v", unlimited.RPS)
	}
	if unlimited.Status != AddonStatusActive {
		t.Fatalf("expected the owned unlimited tier to read active, got %q", unlimited.Status)
	}

	other := byUID["zzz-unknown"]
	if other.Kind != AddonKindOther || other.Status != AddonStatusAvailable {
		t.Fatalf("unclassified add-on = %+v", other)
	}

	// Premium first, then unlimited, then the rest.
	if entries[0].Kind != AddonKindPremium || entries[len(entries)-1].Kind != AddonKindOther {
		t.Fatalf("catalog is not sorted by kind: %+v", entries)
	}
}

func TestBuildAddonCatalogWithoutAccountMarksEverythingAvailable(t *testing.T) {
	entries := BuildAddonCatalog([]CatalogAddOn{{UID: "gWKew2Qp", Name: "Hyperliquid Orderbook Service"}}, nil)
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(entries))
	}
	if entries[0].Status != AddonStatusAvailable {
		t.Fatalf("status = %q, want %q", entries[0].Status, AddonStatusAvailable)
	}
}

func TestFilterAddonCatalog(t *testing.T) {
	entries := []AddonCatalogEntry{
		{AddOnUID: "a", Kind: AddonKindPremium, Endpoint: "api-kusama-sidecar", Chain: "Kusama Sidecar"},
		{AddOnUID: "b", Kind: AddonKindUnlimited, Endpoint: "api-base-mainnet", Name: "Unlimited 200 RPS - api-base-mainnet.n.dwellir.com"},
		{AddOnUID: "c", Kind: AddonKindOther, Name: "Support Retainer"},
	}

	tests := []struct {
		name      string
		premium   bool
		unlimited bool
		chain     string
		wantUIDs  []string
	}{
		{name: "no filters keeps everything", wantUIDs: []string{"a", "b", "c"}},
		{name: "premium only", premium: true, wantUIDs: []string{"a"}},
		{name: "unlimited only", unlimited: true, wantUIDs: []string{"b"}},
		{name: "both kinds", premium: true, unlimited: true, wantUIDs: []string{"a", "b"}},
		{name: "chain by name", chain: "kusama", wantUIDs: []string{"a"}},
		{name: "chain by slug", chain: "api-base-mainnet", wantUIDs: []string{"b"}},
		{name: "chain by url", chain: "https://api-base-mainnet.n.dwellir.com/<key>", wantUIDs: []string{"b"}},
		{name: "chain plus kind", premium: true, chain: "base", wantUIDs: []string{}},
		{name: "no match", chain: "solana", wantUIDs: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterAddonCatalog(entries, tt.premium, tt.unlimited, tt.chain)
			if len(got) != len(tt.wantUIDs) {
				t.Fatalf("got %d entries, want %d: %+v", len(got), len(tt.wantUIDs), got)
			}
			for i, uid := range tt.wantUIDs {
				if got[i].AddOnUID != uid {
					t.Fatalf("entry %d = %q, want %q", i, got[i].AddOnUID, uid)
				}
			}
		})
	}
}

func TestBuildAddonsStatusReportsTrialsAndKeepsInstanceUIDs(t *testing.T) {
	trialEnds := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	account := &AccountInfo{
		PremiumEndpointState: PremiumEndpointState{
			{HostSlug: "api-kusama-sidecar", Status: PremiumStatusTrialActive, TrialEndsAt: trialEnds},
			{HostSlug: "api-polkadot-sidecar", Status: PremiumStatusTrialExpired},
			// An entry whose add-on has since been bought is not a trial row.
			{HostSlug: "api-hyperliquid-mainnet-orderbook", Status: PremiumStatusTrialActive},
			// A locked entry has no trial to report either.
			{HostSlug: "api-kilt-sidecar", Status: PremiumStatusLocked},
		},
		CurrentSubscription: &CurrentSubscriptionWindow{
			SubscriptionAddOns: []OutsetaSubscriptionAddOn{{UID: "inst-1", AddOnUID: "gWKew2Qp"}},
		},
	}

	active := []ActiveAddOn{{InstanceUID: "inst-1", AddOnUID: "gWKew2Qp", Name: "Hyperliquid Orderbook Service"}}
	status := BuildAddonsStatus(active, account, AddonsSourceBilling)

	if status.AddOnsSource != AddonsSourceBilling {
		t.Fatalf("source = %q", status.AddOnsSource)
	}
	if len(status.AddOns) != 1 || status.AddOns[0].InstanceUID != "inst-1" {
		t.Fatalf("active add-ons = %+v", status.AddOns)
	}
	if len(status.Trials) != 2 {
		t.Fatalf("expected 2 trial rows, got %d: %+v", len(status.Trials), status.Trials)
	}
	if status.Trials[0].Endpoint != "api-kusama-sidecar" || status.Trials[0].Status != string(PremiumStatusTrialActive) {
		t.Fatalf("first trial = %+v", status.Trials[0])
	}
	if status.Trials[0].Chain != "Kusama Sidecar" {
		t.Fatalf("expected the chain name from the rule table, got %q", status.Trials[0].Chain)
	}
	if status.Trials[1].Endpoint != "api-polkadot-sidecar" || status.Trials[1].Status != string(PremiumStatusTrialExpired) {
		t.Fatalf("second trial = %+v", status.Trials[1])
	}
}

func TestBuildAddonsStatusNeverReturnsNilSlices(t *testing.T) {
	status := BuildAddonsStatus(nil, nil, AddonsSourceAccount)
	if status.AddOns == nil || status.Trials == nil {
		t.Fatalf("expected empty slices so JSON renders [], got %+v", status)
	}
}

func TestActiveAddOnsFromAccount(t *testing.T) {
	tests := []struct {
		name    string
		account *AccountInfo
		want    []ActiveAddOn
	}{
		{name: "nil account", account: nil},
		{name: "no subscription", account: &AccountInfo{}},
		{
			name: "drops instances without a uid",
			account: &AccountInfo{CurrentSubscription: &CurrentSubscriptionWindow{
				SubscriptionAddOns: []OutsetaSubscriptionAddOn{
					{Name: "No instance uid", AddOnUID: "gWKew2Qp"},
					{UID: "inst-1", Name: "Kept", AddOnUID: "y9gpzRWM", EndDate: "2030-01-01T00:00:00Z"},
				},
			}},
			want: []ActiveAddOn{{
				InstanceUID: "inst-1",
				AddOnUID:    "y9gpzRWM",
				Name:        "Kept",
				EndDate:     "2030-01-01T00:00:00Z",
			}},
		},
		{
			name: "reads the uid off the nested add-on product",
			account: &AccountInfo{CurrentSubscription: &CurrentSubscriptionWindow{
				SubscriptionAddOns: []OutsetaSubscriptionAddOn{
					{UID: "inst-2", Name: "Nested", AddOn: &OutsetaAddOnProduct{UID: "gWKew2Qp"}},
				},
			}},
			want: []ActiveAddOn{{InstanceUID: "inst-2", AddOnUID: "gWKew2Qp", Name: "Nested"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ActiveAddOnsFromAccount(tt.account)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d add-ons, want %d: %+v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("add-on %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestAddonsAPICatalogAndActive(t *testing.T) {
	var requestedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		switch r.URL.Path {
		case "/v4/billing/addons":
			_, _ = w.Write([]byte(`{"addons":[{"uid":"gWKew2Qp","name":"Orderbook","monthlyRate":199.0,"annualRate":null}]}`))
		case "/v4/billing/addons/active":
			_, _ = w.Write([]byte(`{"addOns":[{"instanceUid":"inst-1","addOnUid":"gWKew2Qp","quantity":1,"renewalDate":"2026-09-01T00:00:00Z"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	addons := NewAddonsAPI(NewClient(server.URL, "test-token"))

	catalog, err := addons.Catalog()
	if err != nil {
		t.Fatalf("Catalog() error: %v", err)
	}
	if len(catalog) != 1 || catalog[0].UID != "gWKew2Qp" {
		t.Fatalf("catalog = %+v", catalog)
	}
	if catalog[0].MonthlyRate == nil || *catalog[0].MonthlyRate != 199.0 {
		t.Fatalf("monthly rate = %+v", catalog[0].MonthlyRate)
	}
	if catalog[0].AnnualRate != nil {
		t.Fatalf("expected a null annual rate to stay nil, got %+v", catalog[0].AnnualRate)
	}

	active, err := addons.Active()
	if err != nil {
		t.Fatalf("Active() error: %v", err)
	}
	if len(active) != 1 || active[0].InstanceUID != "inst-1" {
		t.Fatalf("active = %+v", active)
	}

	if len(requestedPaths) != 2 {
		t.Fatalf("expected 2 requests, got %v", requestedPaths)
	}
}

// Marly guards /v4/billing/addons/active with a cookie-only dependency, so a
// CLI token gets 401. The client must surface that as a typed APIError.
func TestAddonsAPIActiveSurfaces401AsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"Not authenticated"}`))
	}))
	defer server.Close()

	_, err := NewAddonsAPI(NewClient(server.URL, "test-token")).Active()
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T (%v)", err, err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", apiErr.StatusCode)
	}
}

func TestAddonsAPIPaymentMethod(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantNil  bool
		wantLast string
		wantExp  string
	}{
		{name: "no card on file", body: `null`, wantNil: true},
		{
			name:     "card with numeric expiry",
			body:     `{"brand":"Visa","last4":"4242","expMonth":4,"expYear":2029,"name":"A Buyer"}`,
			wantLast: "4242",
			wantExp:  "4",
		},
		{
			name:     "card with string expiry",
			body:     `{"brand":"Visa","last4":"4242","expMonth":"04","expYear":"2029"}`,
			wantLast: "4242",
			wantExp:  "04",
		},
		{
			name:     "card with null expiry",
			body:     `{"brand":"Visa","last4":"4242","expMonth":null,"expYear":null}`,
			wantLast: "4242",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			method, err := NewAddonsAPI(NewClient(server.URL, "test-token")).PaymentMethod()
			if err != nil {
				t.Fatalf("PaymentMethod() error: %v", err)
			}
			if tt.wantNil {
				if method != nil {
					t.Fatalf("expected nil for a null body, got %+v", method)
				}
				return
			}
			if method == nil {
				t.Fatal("expected a payment method")
			}
			if method.Last4 != tt.wantLast {
				t.Fatalf("last4 = %q, want %q", method.Last4, tt.wantLast)
			}
			if string(method.ExpMonth) != tt.wantExp {
				t.Fatalf("expMonth = %q, want %q", method.ExpMonth, tt.wantExp)
			}
		})
	}
}

func TestPaymentMethodResultMarshalsWithoutACard(t *testing.T) {
	encoded, err := json.Marshal(PaymentMethodResult{HasPaymentMethod: false})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if string(encoded) != `{"hasPaymentMethod":false}` {
		t.Fatalf("encoded = %s", encoded)
	}
}
