package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestFeedbackCommandRegistrationFollowsTheGate(t *testing.T) {
	registered := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "feedback" {
			registered = true
		}
	}
	if registered != feedbackEnabled() {
		t.Fatalf("feedback command registered = %v, but the gate is %v", registered, feedbackEnabled())
	}
}

type sentFeedback struct {
	id    string
	props map[string]interface{}
}

// runFeedbackCmd runs `dwellir feedback <args>` in process against a local
// PostHog that answers with posthogStatus. A zero status runs without a
// PostHog key. Callers clear or set agent markers first. The command is
// registered for the run, so the test covers it in both gate states.
func runFeedbackCmd(t *testing.T, stdin string, terminal bool, posthogStatus int, args ...string) (cliRun, []sentFeedback) {
	t.Helper()
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	resetOutputFlagsForTest(t)
	setStdoutTerminalForTest(t, true)

	feedbackSentiment, feedbackMessage, feedbackContext, feedbackArea, feedbackActor = "", "", "", "", ""

	if !feedbackEnabled() {
		rootCmd.AddCommand(feedbackCmd)
		t.Cleanup(func() { rootCmd.RemoveCommand(feedbackCmd) })
	}

	var sent []sentFeedback
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			UUID       string                 `json:"uuid"`
			Properties map[string]interface{} `json:"properties"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		sent = append(sent, sentFeedback{id: body.UUID, props: body.Properties})
		w.WriteHeader(posthogStatus)
	}))
	t.Cleanup(server.Close)
	key := "test-key"
	if posthogStatus == 0 {
		key = ""
	}
	t.Setenv("DWELLIR_API_URL", "")
	t.Setenv("DWELLIR_POSTHOG_KEY", key)
	t.Setenv("DWELLIR_POSTHOG_HOST", server.URL)

	oldStdinIsTerminal, oldArgs, oldTelemetry := stdinIsTerminal, os.Args, telemetryClient
	stdinIsTerminal = func() bool { return terminal }
	telemetryClient = &fakeTelemetry{}
	args = append([]string{"feedback"}, args...)
	os.Args = append([]string{"dwellir"}, args...)

	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetIn(strings.NewReader(stdin))
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		stdinIsTerminal, os.Args, telemetryClient = oldStdinIsTerminal, oldArgs, oldTelemetry
		rootCmd.SetIn(nil)
		rootCmd.SetArgs(nil)
	})
	err := Execute()
	return cliRun{stdout: stdout.String(), stderr: stderr.String(), err: err}, sent
}

func TestFeedbackSendsFlagsAndMasksKeys(t *testing.T) {
	clearAgentMarkers(t)
	t.Setenv("CLAUDECODE", "1")
	run, sent := runFeedbackCmd(t, "", false, http.StatusOK, "--json",
		"--sentiment", "negative", "--area", "docs",
		"--message", "My key "+testKeyValue+" fails\x07 on\tws",
		"--context", "Connecting a trading bot.")
	if run.err != nil {
		t.Fatalf("err = %v\nstdout: %s", run.err, run.stdout)
	}
	if len(sent) != 1 {
		t.Fatalf("sent %d feedback events, want 1", len(sent))
	}
	props := sent[0].props
	message, _ := props["message"].(string)
	if strings.Contains(message, testKeyValue) || !strings.HasSuffix(message, " fails on ws") {
		t.Fatalf("message was not cleaned: %q", props["message"])
	}
	for key, want := range map[string]interface{}{
		"sentiment": "negative", "area": "docs", "context": "Connecting a trading bot.",
		"actor": "agent", "channel": "cli", "feedback_id": sent[0].id,
	} {
		if props[key] != want {
			t.Fatalf("props[%q] = %v, want %v", key, props[key], want)
		}
	}

	var resp struct {
		OK   bool              `json:"ok"`
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal([]byte(run.stdout), &resp); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, run.stdout)
	}
	if !resp.OK || resp.Data["id"] != sent[0].id {
		t.Fatalf("response = %+v, want ok with id %s", resp, sent[0].id)
	}
}

func TestFeedbackRejectsBadInputBeforeSending(t *testing.T) {
	ok := []string{"--sentiment", "positive", "--message", "Works well for me.", "--context", "Listing endpoints."}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing flags", []string{"--sentiment", "positive"}, "Missing required flags: --message, --context."},
		{"bad sentiment", []string{"--sentiment", "meh"}, "--sentiment must be one of"},
		{"bad area", append([]string{"--area", "nft"}, ok...), "--area must be one of"},
		{"bad actor", append([]string{"--actor", "bot"}, ok...), "--actor must be one of"},
		{"short message", []string{"--sentiment", "positive", "--message", "ok", "--context", "Listing endpoints."}, "--message must be 10 to 2000"},
		{"long context", []string{"--sentiment", "positive", "--message", "Works well for me.", "--context", strings.Repeat("x", 2001)}, "--context must be 10 to 2000"},
		{"arguments", append([]string{"extra"}, ok...), "Unexpected arguments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearAgentMarkers(t)
			// A terminal with JSON output must not prompt either.
			run, sent := runFeedbackCmd(t, "", true, http.StatusOK, append([]string{"--json"}, tt.args...)...)
			if len(sent) != 0 {
				t.Fatalf("sent feedback for invalid input")
			}
			if ExitCode(run.err) != exitValidation || !strings.Contains(run.stdout, tt.want) {
				t.Fatalf("exit %d, stdout %s; want exit %d with %q", ExitCode(run.err), run.stdout, exitValidation, tt.want)
			}
		})
	}
}

func TestFeedbackPromptsForMissingFieldsInATerminal(t *testing.T) {
	stdin := strings.Join([]string{
		"meh",                    // invalid sentiment, asked again
		"positive",               //
		"",                       // area skipped
		"short",                  // too short, asked again
		"Docs search is quick.",  //
		"Finding the Base docs.", //
	}, "\n") + "\n"
	clearAgentMarkers(t)
	run, sent := runFeedbackCmd(t, stdin, true, http.StatusOK, "--human")
	if run.err != nil {
		t.Fatalf("err = %v\nstderr: %s", run.err, run.stderr)
	}
	if len(sent) != 1 {
		t.Fatalf("sent %d feedback events, want 1", len(sent))
	}
	props := sent[0].props
	if props["sentiment"] != "positive" || props["message"] != "Docs search is quick." ||
		props["context"] != "Finding the Base docs." || props["actor"] != "human" {
		t.Fatalf("props = %v", props)
	}
	if _, ok := props["area"]; ok {
		t.Fatalf("skipped area was sent: %v", props["area"])
	}
	if !strings.Contains(run.stderr, "sentiment must be one of") || !strings.Contains(run.stderr, "feedback must be 10 to 2000") {
		t.Fatalf("invalid answers were not explained:\n%s", run.stderr)
	}
	if !strings.Contains(run.stdout, "Thanks.") {
		t.Fatalf("human output has no thanks:\n%s", run.stdout)
	}
}

// AGENTS.md allows prompts only in explicit --human mode, so an agent that runs
// the CLI under a pseudo-terminal gets an error instead of a hung prompt.
func TestFeedbackDoesNotPromptWithoutHumanFlag(t *testing.T) {
	clearAgentMarkers(t)
	run, sent := runFeedbackCmd(t, "positive\n", true, http.StatusOK)
	if len(sent) != 0 || ExitCode(run.err) != exitValidation || strings.Contains(run.stderr, "How do you feel") {
		t.Fatalf("sent %d, exit %d, output:\n%s%s", len(sent), ExitCode(run.err), run.stdout, run.stderr)
	}
}

func TestFeedbackPromptStopsAtEndOfInput(t *testing.T) {
	clearAgentMarkers(t)
	run, sent := runFeedbackCmd(t, "positive\n", true, http.StatusOK, "--human")
	if len(sent) != 0 || ExitCode(run.err) != exitValidation || !strings.Contains(run.stderr+run.stdout, "input ended") {
		t.Fatalf("sent %d, exit %d, output:\n%s%s", len(sent), ExitCode(run.err), run.stdout, run.stderr)
	}
}

func TestFeedbackSendErrors(t *testing.T) {
	args := []string{"--json", "--sentiment", "neutral", "--message", "Works well for me.", "--context", "Listing endpoints."}
	tests := []struct {
		name     string
		status   int
		wantCode string
	}{
		{"no channel in this build", 0, "feedback_unavailable"},
		{"PostHog rejects the event", http.StatusInternalServerError, "network_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearAgentMarkers(t)
			run, _ := runFeedbackCmd(t, "", false, tt.status, args...)
			if ExitCode(run.err) != exitError || !strings.Contains(run.stdout, `"code":"`+tt.wantCode+`"`) {
				t.Fatalf("exit %d, stdout %s; want code %s", ExitCode(run.err), run.stdout, tt.wantCode)
			}
		})
	}
}
