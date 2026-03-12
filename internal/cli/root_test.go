package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestResolvedOutputFormat_DefaultHuman(t *testing.T) {
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	clearAgentMarkers(t)
	resetOutputFlagsForTest(t)

	if got := resolvedOutputFormat(); got != "human" {
		t.Fatalf("resolvedOutputFormat() = %q, want %q", got, "human")
	}
}

func TestResolvedOutputFormat_DefaultsHumanForAgentEnvWhenStdoutIsTTY(t *testing.T) {
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	setStdoutTerminalForTest(t, true)
	t.Setenv("CODEX_CI", "1")

	if got := resolvedOutputFormat(); got != "human" {
		t.Fatalf("resolvedOutputFormat() = %q, want %q", got, "human")
	}
}

func TestResolvedOutputFormat_DefaultsHumanForNonTerminalWithoutAgentMarkers(t *testing.T) {
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	setStdoutTerminalForTest(t, false)

	if got := resolvedOutputFormat(); got != "human" {
		t.Fatalf("resolvedOutputFormat() = %q, want %q", got, "human")
	}
}

func TestResolvedOutputFormat_DwellirAgentOverrideForcesAutoStructuredOutput(t *testing.T) {
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	setStdoutTerminalForTest(t, false)
	t.Setenv("DWELLIR_AGENT", "1")

	if got := resolvedOutputFormat(); got != "toon" {
		t.Fatalf("resolvedOutputFormat() = %q, want %q", got, "toon")
	}
}

func TestResolvedOutputFormat_DwellirAgentOverrideCanDisableAgentDetection(t *testing.T) {
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	setStdoutTerminalForTest(t, false)
	t.Setenv("CODEX_CI", "1")
	t.Setenv("DWELLIR_AGENT", "0")

	if got := resolvedOutputFormat(); got != "human" {
		t.Fatalf("resolvedOutputFormat() = %q, want %q", got, "human")
	}
}

func TestResolvedOutputFormat_HumanFlagOverridesAgentDefault(t *testing.T) {
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	t.Setenv("CODEX_CI", "1")
	humanOutput = true

	if got := resolvedOutputFormat(); got != "human" {
		t.Fatalf("resolvedOutputFormat() = %q, want %q", got, "human")
	}
}

func TestResolvedOutputFormat_JSONFlagOverridesHumanConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DWELLIR_CONFIG_DIR", dir)
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"output":"human","default_profile":"default"}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	jsonOutput = true

	if got := resolvedOutputFormat(); got != "json" {
		t.Fatalf("resolvedOutputFormat() = %q, want %q", got, "json")
	}
}

func TestResolvedOutputFormat_TOONFlagOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DWELLIR_CONFIG_DIR", dir)
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"output":"human","default_profile":"default"}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	toonOutput = true

	if got := resolvedOutputFormat(); got != "toon" {
		t.Fatalf("resolvedOutputFormat() = %q, want %q", got, "toon")
	}
}

func TestResolvedOutputFormat_ConfigOverridesAgentDefaultWhenConfigExists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DWELLIR_CONFIG_DIR", dir)
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	t.Setenv("CODEX_CI", "1")
	setStdoutTerminalForTest(t, false)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"output":"human","default_profile":"default"}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if got := resolvedOutputFormat(); got != "human" {
		t.Fatalf("resolvedOutputFormat() = %q, want %q", got, "human")
	}
}

func TestResolvedOutputFormat_AgentDefaultUsesTOONWhenConfigExistsWithoutExplicitOutput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DWELLIR_CONFIG_DIR", dir)
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	setStdoutTerminalForTest(t, false)
	t.Setenv("CODEX_CI", "1")
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"default_profile":"work"}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if got := resolvedOutputFormat(); got != "toon" {
		t.Fatalf("resolvedOutputFormat() = %q, want %q", got, "toon")
	}
}

func TestExplicitOutputFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "empty",
			args: nil,
			want: "",
		},
		{
			name: "json",
			args: []string{"version", "--json"},
			want: "json",
		},
		{
			name: "toon",
			args: []string{"version", "--toon"},
			want: "toon",
		},
		{
			name: "last wins",
			args: []string{"version", "--json", "--human", "--toon"},
			want: "toon",
		},
		{
			name: "unknown flags ignored",
			args: []string{"--nope"},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := explicitOutputFromArgs(tc.args); got != tc.want {
				t.Fatalf("explicitOutputFromArgs(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

func resetOutputFlagsForTest(t *testing.T) {
	t.Helper()
	oldJSON := jsonOutput
	oldHuman := humanOutput
	oldTOON := toonOutput
	t.Cleanup(func() {
		jsonOutput = oldJSON
		humanOutput = oldHuman
		toonOutput = oldTOON
	})
	jsonOutput = false
	humanOutput = false
	toonOutput = false
	setStdoutTerminalForTest(t, true)
}

func setStdoutTerminalForTest(t *testing.T, value bool) {
	t.Helper()
	old := stdoutIsTerminal
	t.Cleanup(func() {
		stdoutIsTerminal = old
	})
	stdoutIsTerminal = func() bool { return value }
}

func clearAgentMarkers(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"CODEX_CI",
		"CODEX_THREAD_ID",
		"CLAUDECODE",
		"CLAUDE_CODE_ENTRYPOINT",
		"OPENCODE",
		"CURSOR_AGENT",
		"DWELLIR_AGENT",
	} {
		t.Setenv(key, "")
	}
}

type telemetryCall struct {
	command string
	extra   map[string]interface{}
}

type fakeTelemetry struct {
	trackCalls []telemetryCall
}

func (f *fakeTelemetry) Init(string, string, string, string, bool) {}

func (f *fakeTelemetry) Identify(map[string]interface{}) {}

func (f *fakeTelemetry) TrackCommand(command string, extra map[string]interface{}) {
	cp := map[string]interface{}{}
	for k, v := range extra {
		cp[k] = v
	}
	f.trackCalls = append(f.trackCalls, telemetryCall{
		command: command,
		extra:   cp,
	})
}

func (f *fakeTelemetry) Close() {}

func TestExecute_TracksSuccess(t *testing.T) {
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	setStdoutTerminalForTest(t, true)

	oldArgs := os.Args
	oldTelemetry := telemetryClient
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"version", "--human"})
	fake := &fakeTelemetry{}
	telemetryClient = fake
	os.Args = []string{"dwellir", "version", "--human"}
	t.Cleanup(func() {
		os.Args = oldArgs
		telemetryClient = oldTelemetry
		rootCmd.SetArgs(nil)
	})

	if err := Execute(); err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if len(fake.trackCalls) != 1 {
		t.Fatalf("expected 1 telemetry event, got %d", len(fake.trackCalls))
	}

	call := fake.trackCalls[0]
	if call.command != "version" {
		t.Fatalf("command = %q, want %q", call.command, "version")
	}
	if ok, _ := call.extra["success"].(bool); !ok {
		t.Fatalf("expected success=true, got %#v", call.extra["success"])
	}
	if format, _ := call.extra["output_format"].(string); format != "human" {
		t.Fatalf("output_format = %q, want %q", format, "human")
	}
}

func TestExecute_TracksUnknownCommandError(t *testing.T) {
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	setStdoutTerminalForTest(t, true)

	oldArgs := os.Args
	oldTelemetry := telemetryClient
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"get", "--human"})
	fake := &fakeTelemetry{}
	telemetryClient = fake
	os.Args = []string{"dwellir", "get", "--human"}
	t.Cleanup(func() {
		os.Args = oldArgs
		telemetryClient = oldTelemetry
		rootCmd.SetArgs(nil)
	})

	if err := Execute(); err == nil {
		t.Fatal("Execute() expected error, got nil")
	}

	if len(fake.trackCalls) != 1 {
		t.Fatalf("expected 1 telemetry event, got %d", len(fake.trackCalls))
	}
	call := fake.trackCalls[0]
	if ok, _ := call.extra["success"].(bool); ok {
		t.Fatalf("expected success=false, got %#v", call.extra["success"])
	}
	if code, _ := call.extra["error_code"].(string); code != "validation_error" {
		t.Fatalf("error_code = %q, want %q", code, "validation_error")
	}
	if unknown, _ := call.extra["unknown_command"].(string); unknown != "get" {
		t.Fatalf("unknown_command = %q, want %q", unknown, "get")
	}
}

func TestExecute_TracksUnknownCommandError_WithProfileFlagValue(t *testing.T) {
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	resetOutputFlagsForTest(t)
	clearAgentMarkers(t)
	setStdoutTerminalForTest(t, true)

	oldArgs := os.Args
	oldTelemetry := telemetryClient
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"--profile", "prod", "get", "--human"})
	fake := &fakeTelemetry{}
	telemetryClient = fake
	os.Args = []string{"dwellir", "--profile", "prod", "get", "--human"}
	t.Cleanup(func() {
		os.Args = oldArgs
		telemetryClient = oldTelemetry
		rootCmd.SetArgs(nil)
	})

	if err := Execute(); err == nil {
		t.Fatal("Execute() expected error, got nil")
	}

	if len(fake.trackCalls) != 1 {
		t.Fatalf("expected 1 telemetry event, got %d", len(fake.trackCalls))
	}
	call := fake.trackCalls[0]
	if call.command != "get" {
		t.Fatalf("command = %q, want %q", call.command, "get")
	}
	if unknown, _ := call.extra["unknown_command"].(string); unknown != "get" {
		t.Fatalf("unknown_command = %q, want %q", unknown, "get")
	}
}
