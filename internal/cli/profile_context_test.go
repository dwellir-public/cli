package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProfileContext_FlagWins(t *testing.T) {
	ctx := resolveProfileContext("work", t.TempDir(), t.TempDir())
	if ctx.Name != "work" {
		t.Fatalf("name = %q, want %q", ctx.Name, "work")
	}
	if ctx.Source != "flag" {
		t.Fatalf("source = %q, want %q", ctx.Source, "flag")
	}
}

func TestResolveProfileContext_EnvWins(t *testing.T) {
	t.Setenv("DWELLIR_PROFILE", "env-profile")
	ctx := resolveProfileContext("", t.TempDir(), t.TempDir())
	if ctx.Name != "env-profile" {
		t.Fatalf("name = %q, want %q", ctx.Name, "env-profile")
	}
	if ctx.Source != "env" {
		t.Fatalf("source = %q, want %q", ctx.Source, "env")
	}
}

func TestResolveProfileContext_DwellirJSONWins(t *testing.T) {
	configDir := t.TempDir()
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, ".dwellir.json"), []byte(`{"profile":"project"}`), 0o644); err != nil {
		t.Fatalf("write .dwellir.json: %v", err)
	}

	ctx := resolveProfileContext("", projectDir, configDir)
	if ctx.Name != "project" {
		t.Fatalf("name = %q, want %q", ctx.Name, "project")
	}
	if ctx.Source != "dwellir_json" {
		t.Fatalf("source = %q, want %q", ctx.Source, "dwellir_json")
	}
}

func TestResolveProfileContext_ConfigDefaultWins(t *testing.T) {
	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{"output":"human","default_profile":"bench"}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	ctx := resolveProfileContext("", t.TempDir(), configDir)
	if ctx.Name != "bench" {
		t.Fatalf("name = %q, want %q", ctx.Name, "bench")
	}
	if ctx.Source != "config_default" {
		t.Fatalf("source = %q, want %q", ctx.Source, "config_default")
	}
}

func TestResolveProfileContext_FallbackDefault(t *testing.T) {
	ctx := resolveProfileContext("", t.TempDir(), t.TempDir())
	if ctx.Name != "default" {
		t.Fatalf("name = %q, want %q", ctx.Name, "default")
	}
	if ctx.Source != "fallback_default" {
		t.Fatalf("source = %q, want %q", ctx.Source, "fallback_default")
	}
}
