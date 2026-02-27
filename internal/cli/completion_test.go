package cli

import "testing"

func TestCompletionInstallPathSupportedShells(t *testing.T) {
	cases := []string{"bash", "zsh", "fish", "powershell"}
	for _, shell := range cases {
		path, hint, err := completionInstallPath(shell)
		if err != nil {
			t.Fatalf("expected no error for %s: %v", shell, err)
		}
		if path == "" || hint == "" {
			t.Fatalf("expected path and hint for %s", shell)
		}
	}
}

func TestGenerateCompletionScriptUnsupportedShell(t *testing.T) {
	if _, err := generateCompletionScript("invalid-shell"); err == nil {
		t.Fatalf("expected error for unsupported shell")
	}
}
