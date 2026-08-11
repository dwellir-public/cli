package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dwellir-public/cli/internal/api"
)

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

func TestHumanAddonsList(t *testing.T) {
	var buf bytes.Buffer
	err := NewHumanFormatter(&buf).Success("addons.list", []api.AddonCatalogEntry{
		{
			AddOnUID:    "gWKew2Qp",
			Name:        "Hyperliquid Orderbook Service",
			Kind:        api.AddonKindPremium,
			Endpoint:    "api-hyperliquid-mainnet-orderbook",
			Chain:       "Hyperliquid Orderbook",
			MonthlyRate: floatPtr(199),
			Status:      api.AddonStatusTrial,
		},
		{
			AddOnUID: "auto-1",
			Name:     "Unlimited 200 RPS - api-base-mainnet.n.dwellir.com",
			Kind:     api.AddonKindUnlimited,
			Endpoint: "api-base-mainnet",
			RPS:      intPtr(200),
			Status:   api.AddonStatusAvailable,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"ADD-ON", "CHAIN", "KIND", "TIER", "PRICE/MO", "STATUS",
		"api-hyperliquid-mainnet-orderbook", "Hyperliquid Orderbook",
		"premium", "199.00", "trial",
		"api-base-mainnet", "unlimited", "200 RPS", "available",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output:\n%s", want, got)
		}
	}
}

// A nil monthly rate means Outseta does not sell the add-on monthly. Rendering
// it as 0.00 would read as free.
func TestHumanAddonsListRendersAMissingRateAsADash(t *testing.T) {
	var buf bytes.Buffer
	err := NewHumanFormatter(&buf).Success("addons.list", []api.AddonCatalogEntry{
		{AddOnUID: "x", Kind: api.AddonKindOther, Name: "One-time setup", Status: api.AddonStatusAvailable},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if strings.Contains(got, "0.00") {
		t.Fatalf("a nil monthly rate must not render as 0.00:\n%s", got)
	}
	// A catalog entry with no endpoint must still be identifiable.
	if !strings.Contains(got, "One-time setup") {
		t.Fatalf("expected the product name to label an endpoint-less row:\n%s", got)
	}
}

func TestHumanAddonsListEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := NewHumanFormatter(&buf).Success("addons.list", []api.AddonCatalogEntry{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No add-ons match those filters.") {
		t.Fatalf("unexpected output:\n%s", buf.String())
	}
}

func TestHumanAddonsStatusAlwaysShowsTheInstanceUID(t *testing.T) {
	var buf bytes.Buffer
	err := NewHumanFormatter(&buf).Success("addons.status", api.AddonsStatus{
		AddOnsSource: api.AddonsSourceBilling,
		AddOns: []api.ActiveAddOn{
			{
				InstanceUID: "L9P6xpnm",
				Name:        "Hyperliquid Orderbook Service",
				Quantity:    intPtr(1),
				RenewalDate: "2026-09-01T00:00:00Z",
			},
			{InstanceUID: "M2Q7ypqr", Name: "Kusama Sidecar", EndDate: "2026-08-31T00:00:00Z"},
		},
		Trials: []api.AddonTrial{
			{Endpoint: "api-kilt-sidecar", Chain: "KILT Sidecar", Status: "trial-active", EndsAt: "2026-08-14T00:00:00Z"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"Active add-ons", "INSTANCE UID", "L9P6xpnm", "M2Q7ypqr",
		"renews 2026-09-01T00:00:00Z", "cancels 2026-08-31T00:00:00Z",
		"Trials", "api-kilt-sidecar", "trial-active", "2026-08-14T00:00:00Z",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output:\n%s", want, got)
		}
	}
}

func TestHumanAddonsStatusEmptyAndNoticed(t *testing.T) {
	var buf bytes.Buffer
	err := NewHumanFormatter(&buf).Success("addons.status", api.AddonsStatus{
		AddOns:       []api.ActiveAddOn{},
		AddOnsSource: api.AddonsSourceAccount,
		Trials:       []api.AddonTrial{},
		Notices: []api.AddonNotice{{
			Code:    "addons_active_unavailable",
			Message: "The billing add-on route rejected this CLI token.",
			Help:    "Renewal dates are missing from the fallback.",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if strings.Count(got, "None.") != 2 {
		t.Fatalf("expected both sections to report None:\n%s", got)
	}
	for _, want := range []string{
		"addons_active_unavailable",
		"The billing add-on route rejected this CLI token.",
		"Renewal dates are missing from the fallback.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output:\n%s", want, got)
		}
	}
}

func TestHumanPaymentMethod(t *testing.T) {
	tests := []struct {
		name     string
		result   api.PaymentMethodResult
		want     []string
		wantNone bool
	}{
		{
			name:     "no card on file",
			result:   api.PaymentMethodResult{HasPaymentMethod: false},
			wantNone: true,
		},
		{
			name: "card on file",
			result: api.PaymentMethodResult{
				HasPaymentMethod: true,
				PaymentMethod: &api.PaymentMethod{
					Brand:    "Visa",
					Last4:    "4242",
					ExpMonth: "04",
					ExpYear:  "2029",
					Name:     "A Buyer",
				},
			},
			want: []string{"Visa", "•••• 4242", "04/2029", "A Buyer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := NewHumanFormatter(&buf).Success("account.payment-method", tt.result); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := buf.String()
			if tt.wantNone {
				if !strings.Contains(got, "No payment method on file.") {
					t.Fatalf("unexpected output:\n%s", got)
				}
				return
			}
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("expected %q in output:\n%s", want, got)
				}
			}
		})
	}
}

func TestFormatCardExpiry(t *testing.T) {
	tests := []struct {
		name  string
		month string
		year  string
		want  string
	}{
		{name: "both", month: "04", year: "2029", want: "04/2029"},
		{name: "year only", year: "2029", want: "?/2029"},
		{name: "month only", month: "04", want: "04/?"},
		{name: "neither", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatCardExpiry(tt.month, tt.year); got != tt.want {
				t.Fatalf("formatCardExpiry(%q, %q) = %q, want %q", tt.month, tt.year, got, tt.want)
			}
		})
	}
}
