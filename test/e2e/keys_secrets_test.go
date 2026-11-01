//go:build e2e

package e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	e2eKey        = "9f1c2d3e-4b5a-6789-abcd-ef0123456789"
	e2eRotatedKey = "1a2b3c4d-5e6f-7081-92a3-b4c5d6e7f809"
)

func keysServer(t *testing.T) map[string]string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v4/organization/apikeys":
			_, _ = w.Write([]byte(`[{"api_key":"` + e2eKey + `","name":"ci","enabled":true}]`))
		case "/v4/organization/apikeys/" + e2eKey:
			_, _ = w.Write([]byte(`{"api_key":"` + e2eKey + `","name":"ci","enabled":false}`))
		case "/v4/organization/apikeys/" + e2eKey + "/rotate":
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			_, _ = w.Write([]byte(`{"api_key":"` + e2eRotatedKey + `","name":"ci","enabled":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return map[string]string{"DWELLIR_TOKEN": "test-token", "DWELLIR_API_URL": server.URL}
}

func TestKeyCommandsNeverPrintTheKey(t *testing.T) {
	env := keysServer(t)
	for _, args := range [][]string{
		{"keys", "list", "--json"},
		{"keys", "list", "--human"},
		{"keys", "list", "--toon"},
		{"keys", "disable", "ci", "--json"},
		{"keys", "disable", "9f1c****6789", "--json"},
	} {
		res := runCLIWithEnv(t, env, args...)
		if res.exitCode != 0 {
			t.Fatalf("%v: expected success, got %d\nstdout: %s\nstderr: %s", args, res.exitCode, res.stdout, res.stderr)
		}
		output := res.stdout + res.stderr
		if strings.Contains(output, e2eKey) || !strings.Contains(output, "9f1c****6789") {
			t.Fatalf("%v: expected only the masked key, got:\n%s", args, output)
		}
	}
}

func TestKeysRotateNeedsYesWithoutATerminal(t *testing.T) {
	env := keysServer(t)
	for _, format := range []string{"--json", "--toon", "--human"} {
		res := runCLIWithEnv(t, env, "keys", "rotate", "ci", format)
		output := res.stdout + res.stderr
		if res.exitCode != 4 || !strings.Contains(output, "needs confirmation") || !strings.Contains(output, "--yes") {
			t.Fatalf("%s: expected a confirmation error naming --yes, got %d\n%s", format, res.exitCode, output)
		}
		if format == "--json" && !strings.Contains(res.stdout, `"code":"validation_error"`) {
			t.Fatalf("expected validation_error JSON, got: %s", res.stdout)
		}
	}
}

func TestKeysRotatePrintsOnlyTheMaskedKey(t *testing.T) {
	env := keysServer(t)
	for _, format := range []string{"--json", "--toon", "--human"} {
		res := runCLIWithEnv(t, env, "keys", "rotate", "ci", "--yes", format)
		if res.exitCode != 0 {
			t.Fatalf("%s: expected success, got %d\nstdout: %s\nstderr: %s", format, res.exitCode, res.stdout, res.stderr)
		}
		output := res.stdout + res.stderr
		if strings.Contains(output, e2eKey) || strings.Contains(output, e2eRotatedKey) {
			t.Fatalf("%s: output contains a full key:\n%s", format, output)
		}
		for _, want := range []string{"1a2b****f809", "dwellir keys copy <name>", "dwellir keys env <name>"} {
			if !strings.Contains(res.stdout, want) {
				t.Fatalf("%s: expected %q in output, got:\n%s", format, want, res.stdout)
			}
		}
	}
}

func TestKeysEnvWritesTheKeyWithoutPrintingIt(t *testing.T) {
	dir := t.TempDir()
	res := runCLIWithConfigDirAndEnvAndDir(t, t.TempDir(), keysServer(t), dir, "keys", "env", "ci", "--json")
	if res.exitCode != 0 {
		t.Fatalf("expected success, got %d\nstdout: %s\nstderr: %s", res.exitCode, res.stdout, res.stderr)
	}
	if strings.Contains(res.stdout+res.stderr, e2eKey) {
		t.Fatalf("output contains the full key:\n%s", res.stdout)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("expected .env: %v", err)
	}
	if string(data) != "DWELLIR_API_KEY='"+e2eKey+"'\n" {
		t.Fatalf("unexpected .env contents: %q", data)
	}
	if ignore, _ := os.ReadFile(filepath.Join(dir, ".gitignore")); string(ignore) != "/.env\n" {
		t.Fatalf("unexpected .gitignore contents: %q", ignore)
	}
	info, _ := os.Stat(filepath.Join(dir, ".env"))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected mode 0600, got %v", info.Mode().Perm())
	}
}

func TestEndpointsKeyFlagIsRemoved(t *testing.T) {
	for _, args := range [][]string{
		{"endpoints", "list", "--key", "--json"},
		{"endpoints", "get", "base", "--key", "ci", "--json"},
		{"endpoints", "search", "base", "--key=ci", "--json"},
	} {
		res := runCLI(t, args...)
		if res.exitCode == 0 || !strings.Contains(res.stdout, "--key was removed") {
			t.Fatalf("%v: expected a validation error for --key, got %d\n%s", args, res.exitCode, res.stdout)
		}
	}
}

func TestArgumentErrorsMaskKeys(t *testing.T) {
	for _, args := range [][]string{
		{"endpoints", "list", e2eKey},
		{"keys", "create", "--name", "ci", "--daily-quota", e2eKey},
		{"keys", "create", "--name", "ci", "--daily-quota", "KEY_" + e2eKey},
		{e2eKey},
	} {
		for _, format := range []string{"--json", "--toon", "--human"} {
			res := runCLI(t, append(args, format)...)
			if res.exitCode == 0 {
				t.Fatalf("%v %s: expected an error", args, format)
			}
			if output := res.stdout + res.stderr; strings.Contains(output, e2eKey) {
				t.Fatalf("%v %s: error output contains the key:\n%s", args, format, output)
			}
		}
	}
}

func TestMaskedOutputExplainsHowToUseAKey(t *testing.T) {
	env := keysServer(t)
	res := runCLIWithEnv(t, env, "keys", "list", "--json")
	if !strings.Contains(res.stdout, `"notice":"API keys are masked`) || !strings.Contains(res.stdout, "dwellir keys copy <name>") {
		t.Fatalf("expected a notice in meta, got: %s", res.stdout)
	}
	res = runCLIWithEnv(t, env, "keys", "list", "--human")
	if !strings.Contains(res.stdout, "Note: API keys are masked") {
		t.Fatalf("expected a human notice, got:\n%s", res.stdout)
	}
}

func TestRemovedShowSecretsFlagExplainsTheReplacement(t *testing.T) {
	res := runCLI(t, "logs", "errors", "--show-secrets", "--json")
	if res.exitCode == 0 || !strings.Contains(res.stdout, "--show-secrets was removed") || !strings.Contains(res.stdout, "dwellir keys env <name>") {
		t.Fatalf("expected a removal error with a replacement hint, got %d\n%s", res.exitCode, res.stdout)
	}
}

func TestCobraHelpMasksKeys(t *testing.T) {
	res := runCLI(t, "help", e2eKey)
	if output := res.stdout + res.stderr; strings.Contains(output, e2eKey) || !strings.Contains(output, "9f1c****6789") {
		t.Fatalf("expected help output to mask the key, got:\n%s", output)
	}
}

func TestDoctorMasksKeysInAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"key `+e2eKey+` is revoked"}`, http.StatusForbidden)
	}))
	defer server.Close()
	res := runCLIWithEnv(t, map[string]string{"DWELLIR_TOKEN": "test-token", "DWELLIR_API_URL": server.URL}, "doctor", "--verify-api", "--json")
	if output := res.stdout + res.stderr; strings.Contains(output, e2eKey) {
		t.Fatalf("doctor output contains the key:\n%s", output)
	}
}

func TestKeysDeleteExplainsTheReplacement(t *testing.T) {
	res := runCLIWithEnv(t, keysServer(t), "keys", "delete", "ci", "--json")
	if res.exitCode != 0 || !strings.Contains(res.stdout, `"notice":"Requests that use the deleted key now fail`) {
		t.Fatalf("expected a delete notice, got %d\n%s", res.exitCode, res.stdout)
	}
}

func TestNetworkErrorsExplainTheNextStep(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	res := runCLIWithEnv(t, map[string]string{"DWELLIR_TOKEN": "test-token", "DWELLIR_API_URL": server.URL}, "keys", "list", "--json")
	if !strings.Contains(res.stdout, `"code":"network_error"`) || !strings.Contains(res.stdout, "Check the network connection") {
		t.Fatalf("expected a network error with a hint, got:\n%s", res.stdout)
	}
}
