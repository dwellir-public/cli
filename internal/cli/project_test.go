package cli

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dwellir-public/cli/internal/api"
	"github.com/dwellir-public/cli/internal/output"
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

func TestProjectKeyRejectsExplicitZeroQuotaMismatch(t *testing.T) {
	limit := 100
	key := api.APIKey{APIKey: "secret", Enabled: true, DailyQuota: &limit}
	if _, err := existingProjectKey(key, "project", new(int), nil); err == nil {
		t.Fatal("accepted explicit unlimited quota for a limited key")
	}
}

func TestProjectSetupAuthenticationErrors(t *testing.T) {
	for _, status := range []int{0, 401, 403} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
			t.Setenv("DWELLIR_TOKEN", "")
			want := "not_authenticated"
			if status != 0 {
				want = "forbidden"
				t.Setenv("DWELLIR_TOKEN", "synthetic-token")
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(status)
				}))
				defer server.Close()
				t.Setenv("DWELLIR_API_URL", server.URL)
			}
			cmd := newProjectCommand()
			cmd.SetArgs([]string{"setup", "--chain", "ethereum", "--network", "mainnet", "--create-key", "project"})
			err := cmd.Execute()
			var rendered *output.RenderedError
			if !errors.As(err, &rendered) || rendered.Code != want {
				t.Fatalf("got %v; want rendered %s", err, want)
			}
		})
	}
}
