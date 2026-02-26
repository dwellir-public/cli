package cli

import "github.com/spf13/cobra"

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
}

func Execute() error {
	return rootCmd.Execute()
}
