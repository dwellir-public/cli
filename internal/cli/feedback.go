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
	Args: noArgs(`dwellir feedback --sentiment positive --message "..." --context "..."`),
	RunE: withRenderedExitCodes(runFeedback),
}

func runFeedback(cmd *cobra.Command, args []string) error {
	f := getFormatter()
	if feedbackActor == "" {
		feedbackActor = "human"
		if isAgentEnvironment() {
			feedbackActor = "agent"
		}
	}
	// Check the flags that were passed before asking for the rest, so a bad
	// flag fails at once instead of after the prompts.
	if err := validateFeedbackFlags(); err != nil {
		return f.Error("validation_error", err.Error(), "Run 'dwellir feedback --help' for the accepted values.")
	}

	if missing := missingFeedbackFlags(); len(missing) > 0 {
		canPrompt := humanOutput && isHumanOutput() && stdinIsTerminal()
		if !canPrompt {
			return f.Error("validation_error",
				"Missing required flags: "+strings.Join(missing, ", ")+".",
				`Pass every required flag, or run 'dwellir feedback --human' in a terminal to be asked.
Example: dwellir feedback --sentiment positive --message "..." --context "..."`)
		}
		if err := promptFeedback(cmd.InOrStdin(), cmd.ErrOrStderr()); err != nil {
			return f.Error("validation_error", "Feedback was not sent: "+err.Error(), "")
		}
	}

	id, err := uuid.NewV7()
	if err != nil {
		return f.Error("error", err.Error(), "")
	}
	props := map[string]interface{}{
		"feedback_id":   id.String(),
		"sentiment":     feedbackSentiment,
		"message":       cleanFeedbackText(feedbackMessage),
		"context":       cleanFeedbackText(feedbackContext),
		"actor":         feedbackActor,
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
			return f.Error("feedback_unavailable", "This CLI build cannot send feedback.",
				"Release builds send feedback. A development build or a custom DWELLIR_API_URL cannot.")
		}
		return f.Error("network_error", "Feedback was not sent: "+err.Error(),
			"Check the network connection and retry.")
	}

	result := map[string]string{"id": id.String(), "sentiment": feedbackSentiment, "actor": feedbackActor}
	if feedbackArea != "" {
		result["area"] = feedbackArea
	}
	return f.Success("feedback", output.Notice{
		Data: result,
		Text: "Thanks. Your feedback was sent. " + feedbackNotSupport,
	})
}

func validateFeedbackFlags() error {
	checks := []struct {
		flag, value string
		allowed     []string
	}{
		{"--sentiment", feedbackSentiment, feedbackSentiments},
		{"--area", feedbackArea, feedbackAreas},
		{"--actor", feedbackActor, feedbackActors},
	}
	for _, c := range checks {
		if c.value != "" && !slices.Contains(c.allowed, c.value) {
			return fmt.Errorf("%s must be one of: %s", c.flag, strings.Join(c.allowed, ", "))
		}
	}
	if feedbackMessage != "" {
		if err := checkFeedbackText("--message", feedbackMessage); err != nil {
			return err
		}
	}
	if feedbackContext != "" {
		if err := checkFeedbackText("--context", feedbackContext); err != nil {
			return err
		}
	}
	return nil
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
		return fmt.Errorf("%s must be %d to %d characters (got %d)", field, feedbackMinLen, feedbackMaxLen, n)
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

	if feedbackEnabled() {
		rootCmd.AddCommand(feedbackCmd)
	}
}
