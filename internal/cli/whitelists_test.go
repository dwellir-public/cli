package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/dwellir-public/cli/internal/output"
)

// fakeWhitelist is server state. It keeps full key values, so it cannot use
// api.Whitelist, whose keys always encode masked.
type fakeWhitelist struct {
	ID      int64
	Name    string
	Enabled bool
	AllKeys bool
	Entries []string
	Keys    []string
}

// fakeMarly serves the key list and the IP whitelist endpoints, and records
// every request as "METHOD path body".
type fakeMarly struct {
	mu         sync.Mutex
	whitelists []*fakeWhitelist
	requests   []string
	// fail answers a request whose "METHOD path" matches with this status.
	fail map[string]int
}

func newFakeMarly(t *testing.T, whitelists ...*fakeWhitelist) *fakeMarly {
	t.Helper()
	m := &fakeMarly{whitelists: whitelists, fail: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(m.serve))
	t.Cleanup(server.Close)
	t.Setenv("DWELLIR_API_URL", server.URL)
	t.Setenv("DWELLIR_TOKEN", "test-token")
	return m
}

func (m *fakeMarly) serve(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	m.requests = append(m.requests, strings.TrimSpace(r.Method+" "+r.URL.Path+" "+string(body)))
	if status, ok := m.fail[r.Method+" "+r.URL.Path]; ok {
		http.Error(w, `{"detail":"refused"}`, status)
		return
	}
	const base = "/v4/organization/ip-whitelists"
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v4/organization/apikeys":
		_, _ = fmt.Fprintf(w, `[{"api_key":%q,"name":"ci","enabled":true},{"api_key":%q,"name":"deploy","enabled":true}]`,
			testKeyValue, testRotatedKeyValue)
	case r.Method == http.MethodGet && r.URL.Path == base:
		all := make([]map[string]interface{}, 0, len(m.whitelists))
		for _, wl := range m.whitelists {
			all = append(all, wl.json())
		}
		_ = json.NewEncoder(w).Encode(all)
	case r.Method == http.MethodPost && r.URL.Path == base:
		var input struct {
			Name    string   `json:"name"`
			Entries []string `json:"entries"`
			AllKeys bool     `json:"apply_to_all_api_keys"`
			Enabled bool     `json:"enabled"`
		}
		_ = json.Unmarshal(body, &input)
		wl := &fakeWhitelist{ID: int64(40 + len(m.whitelists)), Name: input.Name, Enabled: input.Enabled,
			AllKeys: input.AllKeys, Entries: input.Entries}
		m.whitelists = append(m.whitelists, wl)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(wl.json())
	default:
		m.serveWhitelist(w, r, strings.TrimPrefix(r.URL.Path, base+"/"), body)
	}
}

func (m *fakeMarly) serveWhitelist(w http.ResponseWriter, r *http.Request, rest string, body []byte) {
	idPart, key, hasKey := strings.Cut(strings.Replace(rest, "/api-keys/", "/", 1), "/")
	var wl *fakeWhitelist
	for _, candidate := range m.whitelists {
		if fmt.Sprint(candidate.ID) == idPart {
			wl = candidate
		}
	}
	if wl == nil {
		http.Error(w, `{"detail":"IP whitelist not found"}`, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == http.MethodPatch && !hasKey:
		var input struct {
			Name    *string  `json:"name"`
			Entries []string `json:"entries"`
			AllKeys *bool    `json:"apply_to_all_api_keys"`
			Enabled *bool    `json:"enabled"`
		}
		_ = json.Unmarshal(body, &input)
		if input.Name != nil {
			wl.Name = *input.Name
		}
		if input.Entries != nil {
			wl.Entries = input.Entries
		}
		if input.AllKeys != nil {
			wl.AllKeys = *input.AllKeys
		}
		if input.Enabled != nil {
			wl.Enabled = *input.Enabled
		}
	case r.Method == http.MethodDelete && !hasKey:
		w.WriteHeader(http.StatusNoContent)
		return
	case r.Method == http.MethodPost && hasKey:
		wl.Keys = append(wl.Keys, key)
	case r.Method == http.MethodDelete && hasKey:
		var kept []string
		for _, assigned := range wl.Keys {
			if assigned != key {
				kept = append(kept, assigned)
			}
		}
		wl.Keys = kept
	default:
		http.NotFound(w, r)
		return
	}
	_ = json.NewEncoder(w).Encode(wl.json())
}

func (wl *fakeWhitelist) json() map[string]interface{} {
	entries := []map[string]interface{}{}
	for i, cidr := range wl.Entries {
		entries = append(entries, map[string]interface{}{"id": i + 1, "cidr": cidr, "description": nil, "enabled": true})
	}
	keys := []map[string]interface{}{}
	for i, key := range wl.Keys {
		keys = append(keys, map[string]interface{}{"api_key_id": i + 1, "api_key": key, "assignment_kind": "manual", "enabled": true})
	}
	return map[string]interface{}{
		"id": wl.ID, "name": wl.Name, "description": nil, "enabled": wl.Enabled,
		"apply_to_all_api_keys": wl.AllKeys, "entries": entries, "assigned_api_keys": keys,
	}
}

// writes lists the recorded requests that change state.
func (m *fakeMarly) writes() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var writes []string
	for _, request := range m.requests {
		if !strings.HasPrefix(request, "GET ") {
			writes = append(writes, request)
		}
	}
	return writes
}

type cliRun struct {
	stdout, stderr string
	err            error
}

// runWhitelists runs `dwellir whitelists <args>` in process. terminal sets
// whether stdin is a terminal; stdin holds the answer to a prompt.
func runWhitelists(t *testing.T, stdin string, terminal bool, args ...string) cliRun {
	t.Helper()
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	setStdoutTerminalForTest(t, true)

	// Slice flags remember earlier values across runs, so register fresh flags.
	whitelistName, whitelistDescription, whitelistIPs, whitelistKeys = "", "", nil, nil
	whitelistAllKeys, whitelistDisabled, whitelistDeleteYes = false, false, false
	whitelistsCreateCmd.ResetFlags()
	addWhitelistCreateFlags(whitelistsCreateCmd)
	whitelistsUpdateCmd.ResetFlags()
	addWhitelistUpdateFlags(whitelistsUpdateCmd)
	whitelistsDeleteCmd.ResetFlags()
	addWhitelistDeleteFlags(whitelistsDeleteCmd)

	oldStdinIsTerminal := stdinIsTerminal
	oldArgs := os.Args
	oldTelemetry := telemetryClient
	stdinIsTerminal = func() bool { return terminal }
	telemetryClient = &fakeTelemetry{}
	args = append([]string{"whitelists"}, args...)
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
		rootCmd.SetIn(nil)
		rootCmd.SetArgs(nil)
	})
	err := Execute()
	return cliRun{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func assertWrites(t *testing.T, m *fakeMarly, want ...string) {
	t.Helper()
	got := m.writes()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("write requests:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func assertNoFullKey(t *testing.T, run cliRun) {
	t.Helper()
	for _, key := range []string{testKeyValue, testRotatedKeyValue} {
		if strings.Contains(run.stdout+run.stderr, key) {
			t.Fatalf("output contains a full key:\n%s", run.stdout+run.stderr)
		}
	}
}

const wlBase = "/v4/organization/ip-whitelists"

func TestWhitelistsCreateForKeysEnablesAfterAssigning(t *testing.T) {
	m := newFakeMarly(t)
	run := runWhitelists(t, "", false, "create", "--name", "office", "--ip", "203.0.113.10", "--ip", "2001:DB8::5/128",
		"--key", "ci", "--key", "deploy", "--key", "CI", "--json")
	if run.err != nil {
		t.Fatalf("unexpected error: %v\n%s", run.err, run.stdout)
	}
	assertWrites(t, m,
		`POST `+wlBase+` {"name":"office","entries":["203.0.113.10/32","2001:db8::5/128"],"apply_to_all_api_keys":false,"enabled":false}`,
		"POST "+wlBase+"/40/api-keys/"+testKeyValue,
		"POST "+wlBase+"/40/api-keys/"+testRotatedKeyValue,
		`PATCH `+wlBase+`/40 {"enabled":true}`,
	)
	assertNoFullKey(t, run)
	for _, want := range []string{`"enabled":true`, `"name":"ci"`, `"api_key":"9f1c****6789"`, `"notice":"An enabled whitelist limits`} {
		if !strings.Contains(run.stdout, want) {
			t.Errorf("expected %s in output:\n%s", want, run.stdout)
		}
	}
}

func TestWhitelistsCreateVariants(t *testing.T) {
	cases := map[string]struct {
		args  []string
		write string
	}{
		"all keys": {
			[]string{"--all-keys"},
			`POST ` + wlBase + ` {"name":"office","entries":["203.0.113.10/32"],"apply_to_all_api_keys":true,"enabled":true}`,
		},
		"no keys": {
			nil,
			`POST ` + wlBase + ` {"name":"office","entries":["203.0.113.10/32"],"apply_to_all_api_keys":false,"enabled":true}`,
		},
		"disabled with description": {
			[]string{"--disabled", "--description", "Corp NAT"},
			`POST ` + wlBase + ` {"name":"office","description":"Corp NAT","entries":["203.0.113.10/32"],"apply_to_all_api_keys":false,"enabled":false}`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := newFakeMarly(t)
			args := append([]string{"create", "--name", "office", "--ip", "203.0.113.10", "--json"}, tc.args...)
			if run := runWhitelists(t, "", false, args...); run.err != nil {
				t.Fatalf("unexpected error: %v\n%s", run.err, run.stdout)
			}
			assertWrites(t, m, tc.write)
		})
	}
}

func TestWhitelistsCreateDisabledForKeysStaysDisabled(t *testing.T) {
	m := newFakeMarly(t)
	run := runWhitelists(t, "", false, "create", "--name", "office", "--ip", "203.0.113.10", "--key", "ci", "--disabled", "--json")
	if run.err != nil {
		t.Fatalf("unexpected error: %v\n%s", run.err, run.stdout)
	}
	assertWrites(t, m,
		`POST `+wlBase+` {"name":"office","entries":["203.0.113.10/32"],"apply_to_all_api_keys":false,"enabled":false}`,
		"POST "+wlBase+"/40/api-keys/"+testKeyValue,
	)
	if strings.Contains(run.stdout, "notice") {
		t.Errorf("a disabled whitelist with keys restricts nothing, so it needs no notice:\n%s", run.stdout)
	}
}

func TestWhitelistsCreateFailedAssignmentLeavesItDisabled(t *testing.T) {
	m := newFakeMarly(t)
	m.fail["POST "+wlBase+"/40/api-keys/"+testRotatedKeyValue] = http.StatusUnprocessableEntity
	run := runWhitelists(t, "", false, "create", "--name", "office", "--ip", "203.0.113.10", "--key", "ci", "--key", "deploy", "--json")
	var rendered *output.RenderedError
	if !errors.As(run.err, &rendered) || rendered.Code != "validation_error" {
		t.Fatalf("expected a validation_error, got %v\n%s", run.err, run.stdout)
	}
	assertWrites(t, m,
		`POST `+wlBase+` {"name":"office","entries":["203.0.113.10/32"],"apply_to_all_api_keys":false,"enabled":false}`,
		"POST "+wlBase+"/40/api-keys/"+testKeyValue,
		"POST "+wlBase+"/40/api-keys/"+testRotatedKeyValue,
	)
	for _, want := range []string{"(ID 40) disabled", `could not assign key \"deploy\"`,
		"dwellir whitelists assign 40 deploy; then dwellir whitelists enable 40", "refused"} {
		if !strings.Contains(run.stdout, want) {
			t.Errorf("expected %q in output:\n%s", want, run.stdout)
		}
	}
	assertNoFullKey(t, run)
}

func TestWhitelistsCreateRejectsBadInputBeforeWriting(t *testing.T) {
	cases := map[string]struct {
		args []string
		exit int
		want string
	}{
		"missing name":       {[]string{"--ip", "203.0.113.10"}, exitValidation, "--name"},
		"range":              {[]string{"--name", "o", "--ip", "203.0.113.0/24"}, exitValidation, "only single addresses"},
		"ipv6 range":         {[]string{"--name", "o", "--ip", "2001:db8::/64"}, exitValidation, "only single addresses"},
		"not an address":     {[]string{"--name", "o", "--ip", "office"}, exitValidation, "not an IPv4 or IPv6 address"},
		"duplicate":          {[]string{"--name", "o", "--ip", "203.0.113.10", "--ip", "203.0.113.10/32"}, exitValidation, "Duplicate"},
		"no address":         {[]string{"--name", "o"}, exitValidation, "--ip"},
		"keys without ip":    {[]string{"--name", "o", "--disabled", "--key", "ci"}, exitValidation, "--ip"},
		"all keys and a key": {[]string{"--name", "o", "--ip", "203.0.113.10", "--all-keys", "--key", "ci"}, exitValidation, "cannot be used together"},
		"unknown key":        {[]string{"--name", "o", "--ip", "203.0.113.10", "--key", "nope"}, exitNotFound, "no API key matches"},
		"extra argument":     {[]string{"office", "--name", "o", "--ip", "203.0.113.10"}, exitValidation, "Unexpected arguments"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := newFakeMarly(t)
			run := runWhitelists(t, "", false, append([]string{"create", "--json"}, tc.args...)...)
			if got := ExitCode(run.err); got != tc.exit {
				t.Fatalf("exit code = %d, want %d\n%s", got, tc.exit, run.stdout)
			}
			if !strings.Contains(run.stdout, tc.want) {
				t.Errorf("expected %q in output:\n%s", tc.want, run.stdout)
			}
			assertWrites(t, m)
		})
	}
}

func TestWhitelistsCreateAllowsADisabledWhitelistWithoutAddresses(t *testing.T) {
	m := newFakeMarly(t)
	if run := runWhitelists(t, "", false, "create", "--name", "draft", "--disabled", "--json"); run.err != nil {
		t.Fatalf("unexpected error: %v\n%s", run.err, run.stdout)
	}
	assertWrites(t, m, `POST `+wlBase+` {"name":"draft","entries":[],"apply_to_all_api_keys":false,"enabled":false}`)
}

func TestWhitelistsListHuman(t *testing.T) {
	newFakeMarly(t,
		&fakeWhitelist{ID: 7, Name: "office", Enabled: true, Entries: []string{"203.0.113.10/32", "2001:db8::5/128"}, Keys: []string{testKeyValue}},
		&fakeWhitelist{ID: 8, Name: "everyone", AllKeys: true, Entries: []string{"198.51.100.7/32"}},
		&fakeWhitelist{ID: 9, Name: "draft"},
	)
	run := runWhitelists(t, "", false, "list", "--human")
	if run.err != nil {
		t.Fatalf("unexpected error: %v\n%s", run.err, run.stdout)
	}
	for _, want := range []string{"ID", "office", "ci (9f1c****6789)", "203.0.113.10/32, 2001:db8::5/128", "all keys", "draft", "none"} {
		if !strings.Contains(run.stdout, want) {
			t.Errorf("expected %q in output:\n%s", want, run.stdout)
		}
	}
	assertNoFullKey(t, run)
}

func TestWhitelistsListEmpty(t *testing.T) {
	newFakeMarly(t)
	if run := runWhitelists(t, "", false, "list", "--human"); !strings.Contains(run.stdout, "No IP whitelists found.") {
		t.Fatalf("unexpected output: %v\n%s", run.err, run.stdout)
	}
	if run := runWhitelists(t, "", false, "list", "--json"); !strings.Contains(run.stdout, `"data":[]`) {
		t.Fatalf("expected an empty JSON list, got: %v\n%s", run.err, run.stdout)
	}
}

func TestWhitelistsUpdateEnableDisable(t *testing.T) {
	cases := map[string]struct {
		args  []string
		write string
	}{
		"enable by name":  {[]string{"enable", "OFFICE"}, `PATCH ` + wlBase + `/7 {"enabled":true}`},
		"disable by id":   {[]string{"disable", "7"}, `PATCH ` + wlBase + `/7 {"enabled":false}`},
		"rename":          {[]string{"update", "office", "--name", "hq"}, `PATCH ` + wlBase + `/7 {"name":"hq"}`},
		"replace ips":     {[]string{"update", "office", "--ip", "198.51.100.7,2001:db8::1"}, `PATCH ` + wlBase + `/7 {"entries":["198.51.100.7/32","2001:db8::1/128"]}`},
		"assigned only":   {[]string{"update", "office", "--all-keys=false"}, `PATCH ` + wlBase + `/7 {"apply_to_all_api_keys":false}`},
		"all keys":        {[]string{"update", "office", "--all-keys"}, `PATCH ` + wlBase + `/7 {"apply_to_all_api_keys":true}`},
		"set description": {[]string{"update", "office", "--description", "VPN"}, `PATCH ` + wlBase + `/7 {"description":"VPN"}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := newFakeMarly(t, &fakeWhitelist{ID: 7, Name: "office", Entries: []string{"203.0.113.10/32"}})
			if run := runWhitelists(t, "", false, append(tc.args, "--json")...); run.err != nil {
				t.Fatalf("unexpected error: %v\n%s", run.err, run.stdout)
			}
			assertWrites(t, m, tc.write)
		})
	}
}

func TestWhitelistsUpdateRejectsBadInput(t *testing.T) {
	cases := map[string]struct {
		args []string
		exit int
	}{
		"nothing to update": {[]string{"update", "office"}, exitValidation},
		"empty name":        {[]string{"update", "office", "--name", " "}, exitValidation},
		"range":             {[]string{"update", "office", "--ip", "203.0.113.0/24"}, exitValidation},
		"unknown whitelist": {[]string{"enable", "home"}, exitNotFound},
		"ambiguous name":    {[]string{"disable", "dup"}, exitValidation},
		"missing argument":  {[]string{"enable"}, exitValidation},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := newFakeMarly(t, &fakeWhitelist{ID: 7, Name: "office"}, &fakeWhitelist{ID: 8, Name: "dup"}, &fakeWhitelist{ID: 9, Name: "dup"})
			run := runWhitelists(t, "", false, append(tc.args, "--json")...)
			if got := ExitCode(run.err); got != tc.exit {
				t.Fatalf("exit code = %d, want %d\n%s", got, tc.exit, run.stdout)
			}
			assertWrites(t, m)
		})
	}
}

func TestWhitelistsAssignAndUnassign(t *testing.T) {
	m := newFakeMarly(t, &fakeWhitelist{ID: 7, Name: "office", Enabled: true, Entries: []string{"203.0.113.10/32"}})
	run := runWhitelists(t, "", false, "assign", "office", "9f1c****6789", "--json")
	if run.err != nil {
		t.Fatalf("unexpected error: %v\n%s", run.err, run.stdout)
	}
	if !strings.Contains(run.stdout, `"name":"ci"`) || !strings.Contains(run.stdout, "An enabled whitelist limits") {
		t.Errorf("expected the named key and the notice:\n%s", run.stdout)
	}
	run = runWhitelists(t, "", false, "unassign", "7", "ci", "--json")
	if run.err != nil {
		t.Fatalf("unexpected error: %v\n%s", run.err, run.stdout)
	}
	if !strings.Contains(run.stdout, "restricts no API key yet") {
		t.Errorf("expected the no-keys notice:\n%s", run.stdout)
	}
	assertWrites(t, m, "POST "+wlBase+"/7/api-keys/"+testKeyValue, "DELETE "+wlBase+"/7/api-keys/"+testKeyValue)
	assertNoFullKey(t, run)
}

func TestWhitelistsAssignToAccountWideWhitelistExplainsTheEffect(t *testing.T) {
	// Assigning before --all-keys=false keeps the key restricted without a gap.
	m := newFakeMarly(t, &fakeWhitelist{ID: 7, Name: "office", Enabled: true, AllKeys: true, Entries: []string{"203.0.113.10/32"}})
	run := runWhitelists(t, "", false, "assign", "office", "ci", "--json")
	if run.err != nil {
		t.Fatalf("unexpected error: %v\n%s", run.err, run.stdout)
	}
	assertWrites(t, m, "POST "+wlBase+"/7/api-keys/"+testKeyValue)
	if !strings.Contains(run.stdout, "every key stays restricted") || !strings.Contains(run.stdout, "update 7 --all-keys=false") {
		t.Errorf("expected a notice about --all-keys=false:\n%s", run.stdout)
	}
}

func TestWhitelistsHumanOutputPrintsIDsAsSelectors(t *testing.T) {
	newFakeMarly(t, &fakeWhitelist{ID: 1234, Name: "5000", Entries: []string{"203.0.113.10/32"}})
	for _, args := range [][]string{{"list"}, {"disable", "1234"}, {"delete", "1234", "--yes"}} {
		run := runWhitelists(t, "", false, append(args, "--human")...)
		if run.err != nil {
			t.Fatalf("%v: unexpected error: %v\n%s", args, run.err, run.stdout)
		}
		if !strings.Contains(run.stdout, "1234") || !strings.Contains(run.stdout, "5000") || strings.Contains(run.stdout, "1,234") || strings.Contains(run.stdout, "5,000") {
			t.Errorf("%v: expected the ID and name without separators:\n%s", args, run.stdout)
		}
	}
}

func TestWhitelistsAssignArgumentErrors(t *testing.T) {
	m := newFakeMarly(t, &fakeWhitelist{ID: 7, Name: "office"})
	cases := map[string]struct {
		args []string
		exit int
	}{
		"one argument": {[]string{"assign", "office"}, exitValidation},
		"unknown key":  {[]string{"assign", "office", "nope"}, exitNotFound},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run := runWhitelists(t, "", false, append(tc.args, "--json")...)
			if got := ExitCode(run.err); got != tc.exit {
				t.Fatalf("exit code = %d, want %d\n%s", got, tc.exit, run.stdout)
			}
		})
	}
	assertWrites(t, m)
}

func TestWhitelistsDeleteNeedsConfirmation(t *testing.T) {
	cases := map[string]struct {
		terminal bool
		args     []string
	}{
		"default in a terminal":    {true, nil},
		"json in a terminal":       {true, []string{"--json"}},
		"human without a terminal": {false, []string{"--human"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := newFakeMarly(t, &fakeWhitelist{ID: 7, Name: "office"})
			run := runWhitelists(t, "y\n", tc.terminal, append([]string{"delete", "office"}, tc.args...)...)
			if got := ExitCode(run.err); got != exitValidation {
				t.Fatalf("exit code = %d, want %d\n%s", got, exitValidation, run.stdout)
			}
			if !strings.Contains(run.stdout, "--yes") {
				t.Errorf("expected a hint naming --yes:\n%s", run.stdout)
			}
			assertWrites(t, m)
		})
	}
}

func TestWhitelistsDeletePrompt(t *testing.T) {
	cases := map[string]struct {
		answer  string
		deleted bool
	}{
		"yes":   {"y\n", true},
		"no":    {"n\n", false},
		"empty": {"\n", false},
		"eof":   {"", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := newFakeMarly(t, &fakeWhitelist{ID: 7, Name: "office"})
			run := runWhitelists(t, tc.answer, true, "delete", "office", "--human")
			if run.err != nil {
				t.Fatalf("unexpected error: %v\n%s", run.err, run.stdout)
			}
			if !strings.Contains(run.stderr, `Delete IP whitelist "office" (ID 7)?`) {
				t.Errorf("expected a prompt naming the whitelist:\n%s", run.stderr)
			}
			if tc.deleted {
				assertWrites(t, m, "DELETE "+wlBase+"/7")
				if !strings.Contains(run.stdout, "deleted") {
					t.Errorf("expected a deleted status:\n%s", run.stdout)
				}
				return
			}
			assertWrites(t, m)
			if !strings.Contains(run.stdout, "cancelled") {
				t.Errorf("expected a cancelled status:\n%s", run.stdout)
			}
		})
	}
}

func TestWhitelistsDeleteWithYes(t *testing.T) {
	m := newFakeMarly(t, &fakeWhitelist{ID: 7, Name: "office"})
	run := runWhitelists(t, "", false, "delete", "7", "-y", "--json")
	if run.err != nil {
		t.Fatalf("unexpected error: %v\n%s", run.err, run.stdout)
	}
	assertWrites(t, m, "DELETE "+wlBase+"/7")
	for _, want := range []string{`"status":"deleted"`, `"id":7`, `"notice":"Keys that only this whitelist restricted`} {
		if !strings.Contains(run.stdout, want) {
			t.Errorf("expected %s in output:\n%s", want, run.stdout)
		}
	}
}

func TestWhitelistsWritesWithAReadOnlyTokenExitAsAuthErrors(t *testing.T) {
	m := newFakeMarly(t, &fakeWhitelist{ID: 7, Name: "office", Entries: []string{"203.0.113.10/32"}})
	m.fail["PATCH "+wlBase+"/7"] = http.StatusForbidden
	run := runWhitelists(t, "", false, "disable", "office", "--json")
	var rendered *output.RenderedError
	if !errors.As(run.err, &rendered) || rendered.Code != "forbidden" || ExitCode(run.err) != exitAuth {
		t.Fatalf("expected forbidden with exit %d, got %v (exit %d)\n%s", exitAuth, run.err, ExitCode(run.err), run.stdout)
	}
}

func TestWhitelistsNotSignedIn(t *testing.T) {
	t.Setenv("DWELLIR_TOKEN", "")
	t.Setenv("DWELLIR_API_URL", "http://127.0.0.1:1")
	run := runWhitelists(t, "", false, "list", "--json")
	if got := ExitCode(run.err); got != exitAuth {
		t.Fatalf("exit code = %d, want %d\n%s", got, exitAuth, run.stdout)
	}
}

func TestWhitelistsCreateServerErrorSaysCheckBeforeRetrying(t *testing.T) {
	m := newFakeMarly(t)
	m.fail["POST "+wlBase] = http.StatusBadGateway
	run := runWhitelists(t, "", false, "create", "--name", "office", "--ip", "203.0.113.10", "--json")
	var rendered *output.RenderedError
	if !errors.As(run.err, &rendered) || rendered.Code != "api_error" {
		t.Fatalf("expected an api_error, got %v\n%s", run.err, run.stdout)
	}
	if !strings.Contains(run.stdout, "dwellir whitelists list") || !strings.Contains(run.stdout, "create it twice") {
		t.Fatalf("expected a hint to check the list before retrying:\n%s", run.stdout)
	}
}
