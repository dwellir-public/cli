package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestProjectRefusesTrackedAndBrokenGitIndex(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "Test"}, {"config", "user.email", "test@example.invalid"}} {
		command := exec.Command("git", args...)
		command.Dir = dir
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git: %v: %s", err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("OTHER=value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "add", ".env")
	command.Dir = dir
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	if err := validateUntrackedEnv(dir, ".env"); err == nil {
		t.Fatal("accepted tracked file")
	}
	broken := filepath.Join(dir, "broken-index")
	if err := os.WriteFile(broken, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_INDEX_FILE", broken)
	if err := validateUntrackedEnv(dir, ".env"); err == nil {
		t.Fatal("accepted corrupt Git index")
	}
}

func TestProjectAllowsUntrackedAndNewProject(t *testing.T) {
	dir := t.TempDir()
	if err := validateUntrackedEnv(dir, ".env"); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "init", "-q", dir)
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	if err := validateUntrackedEnv(dir, ".env"); err != nil {
		t.Fatal(err)
	}
}
