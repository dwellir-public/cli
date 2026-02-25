//go:build e2e

package e2e

import "testing"

func TestConfigSetAndGet(t *testing.T) {
	configDir := t.TempDir()

	result := runCLIWithConfigDir(t, configDir, "config", "set", "output", "json")
	if result.exitCode != 0 {
		t.Fatalf("config set failed: %s", result.stderr)
	}

	result = runCLIWithConfigDir(t, configDir, "config", "get", "output", "--json")
	if result.exitCode != 0 {
		t.Fatalf("config get failed: %s", result.stderr)
	}
	parsed := parseJSON(t, result.stdout)
	if parsed["ok"] != true {
		t.Errorf("expected ok: true")
	}
}

func TestKeysListNoAuth(t *testing.T) {
	result := runCLI(t, "keys", "list", "--json")
	if result.exitCode == 0 {
		t.Fatal("expected non-zero exit for unauthenticated request")
	}
}

func TestMissingArgs(t *testing.T) {
	result := runCLI(t, "keys", "enable")
	if result.exitCode == 0 {
		t.Fatal("expected non-zero exit for missing required arg")
	}
}
