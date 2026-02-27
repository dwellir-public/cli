package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvedOutputFormat_DefaultHuman(t *testing.T) {
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	clearAgentMarkers(t)
	resetOutputFlagsForTest(t)

	if got := resolvedOutputFormat(); got != "human" {
		t.Fatalf("resolvedOutputFormat() = %q, want %q", got, "human")
	}
}

func TestResolvedOutputFormat_DefaultsJSONForAgentEnv(t *testing.T) {
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	t.Setenv("CODEX_CI", "1")

	if got := resolvedOutputFormat(); got != "json" {
		t.Fatalf("resolvedOutputFormat() = %q, want %q", got, "json")
	}
}

func TestResolvedOutputFormat_HumanFlagOverridesAgentDefault(t *testing.T) {
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	t.Setenv("CODEX_CI", "1")
	humanOutput = true

	if got := resolvedOutputFormat(); got != "human" {
		t.Fatalf("resolvedOutputFormat() = %q, want %q", got, "human")
	}
}

func TestResolvedOutputFormat_JSONFlagOverridesHumanConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DWELLIR_CONFIG_DIR", dir)
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"output":"human","default_profile":"default"}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	jsonOutput = true

	if got := resolvedOutputFormat(); got != "json" {
		t.Fatalf("resolvedOutputFormat() = %q, want %q", got, "json")
	}
}

func resetOutputFlagsForTest(t *testing.T) {
	t.Helper()
	oldJSON := jsonOutput
	oldHuman := humanOutput
	t.Cleanup(func() {
		jsonOutput = oldJSON
		humanOutput = oldHuman
	})
	jsonOutput = false
	humanOutput = false
}

func clearAgentMarkers(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"CODEX_CI",
		"CODEX_THREAD_ID",
		"CLAUDECODE",
		"CLAUDE_CODE_ENTRYPOINT",
		"OPENCODE",
		"CURSOR_AGENT",
	} {
		t.Setenv(key, "")
	}
}
