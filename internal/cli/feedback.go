package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

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
// --message or --context value that the shell split into words.
func feedbackNoArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return getFormatter().Error("validation_error",
		fmt.Sprintf("dwellir feedback takes no positional arguments (got %d: %s). Nothing was sent.",
			len(args), strings.Join(args, " ")),
		"Put the text in --message and --context, and quote each value so the shell passes it as one argument.\n"+
			"Retry: "+feedbackRetryCommand())
}

func runFeedback(cmd *cobra.Command, args []string) error {
	f := getFormatter()
	// Check the flags that were passed before asking for the rest, so a bad
	// flag fails at once instead of after the prompts.
	if bad := invalidFeedbackFlags(); len(bad) > 0 {
		problems, fixes, flags := make([]string, len(bad)), make([]string, len(bad)), make([]string, len(bad))
		for i, b := range bad {
			problems[i], fixes[i], flags[i] = b.problem, b.fix, b.flag
		}
		return f.Error("validation_error", strings.Join(problems, " ")+" Nothing was sent.",
			strings.Join(fixes, "\n")+"\nRetry: "+feedbackRetryCommand(flags...))
	}

	if missing := missingFeedbackFlags(); len(missing) > 0 {
		canPrompt := humanOutput && isHumanOutput() && stdinIsTerminal()
		if !canPrompt {
			var help strings.Builder
			for _, flag := range missing {
				help.WriteString(flag + ": " + feedbackFlagUsage[flag] + "\n")
			}
			help.WriteString("Retry: " + feedbackRetryCommand())
			help.WriteString("\nA person at a terminal can run 'dwellir feedback --human' to be asked for each field instead.")
			return f.Error("validation_error",
				"Missing required flags: "+strings.Join(missing, ", ")+". Nothing was sent.", help.String())
		}
		if err := promptFeedback(cmd.InOrStdin(), cmd.ErrOrStderr()); err != nil {
			return f.Error("validation_error", "Feedback was not sent: "+err.Error()+".",
				"Run 'dwellir feedback --human' again and answer every question, or pass the flags instead.\n"+
					"Retry: "+feedbackRetryCommand())
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
			"Retry the same command.")
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
				feedbackUnavailableHelp())
		}
		return f.Error("network_error", "Feedback was not sent: "+err.Error()+".",
			"Check the network connection, then retry the same command. "+
				"If the request timed out, the feedback may have arrived; a duplicate is harmless.")
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

// feedbackRetryCommand returns a command line that keeps every valid flag the
// caller passed and puts a placeholder in each required flag that is missing
// or named in invalid. An agent can retry by filling in the placeholders.
func feedbackRetryCommand(invalid ...string) string {
	parts := []string{"dwellir feedback"}
	for _, f := range []struct {
		flag, value, placeholder string
	}{
		{"--sentiment", feedbackSentiment, `"<` + strings.Join(feedbackSentiments, "|") + `>"`},
		{"--message", feedbackMessage, `"<your feedback>"`},
		{"--context", feedbackContext, `"<what you were trying to do>"`},
		{"--area", feedbackArea, ""},
		{"--actor", feedbackActor, ""},
	} {
		switch {
		case f.value != "" && !slices.Contains(invalid, f.flag):
			parts = append(parts, f.flag, shellQuote(f.value))
		case f.placeholder != "":
			parts = append(parts, f.flag, f.placeholder)
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
	if apiURL := strings.TrimSpace(os.Getenv("DWELLIR_API_URL")); apiURL != "" {
		return "Feedback goes only to production, and DWELLIR_API_URL is set to " + apiURL +
			". Unset DWELLIR_API_URL, then retry the same command."
	}
	return "This is a development build without the analytics key that release builds carry. " +
		"Install a release build (see https://github.com/dwellir-public/cli#install), then retry the same command."
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
	n := utf8.RuneCountInString(cleanFeedbackText(s))
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
