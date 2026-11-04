//go:build e2e

package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const e2eWhitelist = `{"id":7,"name":"office","description":null,"enabled":true,"apply_to_all_api_keys":false,` +
	`"entries":[{"id":1,"cidr":"203.0.113.10/32","description":null,"enabled":true}],` +
	`"assigned_api_keys":[{"api_key_id":1,"api_key":"` + e2eKey + `","assignment_kind":"manual","enabled":true}]}`

// whitelistsServer serves one key and one whitelist, and records every
// request that is not a GET.
func whitelistsServer(t *testing.T) (map[string]string, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var writes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			writes = append(writes, strings.TrimSpace(r.Method+" "+r.URL.Path+" "+string(body)))
			mu.Unlock()
		}
		switch {
		case r.URL.Path == "/v4/organization/apikeys":
			_, _ = w.Write([]byte(`[{"api_key":"` + e2eKey + `","name":"ci","enabled":true}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/v4/organization/ip-whitelists":
			_, _ = w.Write([]byte("[" + e2eWhitelist + "]"))
		case r.Method == http.MethodPost && r.URL.Path == "/v4/organization/ip-whitelists":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(e2eWhitelist))
		case r.Method == http.MethodDelete && r.URL.Path == "/v4/organization/ip-whitelists/7":
			w.WriteHeader(http.StatusNoContent)
		case strings.HasPrefix(r.URL.Path, "/v4/organization/ip-whitelists/7"):
			_, _ = w.Write([]byte(e2eWhitelist))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	env := map[string]string{"DWELLIR_TOKEN": "test-token", "DWELLIR_API_URL": server.URL}
	return env, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), writes...)
	}
}

func TestWhitelistsListNeverPrintsTheKey(t *testing.T) {
	env, _ := whitelistsServer(t)
	for _, args := range [][]string{
		{"whitelists", "list", "--json"},
		{"whitelists", "list", "--human"},
		{"whitelist", "list", "--toon"},
	} {
		res := runCLIWithEnv(t, env, args...)
		if res.exitCode != 0 {
			t.Fatalf("%v: expected success, got %d\nstdout: %s\nstderr: %s", args, res.exitCode, res.stdout, res.stderr)
		}
		output := res.stdout + res.stderr
		if strings.Contains(output, e2eKey) || !strings.Contains(output, "9f1c****6789") || !strings.Contains(output, "203.0.113.10/32") {
			t.Fatalf("%v: expected the whitelist with a masked key, got:\n%s", args, output)
		}
	}
}

func TestWhitelistsCreateSendsSingleAddresses(t *testing.T) {
	env, writes := whitelistsServer(t)
	res := runCLIWithEnv(t, env, "whitelists", "create", "--name", "office", "--ip", "203.0.113.10", "--all-keys", "--json")
	if res.exitCode != 0 || !strings.Contains(res.stdout, `"ok":true`) {
		t.Fatalf("expected success, got %d\nstdout: %s\nstderr: %s", res.exitCode, res.stdout, res.stderr)
	}
	want := `POST /v4/organization/ip-whitelists {"name":"office","entries":["203.0.113.10/32"],"apply_to_all_api_keys":true,"enabled":true}`
	if got := writes(); len(got) != 1 || got[0] != want {
		t.Fatalf("unexpected writes: %q", got)
	}
}

func TestWhitelistsCreateRejectsRanges(t *testing.T) {
	env, writes := whitelistsServer(t)
	res := runCLIWithEnv(t, env, "whitelists", "create", "--name", "office", "--ip", "203.0.113.0/24", "--json")
	if res.exitCode != 4 || !strings.Contains(res.stdout, `"code":"validation_error"`) || !strings.Contains(res.stdout, "single addresses") {
		t.Fatalf("expected a validation error, got %d\n%s", res.exitCode, res.stdout)
	}
	if got := writes(); len(got) != 0 {
		t.Fatalf("expected no writes, got %q", got)
	}
}

func TestWhitelistsDeleteNeedsYesWithoutATerminal(t *testing.T) {
	env, writes := whitelistsServer(t)
	for _, format := range []string{"--json", "--toon", "--human"} {
		res := runCLIWithEnv(t, env, "whitelists", "delete", "office", format)
		output := res.stdout + res.stderr
		if res.exitCode != 4 || !strings.Contains(output, "needs confirmation") || !strings.Contains(output, "--yes") {
			t.Fatalf("%s: expected a confirmation error naming --yes, got %d\n%s", format, res.exitCode, output)
		}
	}
	if got := writes(); len(got) != 0 {
		t.Fatalf("expected no writes, got %q", got)
	}

	res := runCLIWithEnv(t, env, "whitelists", "delete", "office", "--yes", "--json")
	if res.exitCode != 0 || !strings.Contains(res.stdout, `"status":"deleted"`) {
		t.Fatalf("expected a deleted status, got %d\n%s", res.exitCode, res.stdout)
	}
	if got := writes(); len(got) != 1 || got[0] != "DELETE /v4/organization/ip-whitelists/7" {
		t.Fatalf("unexpected writes: %q", got)
	}
}

func TestWhitelistsArgumentErrorsExitWithValidationCode(t *testing.T) {
	env, _ := whitelistsServer(t)
	for _, args := range [][]string{
		{"whitelists", "enable", "--json"},
		{"whitelists", "assign", "office", "--json"},
		{"whitelists", "update", "office", "--json"},
	} {
		res := runCLIWithEnv(t, env, args...)
		if res.exitCode != 4 || !strings.Contains(res.stdout, `"code":"validation_error"`) {
			t.Fatalf("%v: expected exit 4 with validation_error, got %d\n%s", args, res.exitCode, res.stdout)
		}
	}
}

func TestWhitelistsUnknownWhitelistExitsNotFound(t *testing.T) {
	env, _ := whitelistsServer(t)
	res := runCLIWithEnv(t, env, "whitelists", "disable", "home", "--json")
	if res.exitCode != 3 || !strings.Contains(res.stdout, `"code":"not_found"`) {
		t.Fatalf("expected exit 3 with not_found, got %d\n%s", res.exitCode, res.stdout)
	}
}
