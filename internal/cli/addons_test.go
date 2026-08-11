package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dwellir-public/cli/internal/api"
)

func TestAddonsEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		set   bool
		want  bool
	}{
		{name: "unset defaults to off", want: false},
		{name: "empty defaults to off", set: true, value: "", want: false},
		{name: "one enables", set: true, value: "1", want: true},
		{name: "true enables", set: true, value: "true", want: true},
		{name: "yes enables", set: true, value: "yes", want: true},
		{name: "on enables", set: true, value: "on", want: true},
		{name: "mixed case enables", set: true, value: "TRUE", want: true},
		{name: "zero disables", set: true, value: "0", want: false},
		{name: "false disables", set: true, value: "false", want: false},
		{name: "garbage disables", set: true, value: "maybe", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv(addonsEnvVar, tt.value)
			} else {
				t.Setenv(addonsEnvVar, "")
			}
			if got := addonsEnabled(); got != tt.want {
				t.Fatalf("addonsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// The whole point of the gate is that a released binary carries no add-on
// commands. Registration happens in init(), so this asserts the state the test
// binary was built into rather than re-running init().
func TestAddonsCommandRegistrationFollowsTheGate(t *testing.T) {
	registered := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "addons" {
			registered = true
		}
	}
	if registered != addonsEnabled() {
		t.Fatalf("addons command registered = %v, but the gate is %v", registered, addonsEnabled())
	}
}

func TestAddonsSubcommandsAreWiredUp(t *testing.T) {
	want := map[string]bool{"list": false, "status": false}
	for _, sub := range addonsCmd.Commands() {
		if _, expected := want[sub.Name()]; expected {
			want[sub.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("addons subcommand %q is not registered", name)
		}
	}
}

func TestAddonsListFlags(t *testing.T) {
	for _, name := range []string{"premium", "unlimited", "chain"} {
		if addonsListCmd.Flags().Lookup(name) == nil {
			t.Fatalf("addons list is missing the --%s flag", name)
		}
	}
}

func TestResolveActiveAddOnsPrefersTheBillingRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"addOns":[{"instanceUid":"inst-1","renewalDate":"2026-09-01T00:00:00Z"}]}`))
	}))
	defer server.Close()

	addons := api.NewAddonsAPI(api.NewClient(server.URL, "test-token"))
	active, source, notices, err := resolveActiveAddOns(addons, &api.AccountInfo{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if source != api.AddonsSourceBilling {
		t.Fatalf("source = %q, want %q", source, api.AddonsSourceBilling)
	}
	if len(notices) != 0 {
		t.Fatalf("expected no notices, got %+v", notices)
	}
	if len(active) != 1 || active[0].RenewalDate == "" {
		t.Fatalf("expected the billing payload with its renewal date, got %+v", active)
	}
}

// Marly still guards /v4/billing/addons/active with a cookie-only dependency.
// The CLI must fall back to the account payload and say so, not dump a raw 401.
func TestResolveActiveAddOnsFallsBackWhenMarlyRejectsTheCLIToken(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"detail":"Not authenticated"}`))
			}))
			defer server.Close()

			account := &api.AccountInfo{CurrentSubscription: &api.CurrentSubscriptionWindow{
				SubscriptionAddOns: []api.OutsetaSubscriptionAddOn{
					{UID: "inst-1", Name: "Orderbook", AddOnUID: "gWKew2Qp"},
				},
			}}

			addons := api.NewAddonsAPI(api.NewClient(server.URL, "test-token"))
			active, source, notices, err := resolveActiveAddOns(addons, account)
			if err != nil {
				t.Fatalf("expected a graceful fallback, got error: %v", err)
			}
			if source != api.AddonsSourceAccount {
				t.Fatalf("source = %q, want %q", source, api.AddonsSourceAccount)
			}
			if len(active) != 1 || active[0].InstanceUID != "inst-1" {
				t.Fatalf("expected the account fallback to keep the instance uid, got %+v", active)
			}
			if len(notices) != 1 || notices[0].Code != "addons_active_unavailable" {
				t.Fatalf("expected a typed notice, got %+v", notices)
			}
			if notices[0].Help == "" {
				t.Fatal("expected the notice to explain the missing renewal dates")
			}
		})
	}
}

// Any other failure is a real failure and must not be papered over with an
// incomplete fallback list.
func TestResolveActiveAddOnsPropagatesOtherFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	addons := api.NewAddonsAPI(api.NewClient(server.URL, "test-token"))
	_, _, _, err := resolveActiveAddOns(addons, &api.AccountInfo{})
	if err == nil {
		t.Fatal("expected a 502 to propagate")
	}
}

func TestAccountPaymentMethodIsGated(t *testing.T) {
	registered := false
	for _, sub := range accountCmd.Commands() {
		if sub.Name() == "payment-method" {
			registered = true
		}
	}
	if registered != addonsEnabled() {
		t.Fatalf("payment-method registered = %v, but the gate is %v", registered, addonsEnabled())
	}
}
