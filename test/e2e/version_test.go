//go:build e2e

package e2e

import "testing"

func TestVersionJSON(t *testing.T) {
	result := runCLI(t, "version", "--json")
	if result.exitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", result.exitCode, result.stderr)
	}
	parsed := parseJSON(t, result.stdout)
	if parsed["ok"] != true {
		t.Errorf("expected ok: true, got: %v", parsed["ok"])
	}
	data := parsed["data"].(map[string]interface{})
	if _, ok := data["version"]; !ok {
		t.Error("expected 'version' in data")
	}
}

func TestVersionHuman(t *testing.T) {
	result := runCLI(t, "version", "--human")
	if result.exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.exitCode)
	}
	if result.stdout == "" {
		t.Error("expected non-empty output")
	}
}

func TestHelpOutput(t *testing.T) {
	result := runCLI(t, "--help")
	if result.exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.exitCode)
	}
	if result.stdout == "" {
		t.Error("expected help output")
	}
}
