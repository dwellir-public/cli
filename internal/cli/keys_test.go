package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/dwellir-public/cli/internal/api"
)

const testKeyValue = "9f1c2d3e-4b5a-6789-abcd-ef0123456789"

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
