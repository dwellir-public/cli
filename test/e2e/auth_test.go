//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

func TestAuthLoginUsesResolvedDefaultProfileFromConfig(t *testing.T) {
	configDir := t.TempDir()

	setDefault := runCLIWithConfigDir(t, configDir, "config", "set", "default_profile", "bench")
	if setDefault.exitCode != 0 {
		t.Fatalf("config set default_profile failed: %s", setDefault.stderr)
	}

	login := runCLIWithConfigDir(t, configDir, "auth", "login", "--token", "bench-token-123")
	if login.exitCode != 0 {
		t.Fatalf("auth login failed: %s", login.stderr)
	}

	token := runCLIWithConfigDir(t, configDir, "auth", "token")
	if token.exitCode != 0 {
		t.Fatalf("auth token failed (expected success): stderr=%s stdout=%s", token.stderr, token.stdout)
	}
	if strings.TrimSpace(token.stdout) != "bench-token-123" {
		t.Fatalf("auth token output = %q, want %q", strings.TrimSpace(token.stdout), "bench-token-123")
	}
}
