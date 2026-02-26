package cli

import (
	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
)

var (
	usageInterval string
	usageFrom     string
	usageTo       string
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "View usage analytics",
}

var usageSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Current billing cycle summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		summary, err := api.NewUsageAPI(client).Summary()
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("usage.summary", summary)
	},
}

var usageHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Usage over time",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		history, err := api.NewUsageAPI(client).History(usageInterval, usageFrom, usageTo)
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("usage.history", history)
	},
}

var usageRPSCmd = &cobra.Command{
	Use:   "rps",
	Short: "Current requests-per-second metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		rps, err := api.NewUsageAPI(client).RPS()
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("usage.rps", rps)
	},
}

func init() {
	usageHistoryCmd.Flags().StringVar(&usageInterval, "interval", "day", "Aggregation interval (minute, hour, day)")
	usageHistoryCmd.Flags().StringVar(&usageFrom, "from", "", "Start time (RFC3339)")
	usageHistoryCmd.Flags().StringVar(&usageTo, "to", "", "End time (RFC3339)")

	usageCmd.AddCommand(usageSummaryCmd, usageHistoryCmd, usageRPSCmd)
	rootCmd.AddCommand(usageCmd)
}
