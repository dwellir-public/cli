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

const e2eKey = "9f1c2d3e-4b5a-6789-abcd-ef0123456789"

func keysServer(t *testing.T) map[string]string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v4/organization/apikeys":
			_, _ = w.Write([]byte(`[{"api_key":"` + e2eKey + `","name":"ci","enabled":true}]`))
		case "/v4/organization/apikeys/" + e2eKey:
			_, _ = w.Write([]byte(`{"api_key":"` + e2eKey + `","name":"ci","enabled":false}`))
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
