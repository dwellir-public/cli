package cli

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	key := api.APIKey{APIKey: api.NewSecret("secret"), Enabled: true, DailyQuota: &limit}
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

func TestProjectNodeListsCandidatesWhenAmbiguous(t *testing.T) {
	chains := []api.Chain{{Networks: []api.Network{
		{Name: "Mainnet", Nodes: []api.Node{{HTTPS: "https://a.example", NodeType: api.NodeType{Name: "Archive"}}}},
		{Name: "Mainnet (Full)", Nodes: []api.Node{{HTTPS: "https://b.example", NodeType: api.NodeType{Name: "Full"}}}},
	}}}
	_, err := projectNode(chains)
	if err == nil {
		t.Fatal("expected an error for two matching endpoints")
	}
	for _, want := range []string{`--network "Mainnet" --node-type Archive`, `--network "Mainnet (Full)" --node-type Full`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %s in error, got %q", want, err.Error())
		}
	}
}

func TestProjectSetupWritesGRPCOnlyEndpoint(t *testing.T) {
	const target = "api-hyperliquid-mainnet-grpc.n.dwellir.com:443"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/chains":
			_, _ = w.Write([]byte(`[{"id": 1, "name": "Hyperliquid gRPC", "networks": [{"id": 1, "name": "Mainnet", "nodes": [
  {"id": 1, "https": null, "wss": null, "grpc": "` + target + `", "node_type": {"name": "Full"}}]}]}]`))
		case "/v4/organization/apikeys":
			_, _ = w.Write([]byte(`[{"api_key": "synthetic-key", "name": "project", "enabled": true}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	t.Setenv("DWELLIR_TOKEN", "synthetic-token")
	t.Setenv("DWELLIR_API_URL", server.URL)

	cmd := newProjectCommand()
	cmd.SetArgs([]string{"setup", "--chain", "hyperliquid-grpc", "--network", "mainnet", "--key-name", "project"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("setup rejected a gRPC-only endpoint: %v", err)
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"DWELLIR_API_KEY='synthetic-key'", "DWELLIR_GRPC_TARGET='" + target + "'", "DWELLIR_RPC_URL=''", "DWELLIR_WSS_URL=''"} {
		if !strings.Contains(string(env), want) {
			t.Errorf("expected %s in .env, got:\n%s", want, env)
		}
	}
}

func TestProjectRejectsUnsafeGRPCTarget(t *testing.T) {
	for _, target := range []string{
		"api.example.com:443",
		"api-x.n.dwellir.com",
		"api-x.n.dwellir.com:50051",
		"api-x.n.dwellir.com:443/secret",
		"user@api-x.n.dwellir.com:443",
	} {
		if err := validateProjectGRPCTarget(target); err == nil {
			t.Errorf("accepted %q", target)
		}
	}
}
