package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dwellir-public/cli/internal/config"
)

func TestResolveTokenFromEnv(t *testing.T) {
	t.Setenv("DWELLIR_TOKEN", "env-token-123")
	token, err := ResolveToken("", "", t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "env-token-123" {
		t.Errorf("expected env token, got: '%s'", token)
	}
}

func TestResolveTokenFromFlag(t *testing.T) {
	token, err := ResolveToken("flag-token", "", t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "flag-token" {
		t.Errorf("expected flag token, got: '%s'", token)
	}
}

func TestResolveTokenFromProfile(t *testing.T) {
	configDir := t.TempDir()
	p := &config.Profile{
		Name:  "default",
		Token: "profile-token",
	}
	config.SaveProfile(configDir, p)

	token, err := ResolveToken("", "", t.TempDir(), configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "profile-token" {
		t.Errorf("expected profile token, got: '%s'", token)
	}
}

func TestResolveTokenFromDwellirJSON(t *testing.T) {
	configDir := t.TempDir()
	projectDir := t.TempDir()

	p := &config.Profile{Name: "work", Token: "work-token"}
	config.SaveProfile(configDir, p)

	os.WriteFile(filepath.Join(projectDir, ".dwellir.json"), []byte(`{"profile":"work"}`), 0o644)

	token, err := ResolveToken("", "", projectDir, configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "work-token" {
		t.Errorf("expected work token, got: '%s'", token)
	}
}

func TestResolveTokenMissing(t *testing.T) {
	_, err := ResolveToken("", "", t.TempDir(), t.TempDir())
	if err == nil {
		t.Fatal("expected error when no token is available")
	}
}
