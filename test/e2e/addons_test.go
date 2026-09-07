//go:build e2e

package e2e

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	addonsCatalogBody = `{"addons":[
  {"uid":"gWKew2Qp","name":"Hyperliquid Orderbook Service","monthlyRate":199.0,"annualRate":2149.0},
  {"uid":"amRyMMWJ","name":"Hyperliquid Orderbook Testnet","monthlyRate":199.0},
  {"uid":"y9gpzRWM","name":"Kusama Sidecar","monthlyRate":100.0},
  {"uid":"auto-1","name":"Unlimited 200 RPS - api-base-mainnet.n.dwellir.com","monthlyRate":450.0},
  {"uid":"misc-1","name":"Support Retainer","monthlyRate":1000.0}
]}`

	// premiumEndpointState is an encoded JSON *string*, not an array. Marly
	// declares it as Optional[str] (pymarly/outseta/models.py:502) and writes it
	// back through encode_premium_state, so the stub must match that shape or it
	// tests a payload marly never sends.
	addonsAccountBody = `{
  "uid":"acct-123",
  "name":"Acme",
  "premiumEndpointState":"[{\"hostSlug\":\"api-hyperliquid-testnet-orderbook\",\"status\":\"trial-active\",\"trialEndsAt\":\"2099-01-01T00:00:00Z\"},{\"hostSlug\":\"api-kusama-sidecar\",\"status\":\"trial-expired\"}]",
  "currentSubscription":{
    "subscriptionAddOns":[
      {
        "uid":"L9P6xpnm",
        "name":"Hyperliquid Orderbook Service",
        "addOnUid":"gWKew2Qp",
        "quantity":2,
        "startDate":"2026-08-01T00:00:00Z",
        "renewalDate":"2026-09-01T00:00:00Z"
      }
    ]
  }
}`
)

// addonsStubServer serves the four routes phase 1 reads. activeStatus of 0
// serves the billing add-on payload; any other value is returned instead, which
// is how the marly-rejects-CLI-tokens case is simulated.
func addonsStubServer(t *testing.T, activeStatus int, paymentMethodBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v4/billing/addons":
			_, _ = w.Write([]byte(addonsCatalogBody))
		case "/v4/organization/information/outseta":
			_, _ = w.Write([]byte(addonsAccountBody))
		case "/v4/billing/addons/active":
			if activeStatus != 0 {
				w.WriteHeader(activeStatus)
				_, _ = w.Write([]byte(`{"detail":"Not authenticated"}`))
				return
			}
			_, _ = w.Write([]byte(`{"addOns":[
  {"instanceUid":"L9P6xpnm","addOnUid":"gWKew2Qp","name":"Hyperliquid Orderbook Service","quantity":1,"renewalDate":"2026-09-01T00:00:00Z"}
]}`))
		case "/v4/billing/payment-method":
			_, _ = w.Write([]byte(paymentMethodBody))
		default:
			http.NotFound(w, r)
		}
	}))
}

func addonsEnv(serverURL string) map[string]string {
	return map[string]string{
		"DWELLIR_TOKEN":   "test-token",
		"DWELLIR_API_URL": serverURL,
		"DWELLIR_ADDONS":  "1",
	}
}

// The gate exists so a released binary carries no add-on surface at all.
func TestAddonsIsUnknownCommandWhenTheGateIsOff(t *testing.T) {
	for _, value := range []string{"", "0", "false"} {
		t.Run("DWELLIR_ADDONS="+value, func(t *testing.T) {
			res := runCLIWithEnv(t, map[string]string{"DWELLIR_ADDONS": value}, "addons", "list", "--json")
			if res.exitCode == 0 {
				t.Fatalf("expected a non-zero exit, got 0\nstdout: %s", res.stdout)
			}
			combined := res.stdout + res.stderr
			if !strings.Contains(combined, "unknown command") {
				t.Fatalf("expected an unknown command error, got:\n%s", combined)
			}
		})
	}
}

func TestAddonsIsAbsentFromHelpWhenTheGateIsOff(t *testing.T) {
	res := runCLIWithEnv(t, map[string]string{"DWELLIR_ADDONS": "0"}, "--help")
	if res.exitCode != 0 {
		t.Fatalf("expected success, got %d\nstderr: %s", res.exitCode, res.stderr)
	}
	if strings.Contains(res.stdout, "addons") {
		t.Fatalf("--help must not mention addons with the gate off:\n%s", res.stdout)
	}
}

func TestAddonsAppearsInHelpWhenTheGateIsOn(t *testing.T) {
	res := runCLIWithEnv(t, map[string]string{"DWELLIR_ADDONS": "1"}, "--help")
	if res.exitCode != 0 {
		t.Fatalf("expected success, got %d\nstderr: %s", res.exitCode, res.stderr)
	}
	if !strings.Contains(res.stdout, "addons") {
		t.Fatalf("expected addons in --help with the gate on:\n%s", res.stdout)
	}
}

func TestAddonsListClassifiesAndMergesOwnership(t *testing.T) {
	server := addonsStubServer(t, 0, `null`)
	defer server.Close()

	res := runCLIWithEnv(t, addonsEnv(server.URL), "addons", "list", "--json")
	if res.exitCode != 0 {
		t.Fatalf("expected success, got %d\nstderr: %s\nstdout: %s", res.exitCode, res.stderr, res.stdout)
	}

	parsed := parseJSON(t, res.stdout)
	entries, _ := parsed["data"].([]interface{})
	if len(entries) != 5 {
		t.Fatalf("expected 5 catalog entries, got %d: %#v", len(entries), parsed["data"])
	}

	byUID := map[string]map[string]interface{}{}
	for _, item := range entries {
		entry, _ := item.(map[string]interface{})
		uid, _ := entry["addOnUid"].(string)
		byUID[uid] = entry
	}

	owned := byUID["gWKew2Qp"]
	if owned["kind"] != "premium" || owned["status"] != "active" {
		t.Fatalf("owned premium entry = %#v", owned)
	}
	if owned["endpoint"] != "api-hyperliquid-mainnet-orderbook" {
		t.Fatalf("expected the endpoint slug from the rule table, got %#v", owned["endpoint"])
	}

	if byUID["amRyMMWJ"]["status"] != "trial" {
		t.Fatalf("trialing entry = %#v", byUID["amRyMMWJ"])
	}
	if byUID["y9gpzRWM"]["status"] != "trial-expired" {
		t.Fatalf("expired trial entry = %#v", byUID["y9gpzRWM"])
	}

	unlimited := byUID["auto-1"]
	if unlimited["kind"] != "unlimited" || unlimited["rps"] != float64(200) {
		t.Fatalf("unlimited entry = %#v", unlimited)
	}
	if byUID["misc-1"]["kind"] != "other" {
		t.Fatalf("uncategorized entry = %#v", byUID["misc-1"])
	}
}

func TestAddonsListFilters(t *testing.T) {
	server := addonsStubServer(t, 0, `null`)
	defer server.Close()

	tests := []struct {
		name     string
		args     []string
		wantUIDs []string
	}{
		{name: "premium", args: []string{"--premium"}, wantUIDs: []string{"gWKew2Qp", "amRyMMWJ", "y9gpzRWM"}},
		{name: "unlimited", args: []string{"--unlimited"}, wantUIDs: []string{"auto-1"}},
		{name: "both", args: []string{"--premium", "--unlimited"}, wantUIDs: []string{"gWKew2Qp", "amRyMMWJ", "y9gpzRWM", "auto-1"}},
		{name: "chain name", args: []string{"--chain", "kusama"}, wantUIDs: []string{"y9gpzRWM"}},
		{name: "chain url", args: []string{"--chain", "https://api-base-mainnet.n.dwellir.com/<key>"}, wantUIDs: []string{"auto-1"}},
		{name: "no match", args: []string{"--chain", "solana"}, wantUIDs: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"addons", "list"}, tt.args...)
			res := runCLIWithEnv(t, addonsEnv(server.URL), append(args, "--json")...)
			if res.exitCode != 0 {
				t.Fatalf("expected success, got %d\nstderr: %s", res.exitCode, res.stderr)
			}

			parsed := parseJSON(t, res.stdout)
			entries, _ := parsed["data"].([]interface{})
			got := make([]string, 0, len(entries))
			for _, item := range entries {
				entry, _ := item.(map[string]interface{})
				uid, _ := entry["addOnUid"].(string)
				got = append(got, uid)
			}

			if len(got) != len(tt.wantUIDs) {
				t.Fatalf("got %v, want %v", got, tt.wantUIDs)
			}
			wanted := map[string]bool{}
			for _, uid := range tt.wantUIDs {
				wanted[uid] = true
			}
			for _, uid := range got {
				if !wanted[uid] {
					t.Fatalf("unexpected uid %q in %v, want %v", uid, got, tt.wantUIDs)
				}
			}
		})
	}
}

func TestAddonsStatusUsesTheBillingRouteWhenItAcceptsTheToken(t *testing.T) {
	server := addonsStubServer(t, 0, `null`)
	defer server.Close()

	res := runCLIWithEnv(t, addonsEnv(server.URL), "addons", "status", "--json")
	if res.exitCode != 0 {
		t.Fatalf("expected success, got %d\nstderr: %s\nstdout: %s", res.exitCode, res.stderr, res.stdout)
	}

	data, _ := parseJSON(t, res.stdout)["data"].(map[string]interface{})
	if data["addOnsSource"] != "billing" {
		t.Fatalf("addOnsSource = %#v, want billing", data["addOnsSource"])
	}
	if _, present := data["notices"]; present {
		t.Fatalf("expected no notices on the happy path: %#v", data["notices"])
	}

	addOns, _ := data["addOns"].([]interface{})
	if len(addOns) != 1 {
		t.Fatalf("expected 1 active add-on, got %#v", data["addOns"])
	}
	first, _ := addOns[0].(map[string]interface{})
	if first["instanceUid"] != "L9P6xpnm" {
		t.Fatalf("expected the instance uid, got %#v", first)
	}
	if first["renewalDate"] != "2026-09-01T00:00:00Z" {
		t.Fatalf("expected the billing route's renewal date, got %#v", first)
	}

	trials, _ := data["trials"].([]interface{})
	if len(trials) != 2 {
		t.Fatalf("expected 2 trials, got %#v", data["trials"])
	}
}

// Marly still guards /v4/billing/addons/active with a cookie-only dependency.
// The CLI must fall back to the account payload, keep the instance UIDs, and
// name the problem instead of dumping a raw HTTP 401.
func TestAddonsStatusFallsBackWhenTheBillingRouteRejectsTheCLIToken(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := addonsStubServer(t, status, `null`)
			defer server.Close()

			res := runCLIWithEnv(t, addonsEnv(server.URL), "addons", "status", "--json")
			if res.exitCode != 0 {
				t.Fatalf("expected a graceful fallback, got exit %d\nstderr: %s\nstdout: %s", res.exitCode, res.stderr, res.stdout)
			}

			parsed := parseJSON(t, res.stdout)
			if parsed["ok"] != true {
				t.Fatalf("expected ok:true, got %#v", parsed)
			}

			data, _ := parsed["data"].(map[string]interface{})
			if data["addOnsSource"] != "account" {
				t.Fatalf("addOnsSource = %#v, want account", data["addOnsSource"])
			}

			notices, _ := data["notices"].([]interface{})
			if len(notices) != 1 {
				t.Fatalf("expected exactly one notice, got %#v", data["notices"])
			}
			notice, _ := notices[0].(map[string]interface{})
			if notice["code"] != "addons_active_unavailable" {
				t.Fatalf("notice code = %#v", notice)
			}
			if help, _ := notice["help"].(string); help == "" {
				t.Fatalf("expected help text on the notice, got %#v", notice)
			}
			if message, _ := notice["message"].(string); strings.Contains(message, "HTTP 401") {
				t.Fatalf("the notice must explain the cause, not echo the status line: %q", message)
			}

			addOns, _ := data["addOns"].([]interface{})
			if len(addOns) != 1 {
				t.Fatalf("expected the account fallback to list 1 add-on, got %#v", data["addOns"])
			}
			first, _ := addOns[0].(map[string]interface{})
			if first["instanceUid"] != "L9P6xpnm" {
				t.Fatalf("the fallback must keep the instance uid, got %#v", first)
			}
			// Marly builds both responses from one model, so the fallback must
			// not silently drop the fields the billing route would have shown.
			if first["quantity"] != float64(2) {
				t.Fatalf("the fallback dropped quantity: %#v", first)
			}
			if first["startDate"] != "2026-08-01T00:00:00Z" {
				t.Fatalf("the fallback dropped startDate: %#v", first)
			}
			if first["renewalDate"] != "2026-09-01T00:00:00Z" {
				t.Fatalf("the fallback dropped renewalDate: %#v", first)
			}

			trials, _ := data["trials"].([]interface{})
			if len(trials) != 2 {
				t.Fatalf("expected the encoded premiumEndpointState string to decode into 2 trials, got %#v", data["trials"])
			}
		})
	}
}

func TestAddonsStatusHumanOutputShowsTheInstanceUIDAndNotice(t *testing.T) {
	server := addonsStubServer(t, http.StatusUnauthorized, `null`)
	defer server.Close()

	res := runCLIWithEnv(t, addonsEnv(server.URL), "addons", "status", "--human")
	if res.exitCode != 0 {
		t.Fatalf("expected success, got %d\nstderr: %s\nstdout: %s", res.exitCode, res.stderr, res.stdout)
	}
	for _, want := range []string{
		"Active add-ons",
		"L9P6xpnm",
		// The fallback carries quantity and renewal date, so the row must not
		// degrade to "Qty -" and a bare "renews".
		"renews 2026-09-01T00:00:00Z",
		"Trials",
		"addons_active_unavailable",
	} {
		if !strings.Contains(res.stdout, want) {
			t.Fatalf("expected %q in human output:\n%s", want, res.stdout)
		}
	}
}

// Any other failure on the billing route is a real failure.
func TestAddonsStatusFailsOnANonAuthBillingError(t *testing.T) {
	server := addonsStubServer(t, http.StatusBadGateway, `null`)
	defer server.Close()

	res := runCLIWithEnv(t, addonsEnv(server.URL), "addons", "status", "--json")
	if res.exitCode == 0 {
		t.Fatalf("expected a non-zero exit on HTTP 502\nstdout: %s", res.stdout)
	}
	parsed := parseJSON(t, res.stdout)
	if parsed["ok"] != false {
		t.Fatalf("expected ok:false, got %#v", parsed)
	}
}

func TestAccountPaymentMethod(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantHas bool
		wantYes []string
	}{
		{name: "no card on file", body: `null`, wantHas: false},
		{
			name:    "card on file",
			body:    `{"brand":"Visa","last4":"4242","expMonth":4,"expYear":2029,"name":"A Buyer"}`,
			wantHas: true,
			// Masked, because the shared cell formatter would print a bare
			// "4242" as "4,242".
			wantYes: []string{"Visa", "•••• 4242", "4/2029", "A Buyer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := addonsStubServer(t, 0, tt.body)
			defer server.Close()

			res := runCLIWithEnv(t, addonsEnv(server.URL), "account", "payment-method", "--json")
			if res.exitCode != 0 {
				t.Fatalf("expected success, got %d\nstderr: %s\nstdout: %s", res.exitCode, res.stderr, res.stdout)
			}

			data, _ := parseJSON(t, res.stdout)["data"].(map[string]interface{})
			if data["hasPaymentMethod"] != tt.wantHas {
				t.Fatalf("hasPaymentMethod = %#v, want %v", data["hasPaymentMethod"], tt.wantHas)
			}

			human := runCLIWithEnv(t, addonsEnv(server.URL), "account", "payment-method", "--human")
			if human.exitCode != 0 {
				t.Fatalf("human run failed: %s", human.stderr)
			}
			if !tt.wantHas {
				if !strings.Contains(human.stdout, "No payment method on file.") {
					t.Fatalf("unexpected human output:\n%s", human.stdout)
				}
				return
			}
			for _, want := range tt.wantYes {
				if !strings.Contains(human.stdout, want) {
					t.Fatalf("expected %q in human output:\n%s", want, human.stdout)
				}
			}
		})
	}
}

// Cobra answers an unrecognized subcommand of a group with the group's help and
// exit 0, so the check that matters is whether the command is registered.
func TestAccountPaymentMethodTOONOutput(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantAny  []string
		wantNone []string
	}{
		{
			name:     "no card on file",
			body:     `null`,
			wantAny:  []string{"hasPaymentMethod: false", "command: account.payment-method", "ok: true"},
			wantNone: []string{"paymentMethod:"},
		},
		{
			name:    "card on file",
			body:    `{"brand":"Visa","last4":"4242","expMonth":4,"expYear":2029,"name":"A Buyer"}`,
			wantAny: []string{"hasPaymentMethod: true", "paymentMethod:", "Visa", "4242", "A Buyer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := addonsStubServer(t, 0, tt.body)
			defer server.Close()

			res := runCLIWithEnv(t, addonsEnv(server.URL), "account", "payment-method", "--toon")
			if res.exitCode != 0 {
				t.Fatalf("expected success, got %d\nstderr: %s\nstdout: %s", res.exitCode, res.stderr, res.stdout)
			}
			for _, want := range tt.wantAny {
				if !strings.Contains(res.stdout, want) {
					t.Fatalf("expected %q in TOON output:\n%s", want, res.stdout)
				}
			}
			for _, unwanted := range tt.wantNone {
				if strings.Contains(res.stdout, unwanted) {
					t.Fatalf("did not expect %q in TOON output:\n%s", unwanted, res.stdout)
				}
			}
		})
	}
}

// An empty 200 must not be reported as "no card on file".
func TestAccountPaymentMethodFailsOnAnEmptyBody(t *testing.T) {
	server := addonsStubServer(t, 0, ``)
	defer server.Close()

	res := runCLIWithEnv(t, addonsEnv(server.URL), "account", "payment-method", "--json")
	if res.exitCode == 0 {
		t.Fatalf("expected a non-zero exit for an empty body\nstdout: %s", res.stdout)
	}
	parsed := parseJSON(t, res.stdout)
	if parsed["ok"] != false {
		t.Fatalf("expected ok:false, got %#v", parsed)
	}
}

func TestAccountPaymentMethodIsAbsentWhenTheGateIsOff(t *testing.T) {
	off := runCLIWithEnv(t, map[string]string{"DWELLIR_ADDONS": "0"}, "account", "--help")
	if off.exitCode != 0 {
		t.Fatalf("expected success, got %d\nstderr: %s", off.exitCode, off.stderr)
	}
	if strings.Contains(off.stdout, "payment-method") {
		t.Fatalf("account --help must not list payment-method with the gate off:\n%s", off.stdout)
	}

	on := runCLIWithEnv(t, map[string]string{"DWELLIR_ADDONS": "1"}, "account", "--help")
	if on.exitCode != 0 {
		t.Fatalf("expected success, got %d\nstderr: %s", on.exitCode, on.stderr)
	}
	if !strings.Contains(on.stdout, "payment-method") {
		t.Fatalf("expected payment-method in account --help with the gate on:\n%s", on.stdout)
	}
}

func TestDoctorReportsTheAddonsGate(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		{name: "gate off", env: "0", want: false},
		{name: "gate on", env: "1", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := runCLIWithConfigDirAndEnv(t, t.TempDir(), map[string]string{"DWELLIR_ADDONS": tt.env}, "doctor", "--json")
			if res.exitCode != 0 {
				t.Fatalf("doctor failed: %s", res.stderr)
			}
			check := findDoctorCheck(t, parseJSON(t, res.stdout), "feature_gates")
			details, ok := check["details"].(map[string]interface{})
			if !ok {
				t.Fatalf("feature_gates details missing: %#v", check)
			}
			if details["addons_enabled"] != tt.want {
				t.Fatalf("addons_enabled = %#v, want %v", details["addons_enabled"], tt.want)
			}
			if details["addons_env"] != tt.env {
				t.Fatalf("addons_env = %#v, want %q", details["addons_env"], tt.env)
			}
		})
	}
}
