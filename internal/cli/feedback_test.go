package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

type sentFeedback struct {
	id    string
	props map[string]interface{}
}

// runFeedbackCmd runs `dwellir feedback <args>` in process against a local
// PostHog that answers with posthogStatus. A zero status runs without a
// PostHog key. Callers clear or set agent markers first.
func runFeedbackCmd(t *testing.T, stdin string, terminal bool, posthogStatus int, args ...string) (cliRun, []sentFeedback) {
	t.Helper()
	return runFeedbackCmdWithAPI(t, "", stdin, terminal, posthogStatus, args...)
}

// runFeedbackCmdWithAPI is runFeedbackCmd with DWELLIR_API_URL set to apiURL.
func runFeedbackCmdWithAPI(t *testing.T, apiURL, stdin string, terminal bool, posthogStatus int, args ...string) (cliRun, []sentFeedback) {
	t.Helper()
	t.Setenv("DWELLIR_CONFIG_DIR", t.TempDir())
	resetOutputFlagsForTest(t)
	setStdoutTerminalForTest(t, true)

	// Flag values and Changed marks survive between in-process runs.
	feedbackSentiment, feedbackMessage, feedbackContext, feedbackArea, feedbackActor = "", "", "", "", ""
	oldProfile, oldAnon, oldQuiet := profile, anonTelemetry, quiet
	profile, anonTelemetry, quiet = "", false, false
	unmark := func(f *pflag.Flag) { f.Changed = false }
	rootCmd.PersistentFlags().VisitAll(unmark)
	feedbackCmd.Flags().VisitAll(unmark)
	t.Cleanup(func() { profile, anonTelemetry, quiet = oldProfile, oldAnon, oldQuiet })

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
	t.Setenv("DWELLIR_API_URL", apiURL)
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

type feedbackErrorBody struct {
	Code, Message, Help string
}

func parseFeedbackError(t *testing.T, stdout string) feedbackErrorBody {
	t.Helper()
	var resp struct {
		OK    bool              `json:"ok"`
		Error feedbackErrorBody `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil || resp.OK {
		t.Fatalf("want a JSON error, got: %s", stdout)
	}
	return resp.Error
}

// Agents retry from the error alone, so every rejection says that nothing was
// sent, gives the rule for each flag that needs a new value, and ends with a
// retry line that keeps the valid flags.
func TestFeedbackRejectsBadInputWithARetryLine(t *testing.T) {
	sentiment := []string{"--sentiment", "positive"}
	message := []string{"--message", "Works well for me."}
	context := []string{"--context", "Listing endpoints."}
	join := func(parts ...[]string) []string { return slices.Concat(parts...) }
	tests := []struct {
		name, wantMessage, wantHelp, wantRetry string
		args                                   []string
	}{
		{
			name: "missing flag", args: join(sentiment, message),
			wantMessage: "Missing required flags: --context. Nothing was sent.",
			wantHelp:    "--context: what you were trying to do, 10 to 2000 characters.",
			wantRetry:   `dwellir feedback --json --sentiment positive --message 'Works well for me.' --context '<what you were trying to do>'`,
		},
		{
			name: "bad sentiment", args: join([]string{"--sentiment", "meh"}, message, context),
			wantMessage: `--sentiment "meh" is not a valid value. Nothing was sent.`,
			wantHelp:    "--sentiment: use one of positive, negative, neutral.",
			wantRetry:   `dwellir feedback --json --sentiment '<positive|negative|neutral>' --message 'Works well for me.' --context 'Listing endpoints.'`,
		},
		{
			name: "bad area", args: join(sentiment, message, context, []string{"--area", "nft"}),
			wantMessage: `--area "nft" is not a valid value.`, wantHelp: "--area: use one of addons, auth,",
			wantRetry: `dwellir feedback --json --sentiment positive --message 'Works well for me.' --context 'Listing endpoints.'`,
		},
		{
			name: "bad actor", args: join(sentiment, message, context, []string{"--actor", "bot"}),
			wantMessage: `--actor "bot" is not a valid value.`, wantHelp: "--actor: use one of human, agent.",
			wantRetry: `--context 'Listing endpoints.'`,
		},
		{
			name: "short message", args: join(sentiment, []string{"--message", "ok"}, context),
			wantMessage: "--message has 2 characters; it needs 10 to 2000. Nothing was sent.",
			wantHelp:    "Quote the value",
			wantRetry:   `--sentiment positive --message '<your feedback>' --context 'Listing endpoints.'`,
		},
		{
			name: "long context", args: join(sentiment, message, []string{"--context", strings.Repeat("x", 2001)}),
			wantMessage: "--context has 2001 characters; it needs 10 to 2000.",
			wantRetry:   `--context '<what you were trying to do>'`,
		},
		{
			name: "placeholder sent unchanged", args: join(sentiment, message, []string{"--context", "<what you were trying to do>"}),
			wantMessage: "--context is still a placeholder (<what you were trying to do>); replace it with real text.",
			wantRetry:   `--context '<what you were trying to do>'`,
		},
		{
			name: "several bad flags", args: join([]string{"--sentiment", "good"}, []string{"--message", "it's slow"}),
			wantMessage: `--sentiment "good" is not a valid value. --message has 9 characters; it needs 10 to 2000. Missing required flags: --context. Nothing was sent.`,
			wantHelp:    "--context: what you were trying to do, 10 to 2000 characters.",
			wantRetry:   `dwellir feedback --json --sentiment '<positive|negative|neutral>' --message '<your feedback>' --context '<what you were trying to do>'`,
		},
		{
			// The shell split an unquoted message into words, so the first
			// word must not be kept as the message.
			name: "unquoted message", args: join(sentiment, []string{"--message", "Works", "well", "for", "me."}, context),
			wantMessage: "dwellir feedback takes no positional arguments (got 3: well for me.). --message has 5 characters",
			wantHelp:    "Quote each --message and --context value",
			wantRetry:   `dwellir feedback --json --sentiment positive --message '<your feedback>' --context '<what you were trying to do>'`,
		},
		{
			name: "unknown flag", args: join([]string{"--sentimant", "positive"}, message, context),
			wantMessage: "unknown flag: --sentimant.",
			wantHelp:    "Run 'dwellir feedback --help' for every flag.",
			wantRetry:   `--sentiment '<positive|negative|neutral>'`,
		},
		{
			name: "flag without a value", args: join(message, context, []string{"--sentiment"}),
			wantMessage: "flag needs an argument: --sentiment.",
			wantRetry:   `--sentiment '<positive|negative|neutral>' --message 'Works well for me.' --context 'Listing endpoints.'`,
		},
		{
			// Dropping --anon-telemetry from the retry would identify the sender.
			name: "global flags kept", args: join([]string{"--anon-telemetry", "--profile", "work"}, sentiment, message),
			wantMessage: "Missing required flags: --context.",
			wantRetry:   `dwellir feedback --profile=work --json --anon-telemetry --sentiment positive`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearAgentMarkers(t)
			// A terminal with JSON output must not prompt either.
			run, sent := runFeedbackCmd(t, "", true, http.StatusOK, append([]string{"--json"}, tt.args...)...)
			if len(sent) != 0 || ExitCode(run.err) != exitValidation {
				t.Fatalf("sent %d, exit %d; want nothing sent and exit %d\n%s", len(sent), ExitCode(run.err), exitValidation, run.stdout)
			}
			e := parseFeedbackError(t, run.stdout)
			if e.Code != "validation_error" || !strings.Contains(e.Message, tt.wantMessage) ||
				!strings.Contains(e.Help, tt.wantHelp) || !strings.Contains(e.Help, "Retry: dwellir feedback") ||
				!strings.Contains(e.Help, tt.wantRetry) {
				t.Fatalf("error = %+v\nwant message %q, help %q, retry %q", e, tt.wantMessage, tt.wantHelp, tt.wantRetry)
			}
		})
	}
}

// The retry line must run once its placeholders are filled in: the shell
// splits it, and the same flags then send the feedback.
func TestFeedbackRetryLineRunsWhenFilledIn(t *testing.T) {
	clearAgentMarkers(t)
	run, _ := runFeedbackCmd(t, "", false, http.StatusOK, "--json", "--sentiment", "positive", "--message", "It's quick & simple.")
	help := parseFeedbackError(t, run.stdout).Help
	i := strings.Index(help, "Retry: dwellir feedback ")
	if i < 0 {
		t.Fatalf("no retry line in %q", help)
	}
	retry := strings.SplitN(help[i+len("Retry: dwellir feedback "):], "\n", 2)[0]
	retry = strings.Replace(retry, "<what you were trying to do>", "Checking the retry line.", 1)

	out, err := exec.Command("sh", "-c", `eval "set -- $1"; for a in "$@"; do printf '%s\0' "$a"; done`, "sh", retry).Output()
	if err != nil {
		t.Fatalf("shell could not parse %q: %v", retry, err)
	}
	args := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")

	run, sent := runFeedbackCmd(t, "", false, http.StatusOK, args...)
	if run.err != nil || len(sent) != 1 || sent[0].props["message"] != "It's quick & simple." ||
		sent[0].props["context"] != "Checking the retry line." {
		t.Fatalf("retry %q: err %v, sent %v", args, run.err, sent)
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
	if !strings.Contains(run.stderr, "sentiment must be one of") || !strings.Contains(run.stderr, "feedback has 5 characters") {
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
	retry := `Retry: dwellir feedback --json --sentiment neutral --message 'Works well for me.' --context 'Listing endpoints.'`
	tests := []struct {
		name, apiURL string
		status       int
		wantCode     string
		wantHelp     string
	}{
		{"no analytics key in this build", "", 0, "feedback_unavailable", "Install a release build"},
		{"no analytics key with the production API set", "https://marly.dwellir.com", 0, "feedback_unavailable", "Install a release build"},
		{"staging API", "https://marly-staging.dwellir.com", http.StatusOK, "feedback_unavailable", "Unset DWELLIR_API_URL"},
		{"PostHog rejects the event", "", http.StatusInternalServerError, "network_error", "Check the network connection"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearAgentMarkers(t)
			run, sent := runFeedbackCmdWithAPI(t, tt.apiURL, "", false, tt.status, args...)
			e := parseFeedbackError(t, run.stdout)
			if ExitCode(run.err) != exitError || e.Code != tt.wantCode || !strings.Contains(e.Help, tt.wantHelp) ||
				!strings.Contains(e.Help, retry) {
				t.Fatalf("exit %d, error %+v; want code %s with help %q and %q", ExitCode(run.err), e, tt.wantCode, tt.wantHelp, retry)
			}
			if tt.wantCode == "feedback_unavailable" && len(sent) != 0 {
				t.Fatalf("sent %d events from a build that cannot send", len(sent))
			}
		})
	}
}
