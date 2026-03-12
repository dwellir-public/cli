package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefault(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Output != "human" {
		t.Errorf("expected default output 'human', got '%s'", cfg.Output)
	}
	if cfg.DefaultProfile != "default" {
		t.Errorf("expected default profile 'default', got '%s'", cfg.DefaultProfile)
	}
}

func TestSetAndGet(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := Load(dir)
	err := cfg.Set("output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val := cfg.Get("output")
	if val != "json" {
		t.Errorf("expected 'json', got '%s'", val)
	}
}

func TestSetOutputTOON(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := Load(dir)
	if err := cfg.Set("output", "toon"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.Get("output"); got != "toon" {
		t.Fatalf("expected 'toon', got '%s'", got)
	}
}

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := Load(dir)
	if err := cfg.Set("output", "json"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err := cfg.Save()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg2, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg2.Output != "json" {
		t.Errorf("expected 'json' after reload, got '%s'", cfg2.Output)
	}
}

func TestSetDefaultProfileDoesNotPersistImplicitHumanOutput(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cfg.Set("default_profile", "work"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var stored map[string]string
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if _, ok := stored["output"]; ok {
		t.Fatalf("expected output to be omitted, got %q", stored["output"])
	}
	if got := stored["default_profile"]; got != "work" {
		t.Fatalf("default_profile = %q, want %q", got, "work")
	}
}

func TestSetExplicitHumanOutputPersistsOutput(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cfg.Set("output", "human"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var stored map[string]string
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if got := stored["output"]; got != "human" {
		t.Fatalf("output = %q, want %q", got, "human")
	}
}

func TestProfileSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	p := &Profile{
		Name:  "work",
		Token: "test-token-123",
		Org:   "my-org",
		User:  "me@example.com",
	}
	err := SaveProfile(dir, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	loaded, err := LoadProfile(dir, "work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.Token != "test-token-123" {
		t.Errorf("expected token 'test-token-123', got '%s'", loaded.Token)
	}
}

func TestResolveProfileFromDwellirJSON(t *testing.T) {
	configDir := t.TempDir()
	projectDir := t.TempDir()

	content := []byte(`{"profile": "work"}`)
	if err := os.WriteFile(filepath.Join(projectDir, ".dwellir.json"), content, 0o644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	name := ResolveProfileName("", "", projectDir, configDir)
	if name != "work" {
		t.Errorf("expected 'work' from .dwellir.json, got '%s'", name)
	}
}
