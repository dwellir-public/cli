//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

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

func TestConfigOutputHumanOverridesAgentDefault(t *testing.T) {
	configDir := t.TempDir()

	set := runCLIWithConfigDirAndEnv(t, configDir, map[string]string{"CODEX_CI": "1"}, "config", "set", "output", "human")
	if set.exitCode != 0 {
		t.Fatalf("config set failed: %s", set.stderr)
	}

	version := runCLIWithConfigDirAndEnv(t, configDir, map[string]string{"CODEX_CI": "1"}, "version")
	if version.exitCode != 0 {
		t.Fatalf("version command failed: %s", version.stderr)
	}
	if strings.Contains(version.stdout, "{\"ok\":") {
		t.Fatalf("expected human output to be respected from config, got JSON: %s", version.stdout)
	}
	if !strings.Contains(version.stdout, "Version") {
		t.Fatalf("expected human version output, got: %s", version.stdout)
	}
}

func TestNoConfigStillDefaultsJSONInAgentEnv(t *testing.T) {
	version := runCLIWithConfigDirAndEnv(t, t.TempDir(), map[string]string{"CODEX_CI": "1"}, "version")
	if version.exitCode != 0 {
		t.Fatalf("version command failed: %s", version.stderr)
	}
	if !strings.Contains(version.stdout, "{\"ok\":") {
		t.Fatalf("expected JSON output in agent env without explicit config, got: %s", version.stdout)
	}
}
