package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/config"
	"github.com/dwellir-public/cli/internal/output"
)

var (
	jsonOutput    bool
	humanOutput   bool
	profile       string
	quiet         bool
	anonTelemetry bool
)

var rootCmd = &cobra.Command{
	Use:   "dwellir",
	Short: "Dwellir CLI — Blockchain RPC infrastructure from your terminal",
	Long: `Dwellir CLI provides full access to the Dwellir platform.

Manage API keys, browse blockchain endpoints, view usage analytics,
and debug error logs — all from the command line.

Get started:
  dwellir auth login       Authenticate with your Dwellir account
  dwellir docs search rpc  Search Dwellir documentation
  dwellir endpoints list   Browse available blockchain endpoints
  dwellir keys list        List your API keys

Documentation: https://dwellir.com/docs
Dashboard:     https://dashboard.dwellir.com`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&humanOutput, "human", false, "Output as human-readable (default)")
	rootCmd.PersistentFlags().StringVar(&profile, "profile", "", "Use a specific auth profile")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&anonTelemetry, "anon-telemetry", false, "Anonymize telemetry data")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		if output.IsRenderedError(err) {
			return err
		}
		code, message, help := classifyExecutionError(err)
		f := getFormatter()
		if isJSONRequested(os.Args[1:]) {
			f = output.New("json", rootCmd.OutOrStdout())
		}
		return f.Error(code, message, help)
	}
	return nil
}

func isJSONRequested(args []string) bool {
	if len(args) == 0 {
		return false
	}
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
	}
	return false
}

func buildFormatter(format string) output.Formatter {
	return output.New(format, rootCmd.OutOrStdout())
}

func resolvedOutputFormat() string {
	cfg, _ := config.Load(config.DefaultConfigDir())
	format := cfg.Output
	if isAgentEnvironment() {
		format = "json"
	}
	if jsonOutput {
		format = "json"
	}
	if humanOutput {
		format = "human"
	}
	return format
}

func isHumanOutput() bool {
	return resolvedOutputFormat() != "json"
}

func getFormatter() output.Formatter {
	return buildFormatter(resolvedOutputFormat())
}

func isAgentEnvironment() bool {
	markers := [...]string{
		"CODEX_CI",
		"CODEX_THREAD_ID",
		"CLAUDECODE",
		"CLAUDE_CODE_ENTRYPOINT",
		"OPENCODE",
		"CURSOR_AGENT",
	}
	for _, key := range markers {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}
