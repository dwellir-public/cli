package cli

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dwellir-public/cli/internal/api"
	"github.com/dwellir-public/cli/internal/output"
)

const (
	testKeyValue        = "9f1c2d3e-4b5a-6789-abcd-ef0123456789"
	testRotatedKeyValue = "1a2b3c4d-5e6f-7081-92a3-b4c5d6e7f809"
)

func TestDeleteSuccessPayloadMasksTheKey(t *testing.T) {
	key := api.APIKey{APIKey: api.NewSecret(testKeyValue), Name: "ci"}
	cases := map[string]struct {
		result *api.DeleteKeyResult
		status string
	}{
		"cleanup pending": {&api.DeleteKeyResult{CleanupPending: true}, "cleanup_pending"},
		"deleted":         {&api.DeleteKeyResult{}, "deleted"},
		"nil result":      {nil, "deleted"},
	}
	for name, tc := range cases {
		got := deleteSuccessPayload(key, tc.result)
		want := map[string]interface{}{"key": "9f1c****6789", "name": "ci", "status": tc.status}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: unexpected payload %#v", name, got)
		}
	}
}

func TestKeyFilterValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"api_key":"` + testKeyValue + `","name":"ci"}]`))
	}))
	defer server.Close()
	client := api.NewClient(server.URL, "token")

	cases := map[string]string{
		"ci":                 testKeyValue,
		"9f1c****6789":       testKeyValue,
		testKeyValue:         testKeyValue,
		"deleted-full-value": "deleted-full-value",
		"":                   "",
	}
	for selector, want := range cases {
		if got, err := keyFilterValue(client, selector); err != nil || got != want {
			t.Errorf("keyFilterValue(%q) = %q, %v; want %q", selector, got, err, want)
		}
	}
}

func TestClipboardToolsFollowTheDisplayServer(t *testing.T) {
	env := func(vars map[string]string) func(string) string {
		return func(name string) string { return vars[name] }
	}
	names := func(tools []clipboardTool) string {
		var out []string
		for _, tool := range tools {
			out = append(out, tool.name)
		}
		return strings.Join(out, ",")
	}

	cases := []struct {
		goos string
		env  map[string]string
		want string
	}{
		{"darwin", nil, "pbcopy"},
		{"windows", nil, "clip.exe"},
		{"linux", map[string]string{"WAYLAND_DISPLAY": "wayland-1"}, "wl-copy,clip.exe,termux-clipboard-set"},
		{"linux", map[string]string{"DISPLAY": ":0"}, "xclip,xsel,clip.exe,termux-clipboard-set"},
		{"freebsd", map[string]string{"WAYLAND_DISPLAY": "w", "DISPLAY": ":0"}, "wl-copy,xclip,xsel,clip.exe,termux-clipboard-set"},
		{"linux", nil, "clip.exe,termux-clipboard-set"},
		{"linux", map[string]string{"WSL_DISTRO_NAME": "Ubuntu", "WAYLAND_DISPLAY": "wayland-0"}, "clip.exe,wl-copy,termux-clipboard-set"},
	}
	for _, tc := range cases {
		if got := names(clipboardTools(tc.goos, env(tc.env))); got != tc.want {
			t.Errorf("%s %v: got %s, want %s", tc.goos, tc.env, got, tc.want)
		}
	}
}

func TestCopyToClipboardSendsTheKeyOnStdin(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("fakes wl-copy, which only Linux selects")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "clipboard")
	script := "#!/bin/sh\nprintf '%s' \"$*\" > " + out + ".args\ncat > " + out + "\n"
	if err := os.WriteFile(filepath.Join(dir, "wl-copy"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")
	t.Setenv("DISPLAY", "")
	t.Setenv("WSL_DISTRO_NAME", "")
	t.Setenv("WSL_INTEROP", "")

	tool, err := copyToClipboard(testKeyValue)
	if err != nil || tool != "wl-copy" {
		t.Fatalf("expected wl-copy to take the key, got %q, %v", tool, err)
	}
	if got, _ := os.ReadFile(out); string(got) != testKeyValue {
		t.Fatalf("expected the key on stdin, got %q", got)
	}
	if args, _ := os.ReadFile(out + ".args"); strings.Contains(string(args), testKeyValue) {
		t.Fatalf("the key must not appear in process arguments, got %q", args)
	}
}

func TestCopyToClipboardWithoutAToolFails(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")
	if _, err := copyToClipboard(testKeyValue); err != errNoClipboard {
		t.Fatalf("expected errNoClipboard, got %v", err)
	}
}

// rotateServer serves one key named ci and its rotate endpoint. It returns
// the number of rotate requests it received.
func rotateServer(t *testing.T) *int32 {
	t.Helper()
	var rotations int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v4/organization/apikeys":
			_, _ = w.Write([]byte(`[{"api_key":"` + testKeyValue + `","name":"ci","enabled":true}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/v4/organization/apikeys/"+testKeyValue+"/rotate":
			atomic.AddInt32(&rotations, 1)
			_, _ = w.Write([]byte(`{"api_key":"` + testRotatedKeyValue + `","name":"ci","enabled":true}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("DWELLIR_API_URL", server.URL)
	t.Setenv("DWELLIR_TOKEN", "test-token")
	return &rotations
}

type rotateRun struct {
	stdout, stderr string
	err            error
}

func runKeysRotate(t *testing.T, stdin string, terminal bool, args ...string) rotateRun {
	t.Helper()
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	setStdoutTerminalForTest(t, true)

	oldStdinIsTerminal := stdinIsTerminal
	oldArgs := os.Args
	oldTelemetry := telemetryClient
	stdinIsTerminal = func() bool { return terminal }
	telemetryClient = &fakeTelemetry{}
	args = append([]string{"keys", "rotate"}, args...)
	os.Args = append([]string{"dwellir"}, args...)

	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetIn(strings.NewReader(stdin))
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		stdinIsTerminal = oldStdinIsTerminal
		os.Args = oldArgs
		telemetryClient = oldTelemetry
		keyRotateYes = false
		rootCmd.SetIn(nil)
		rootCmd.SetArgs(nil)
	})
	err := Execute()
	return rotateRun{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func TestKeysRotateNeedsYesWhenItCannotPrompt(t *testing.T) {
	cases := map[string]struct {
		terminal bool
		args     []string
	}{
		"default in a terminal":     {true, []string{"ci"}},
		"json in a terminal":        {true, []string{"ci", "--json"}},
		"toon in a terminal":        {true, []string{"ci", "--toon"}},
		"human without a terminal":  {false, []string{"ci", "--human"}},
		"json answers are not read": {false, []string{"ci", "--json"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rotations := rotateServer(t)
			res := runKeysRotate(t, "y\n", tc.terminal, tc.args...)
			var rendered *output.RenderedError
			if !errors.As(res.err, &rendered) || rendered.Code != "validation_error" {
				t.Fatalf("expected a validation_error, got %v", res.err)
			}
			if *rotations != 0 {
				t.Fatalf("expected no rotate request, got %d", *rotations)
			}
			if !strings.Contains(res.stdout, "--yes") || !strings.Contains(res.stdout, "stops working") {
				t.Fatalf("expected a hint naming --yes and the impact, got:\n%s", res.stdout)
			}
		})
	}
}

func TestKeysRotatePromptCancels(t *testing.T) {
	for name, answer := range map[string]string{"no": "n\n", "empty": "\n", "eof": ""} {
		t.Run(name, func(t *testing.T) {
			rotations := rotateServer(t)
			res := runKeysRotate(t, answer, true, "ci", "--human")
			if res.err != nil {
				t.Fatalf("unexpected error: %v\n%s", res.err, res.stdout)
			}
			if *rotations != 0 {
				t.Fatalf("expected no rotate request, got %d", *rotations)
			}
			if !strings.Contains(res.stderr, `Rotate API key "ci" (9f1c****6789)?`) ||
				!strings.Contains(res.stderr, "stops working at once") {
				t.Fatalf("expected a prompt that names the key and the impact, got:\n%s", res.stderr)
			}
			if !strings.Contains(res.stdout, "cancelled") {
				t.Fatalf("expected a cancelled status, got:\n%s", res.stdout)
			}
		})
	}
}

func TestKeysRotatePrintsOnlyTheMaskedKey(t *testing.T) {
	cases := map[string]struct {
		stdin    string
		terminal bool
		args     []string
		notice   string
	}{
		"confirmed at the prompt": {"y\n", true, []string{"ci", "--human"}, "Note: The old key value no longer works."},
		"yes with json":           {"", false, []string{"ci", "--yes", "--json"}, `"notice":"The old key value no longer works.`},
		"short yes with toon":     {"", false, []string{"ci", "-y", "--toon"}, "The old key value no longer works."},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rotations := rotateServer(t)
			res := runKeysRotate(t, tc.stdin, tc.terminal, tc.args...)
			if res.err != nil {
				t.Fatalf("unexpected error: %v\n%s", res.err, res.stdout)
			}
			if *rotations != 1 {
				t.Fatalf("expected one rotate request, got %d", *rotations)
			}
			output := res.stdout + res.stderr
			if strings.Contains(output, testRotatedKeyValue) || strings.Contains(output, testKeyValue) {
				t.Fatalf("output contains a full key:\n%s", output)
			}
			for _, want := range []string{"1a2b****f809", tc.notice, "dwellir keys copy <name>", "dwellir keys env <name>"} {
				if !strings.Contains(res.stdout, want) {
					t.Errorf("expected %q in output, got:\n%s", want, res.stdout)
				}
			}
		})
	}
}

func TestKeysRotateLostResponseSaysTheKeyMayHaveRotated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[{"api_key":"` + testKeyValue + `","name":"ci","enabled":true}]`))
			return
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		_ = conn.Close()
	}))
	t.Cleanup(server.Close)
	t.Setenv("DWELLIR_API_URL", server.URL)
	t.Setenv("DWELLIR_TOKEN", "test-token")

	res := runKeysRotate(t, "", false, "ci", "--yes", "--json")
	var rendered *output.RenderedError
	if !errors.As(res.err, &rendered) || rendered.Code != "network_error" {
		t.Fatalf("expected a network_error, got %v", res.err)
	}
	if !strings.Contains(res.stdout, "dwellir keys list") || !strings.Contains(res.stdout, "9f1c****6789") {
		t.Fatalf("expected a hint to check the key list, got:\n%s", res.stdout)
	}
	if strings.Contains(res.stdout, testKeyValue) {
		t.Fatalf("output contains the full key:\n%s", res.stdout)
	}
}

func TestKeysRotateServerErrorSaysCheckBeforeRetrying(t *testing.T) {
	var rotations int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[{"api_key":"` + testKeyValue + `","name":"ci","enabled":true}]`))
			return
		}
		// The rotation commits, then a proxy loses the answer.
		atomic.AddInt32(&rotations, 1)
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)
	t.Setenv("DWELLIR_API_URL", server.URL)
	t.Setenv("DWELLIR_TOKEN", "test-token")

	res := runKeysRotate(t, "", false, "ci", "--yes", "--json")
	var rendered *output.RenderedError
	if !errors.As(res.err, &rendered) || rendered.Code != "api_error" {
		t.Fatalf("expected an api_error, got %v", res.err)
	}
	if !strings.Contains(res.stdout, "dwellir keys list") || !strings.Contains(res.stdout, "Do not retry yet") {
		t.Fatalf("expected a hint to check the key list before retrying, got:\n%s", res.stdout)
	}
	if strings.Contains(res.stdout, "Retry the command") {
		t.Fatalf("a 5xx after rotate must not advise a retry:\n%s", res.stdout)
	}
	if got := atomic.LoadInt32(&rotations); got != 1 {
		t.Fatalf("expected exactly 1 rotate request, got %d", got)
	}
}

func TestKeysRotateExitCodes(t *testing.T) {
	cases := map[string]struct {
		setup func(t *testing.T)
		args  []string
		want  int
	}{
		"no confirmation":  {func(t *testing.T) { rotateServer(t) }, []string{"ci", "--json"}, exitValidation},
		"missing argument": {func(t *testing.T) { rotateServer(t) }, []string{"--yes", "--json"}, exitValidation},
		"unknown key":      {func(t *testing.T) { rotateServer(t) }, []string{"nope", "--yes", "--json"}, exitNotFound},
		"not signed in": {func(t *testing.T) {
			t.Setenv("DWELLIR_TOKEN", "")
			t.Setenv("DWELLIR_API_URL", "http://127.0.0.1:1")
		}, []string{"ci", "--yes", "--json"}, exitAuth},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.setup(t)
			res := runKeysRotate(t, "", false, tc.args...)
			if got := ExitCode(res.err); got != tc.want {
				t.Fatalf("exit code = %d, want %d\n%s", got, tc.want, res.stdout)
			}
		})
	}
}
