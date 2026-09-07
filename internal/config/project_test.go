package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectEnvGitNegation(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	path := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(path, []byte("*\n!/.env\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteProjectEnv(dir, ".env", map[string]string{"DWELLIR_API_KEY": "secret"}, false); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "check-ignore", ".env")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("credentials not ignored: %v %s", err, out)
	}
	info, err := os.Stat(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %v", info.Mode())
	}
}

func TestProjectEnvPreservesSettingsAndRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	original := "# application\nDATABASE_URL='postgres://local'\nDWELLIR_API_KEY='old'\nDWELLIR_WSS_URL='wss://old'\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"DWELLIR_API_KEY": "new", "DWELLIR_WSS_URL": ""}
	if err := WriteProjectEnv(dir, ".env", values, false); err == nil {
		t.Fatal("expected conflict")
	}
	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Fatal("changed file on conflict")
	}
	if err := WriteProjectEnv(dir, ".env", values, true); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(path)
	if !strings.Contains(string(got), "DATABASE_URL='postgres://local'") || strings.Contains(string(got), "wss://old") {
		t.Fatalf("incorrect replacement: %s", got)
	}
}

func TestProjectEnvRefusesSymlinks(t *testing.T) {
	for _, name := range []string{".env", ".gitignore"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(t.TempDir(), "target")
			if err := os.WriteFile(target, []byte("untouched"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
			if err := ValidateProjectEnv(dir, ".env", true); err == nil {
				t.Fatal("accepted symlink")
			}
			if err := WriteProjectEnv(dir, ".env", map[string]string{"DWELLIR_API_KEY": "new"}, true); err == nil {
				t.Fatal("accepted symlink")
			}
			got, _ := os.ReadFile(target)
			if string(got) != "untouched" {
				t.Fatal("modified target")
			}
		})
	}
}
