package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/dwellir-public/cli/internal/api"
	"github.com/dwellir-public/cli/internal/output"
	"github.com/dwellir-public/cli/internal/telemetry"
)

const (
	feedbackMinLen = 10
	feedbackMaxLen = 2000
)

var (
	feedbackSentiments = []string{"positive", "negative", "neutral"}
	feedbackAreas      = []string{
		"addons", "auth", "billing", "cli", "dashboard", "docs", "endpoints",
		"keys", "logs", "rpc", "status", "usage", "whitelists", "other",
	}
	feedbackActors = []string{"human", "agent"}
)

var (
	feedbackSentiment string
	feedbackMessage   string
	feedbackContext   string
	feedbackArea      string
	feedbackActor     string
)

const feedbackNotSupport = "This is not a support channel, and the team cannot reply here. " +
	"For help with your account, contact Dwellir support through https://dashboard.dwellir.com."

var feedbackCmd = &cobra.Command{
	Use:   "feedback",
	Short: "Send feedback about Dwellir to the team",
	Long: `Send feedback about any part of Dwellir (the CLI, RPC endpoints, docs,
the dashboard, billing and more) from your terminal.

` + feedbackNotSupport + `

With --human in a terminal, the command asks for each required field you
did not pass. Scripts and agents pass every required flag.

Areas: ` + strings.Join(feedbackAreas, ", ") + `

Example:
  dwellir feedback --human
  dwellir feedback --sentiment negative --area docs \
    --message "The Hyperliquid page does not list the WebSocket URL." \
    --context "Setting up an order book stream for a trading bot."`,
	Args: feedbackNoArgs,
	RunE: withRenderedExitCodes(runFeedback),
}

// feedbackNoArgs rejects positional arguments. They usually mean an unquoted
// --message or --context value that the shell split into words, so the retry
// asks for both values again.
func feedbackNoArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return feedbackInputError(
		fmt.Sprintf("dwellir feedback takes no positional arguments (got %d: %s).", len(args), strings.Join(args, " ")),
		[]string{"--message", "--context"},
		"Quote each --message and --context value so the shell passes it as one argument.")
}

// feedbackFlagError handles flag parse errors, such as a misspelled flag or a
// flag without a value, with the same guidance as a bad value. Parsing stops
// at the bad flag, so the arguments are parsed again with unknown flags and
// malformed boolean values skipped. That recovers every valid flag for the
// retry line, including --anon-telemetry and the output format.
func feedbackFlagError(cmd *cobra.Command, err error) error {
	lenient := pflag.NewFlagSet("feedback", pflag.ContinueOnError)
	lenient.SetOutput(io.Discard)
	lenient.ParseErrorsAllowlist.UnknownFlags = true
	lenient.AddFlagSet(cmd.Flags())
	var lenientErr error
	if i := slices.Index(os.Args, cmd.Name()); i >= 0 {
		lenientErr = lenient.Parse(withoutMalformedBools(lenient, os.Args[i+1:]))
	}

	// A flag at the end of the line can lack its value. Ask for it again.
	var replace []string
	for _, e := range []error{err, lenientErr} {
		if e == nil {
			continue
		}
		name, ok := strings.CutPrefix(e.Error(), "flag needs an argument: ")
		if ok && feedbackFlagUsage[name] != "" && !slices.Contains(replace, name) {
			replace = append(replace, name)
		}
	}
	return feedbackInputError(err.Error()+".", replace, "Run 'dwellir feedback --help' for every flag.")
}

// withoutMalformedBools drops boolean flags whose value does not parse, such
// as --quiet=no, which would otherwise stop the lenient parse again.
func withoutMalformedBools(fs *pflag.FlagSet, args []string) []string {
	kept := make([]string, 0, len(args))
	for _, arg := range args {
		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		var flag *pflag.Flag
		switch {
		case !hasValue || !strings.HasPrefix(arg, "-"):
		case strings.HasPrefix(arg, "--"):
			flag = fs.Lookup(name)
		case len(name) == 1:
			flag = fs.ShorthandLookup(name)
		}
		if flag != nil && flag.Value.Type() == "bool" {
			if _, err := strconv.ParseBool(value); err != nil {
				continue
			}
		}
		kept = append(kept, arg)
	}
	return kept
}

func runFeedback(cmd *cobra.Command, args []string) error {
	f := getFormatter()
	// Check the flags that were passed before asking for the rest, so a bad
	// flag fails at once instead of after the prompts.
	canPrompt := humanOutput && isHumanOutput() && stdinIsTerminal()
	missing := missingFeedbackFlags()
	if len(invalidFeedbackFlags()) > 0 || (len(missing) > 0 && !canPrompt) {
		var notes []string
		if len(missing) > 0 {
			notes = append(notes, "A person at a terminal can run 'dwellir feedback --human' to be asked for each field instead.")
		}
		return feedbackInputError("", nil, notes...)
	}
	if len(missing) > 0 {
		if err := promptFeedback(cmd.InOrStdin(), cmd.ErrOrStderr()); err != nil {
			return f.Error("validation_error", "Feedback was not sent: "+err.Error()+".",
				"Run 'dwellir feedback --human' again and answer every question, or pass the flags instead.\n"+
					"Retry: "+feedbackRetryCommand(nil))
		}
	}

	actor := feedbackActor
	if actor == "" {
		actor = "human"
		if isAgentEnvironment() {
			actor = "agent"
		}
	}
	id, err := uuid.NewV7()
	if err != nil {
		return f.Error("error", "Feedback was not sent: could not create a feedback ID: "+err.Error()+".",
			"Retry: "+feedbackRetryCommand(nil))
	}
	props := map[string]interface{}{
		"feedback_id":   id.String(),
		"sentiment":     feedbackSentiment,
		"message":       cleanFeedbackText(feedbackMessage),
		"context":       cleanFeedbackText(feedbackContext),
		"actor":         actor,
		"channel":       "cli",
		"agent_env":     isAgentEnvironment(),
		"agent_markers": presentAgentMarkers(),
		"is_ci":         os.Getenv("CI") != "",
	}
	if feedbackArea != "" {
		props["area"] = feedbackArea
	}
	if program := strings.TrimSpace(os.Getenv("TERM_PROGRAM")); program != "" {
		props["terminal_program"] = program
	}

	if err := telemetry.SendFeedback(cmd.Context(), id.String(), props); err != nil {
		if errors.Is(err, telemetry.ErrFeedbackUnavailable) {
			return f.Error("feedback_unavailable", "This CLI build cannot send feedback. Nothing was sent.",
				feedbackUnavailableHelp()+"\nRetry: "+feedbackRetryCommand(nil))
		}
		return f.Error("network_error", "Feedback was not sent: "+err.Error()+".",
			"Check the network connection, then retry. "+
				"If the request timed out, the feedback may have arrived; a duplicate is harmless.\n"+
				"Retry: "+feedbackRetryCommand(nil))
	}

	result := map[string]string{"id": id.String(), "sentiment": feedbackSentiment, "actor": actor}
	if feedbackArea != "" {
		result["area"] = feedbackArea
	}
	return f.Success("feedback", output.Notice{
		Data: result,
		Text: "Thanks. Your feedback was sent. " + feedbackNotSupport,
	})
}

// feedbackFlagUsage describes each flag in the words an error repeats, so a
// caller can fix the flag without opening --help.
var feedbackFlagUsage = map[string]string{
	"--sentiment": "how you feel about it; one of " + strings.Join(feedbackSentiments, ", "),
	"--message":   fmt.Sprintf("your feedback, %d to %d characters", feedbackMinLen, feedbackMaxLen),
	"--context":   fmt.Sprintf("what you were trying to do, %d to %d characters", feedbackMinLen, feedbackMaxLen),
	"--area":      "optional; one of " + strings.Join(feedbackAreas, ", "),
	"--actor":     "optional; human or agent",
	"--profile":   "the auth profile to use; run 'dwellir profiles list' to see them",
}

type feedbackFlagProblem struct {
	flag, problem, fix string
}

// invalidFeedbackFlags reports every flag with a bad value, so one retry can
// fix them all.
func invalidFeedbackFlags() []feedbackFlagProblem {
	var bad []feedbackFlagProblem
	for _, c := range []struct {
		flag, value string
		allowed     []string
	}{
		{"--sentiment", feedbackSentiment, feedbackSentiments},
		{"--area", feedbackArea, feedbackAreas},
		{"--actor", feedbackActor, feedbackActors},
	} {
		if c.value != "" && !slices.Contains(c.allowed, c.value) {
			bad = append(bad, feedbackFlagProblem{c.flag,
				fmt.Sprintf("%s %q is not a valid value.", c.flag, c.value),
				c.flag + ": use one of " + strings.Join(c.allowed, ", ") + "."})
		}
	}
	for _, c := range []struct{ flag, value string }{
		{"--message", feedbackMessage},
		{"--context", feedbackContext},
	} {
		if c.value == "" {
			continue
		}
		if err := checkFeedbackText(c.flag, c.value); err != nil {
			bad = append(bad, feedbackFlagProblem{c.flag, err.Error() + ".",
				c.flag + ": " + feedbackFlagUsage[c.flag] + ". Quote the value so the shell passes it as one argument."})
		}
	}
	return bad
}

// feedbackInputError renders a validation error that says nothing was sent.
// The help gives the rule for every flag that needs a new value: flags with a
// bad value, missing required flags, and the flags in replace. It ends with a
// Retry line, so an agent can fix every flag in one retry.
func feedbackInputError(problem string, replace []string, notes ...string) error {
	var problems, rules []string
	if problem != "" {
		problems = append(problems, problem)
	}
	bad := invalidFeedbackFlags()
	for _, b := range bad {
		problems = append(problems, b.problem)
		replace = append(replace, b.flag)
	}
	missing := missingFeedbackFlags()
	if len(missing) > 0 {
		problems = append(problems, "Missing required flags: "+strings.Join(missing, ", ")+".")
	}
	for _, flag := range append([]string{"--profile"}, feedbackFlagOrder...) {
		if i := slices.IndexFunc(bad, func(b feedbackFlagProblem) bool { return b.flag == flag }); i >= 0 {
			rules = append(rules, bad[i].fix)
		} else if slices.Contains(missing, flag) || slices.Contains(replace, flag) {
			rules = append(rules, flag+": "+feedbackFlagUsage[flag]+".")
		}
	}
	retry := feedbackRetryCommand(replace)
	help := append(rules, notes...)
	help = append(help, "Retry: "+retry)
	if strings.Contains(retry, "'<") {
		help = append(help, "Replace each <...> placeholder inside its single quotes.")
	}
	return getFormatter().Error("validation_error",
		strings.Join(problems, " ")+" Nothing was sent.", strings.Join(help, "\n"))
}

var feedbackFlagOrder = []string{"--sentiment", "--message", "--context", "--area", "--actor"}

// feedbackRetryCommand returns a command line that keeps the global flags and
// every valid feedback flag the caller passed. Each required flag that is
// missing, and each flag named in replace, gets a placeholder. The
// placeholders fail validation, so a retry that still contains one sends
// nothing.
func feedbackRetryCommand(replace []string) string {
	parts := []string{"dwellir feedback"}
	for _, name := range []string{"profile", "json", "toon", "human", "quiet", "anon-telemetry"} {
		// Global flags are shared *pflag.Flag values, so Changed is set here too.
		flag := rootCmd.PersistentFlags().Lookup(name)
		switch {
		case name == "profile" && slices.Contains(replace, "--profile"):
			parts = append(parts, "--profile", "'<profile name>'")
		case flag == nil || !flag.Changed:
		case flag.Value.Type() == "bool" && flag.Value.String() == "true":
			parts = append(parts, "--"+name)
		default:
			parts = append(parts, "--"+name+"="+shellQuote(flag.Value.String()))
		}
	}
	for _, f := range []struct {
		flag, value, placeholder string
		required                 bool
	}{
		{"--sentiment", feedbackSentiment, "'<" + strings.Join(feedbackSentiments, "|") + ">'", true},
		{"--message", feedbackMessage, "'<your feedback>'", true},
		{"--context", feedbackContext, "'<what you were trying to do>'", true},
		{"--area", feedbackArea, "'<" + strings.Join(feedbackAreas, "|") + ">'", false},
		{"--actor", feedbackActor, "'<" + strings.Join(feedbackActors, "|") + ">'", false},
	} {
		switch {
		case slices.Contains(replace, f.flag) || (f.required && f.value == ""):
			parts = append(parts, f.flag, f.placeholder)
		case f.value != "":
			parts = append(parts, f.flag, shellQuote(f.value))
		}
	}
	return strings.Join(parts, " ")
}

// shellQuote returns s unchanged when the shell would pass it as one word,
// and single-quoted otherwise.
func shellQuote(s string) string {
	plain := s != "" && !strings.ContainsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("-_.,:/@", r)
	})
	if plain {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func feedbackUnavailableHelp() string {
	if !telemetry.ProductionAPI() {
		return "Feedback goes only to production, and DWELLIR_API_URL is set to " +
			strings.TrimSpace(os.Getenv("DWELLIR_API_URL")) + ". Unset DWELLIR_API_URL, then retry."
	}
	return "This is a development build without the analytics key that release builds carry. " +
		"Install a release build (see https://github.com/dwellir-public/cli#install), then retry."
}

func missingFeedbackFlags() []string {
	var missing []string
	for _, f := range []struct{ flag, value string }{
		{"--sentiment", feedbackSentiment},
		{"--message", feedbackMessage},
		{"--context", feedbackContext},
	} {
		if f.value == "" {
			missing = append(missing, f.flag)
		}
	}
	return missing
}

// cleanFeedbackText trims the text, turns tabs into spaces, drops other
// control characters except newlines, and masks values shaped like Dwellir API
// keys, so a pasted key never reaches analytics.
func cleanFeedbackText(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if r == '\n' || unicode.IsPrint(r) {
			return r
		}
		return -1
	}, strings.TrimSpace(s))
	return api.MaskKeyShapedValues(s)
}

func checkFeedbackText(field, s string) error {
	clean := cleanFeedbackText(s)
	if strings.HasPrefix(clean, "<") && strings.HasSuffix(clean, ">") {
		return fmt.Errorf("%s is still a placeholder (%s); replace it with real text", field, clean)
	}
	n := utf8.RuneCountInString(clean)
	if n < feedbackMinLen || n > feedbackMaxLen {
		return fmt.Errorf("%s has %d characters; it needs %d to %d", field, n, feedbackMinLen, feedbackMaxLen)
	}
	return nil
}

// promptFeedback asks for each required field that no flag supplied. It asks
// again after an invalid answer and stops at end of input.
func promptFeedback(in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	ask := func(prompt string, check func(string) error) (string, error) {
		for {
			_, _ = fmt.Fprint(out, prompt)
			line, err := reader.ReadString('\n')
			if errors.Is(err, io.EOF) && line == "" {
				return "", errors.New("input ended before every field was answered")
			}
			if err != nil && !errors.Is(err, io.EOF) {
				return "", err
			}
			line = strings.TrimSpace(line)
			checkErr := check(line)
			if checkErr == nil || err != nil {
				return line, checkErr
			}
			_, _ = fmt.Fprintf(out, "  %s\n", checkErr)
		}
	}
	oneOf := func(field string, allowed []string, optional bool) func(string) error {
		return func(s string) error {
			if (optional && s == "") || slices.Contains(allowed, s) {
				return nil
			}
			return fmt.Errorf("%s must be one of: %s", field, strings.Join(allowed, ", "))
		}
	}

	_, _ = fmt.Fprintf(out, "%s\n\n", feedbackNotSupport)
	var err error
	if feedbackSentiment == "" {
		if feedbackSentiment, err = ask("How do you feel about it? (positive, negative, neutral): ",
			oneOf("sentiment", feedbackSentiments, false)); err != nil {
			return err
		}
	}
	if feedbackArea == "" {
		if feedbackArea, err = ask("Which area is it about? (optional, press Enter to skip; "+strings.Join(feedbackAreas, ", ")+"): ",
			oneOf("area", feedbackAreas, true)); err != nil {
			return err
		}
	}
	if feedbackMessage == "" {
		if feedbackMessage, err = ask("What would you like to tell us? ",
			func(s string) error { return checkFeedbackText("feedback", s) }); err != nil {
			return err
		}
	}
	if feedbackContext == "" {
		if feedbackContext, err = ask("What were you trying to do? ",
			func(s string) error { return checkFeedbackText("context", s) }); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	feedbackCmd.Flags().SortFlags = false
	feedbackCmd.SetFlagErrorFunc(feedbackFlagError)
	feedbackCmd.Flags().StringVar(&feedbackSentiment, "sentiment", "",
		"How you feel about it (required): "+strings.Join(feedbackSentiments, ", "))
	feedbackCmd.Flags().StringVar(&feedbackMessage, "message", "",
		fmt.Sprintf("Your feedback (required, %d to %d characters)", feedbackMinLen, feedbackMaxLen))
	feedbackCmd.Flags().StringVar(&feedbackContext, "context", "",
		fmt.Sprintf("What you were trying to do (required, %d to %d characters)", feedbackMinLen, feedbackMaxLen))
	feedbackCmd.Flags().StringVar(&feedbackArea, "area", "", "Part of Dwellir the feedback is about (optional; see Areas)")
	feedbackCmd.Flags().StringVar(&feedbackActor, "actor", "",
		"Who wrote the feedback: human or agent (default: agent when a coding agent runs the CLI)")

	rootCmd.AddCommand(feedbackCmd)
}
