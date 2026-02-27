package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
)

var (
	usageInterval string
	usageFrom     string
	usageTo       string
	usageAPIKey   string
	usageFQDN     string
	usageMethod   string
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
	Short: "Usage grouped by endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		window, err := resolveUsageWindow(usageInterval, usageFrom, usageTo)
		if err != nil {
			return err
		}
		if err := validateUsageLookback(client, window); err != nil {
			return err
		}
		if window.UsedDefaults && window.DefaultLabel != "" && !quiet {
			_, _ = fmt.Fprintf(
				cmd.ErrOrStderr(),
				"Using default usage window (%s): %s to %s\n",
				window.DefaultLabel,
				window.FormattedStart,
				window.FormattedEnd,
			)
		}

		usageAPI := api.NewUsageAPI(client)
		breakdown, err := usageAPI.EndpointBreakdown(
			window.Interval,
			window.FormattedStart,
			window.FormattedEnd,
			usageAPIKey,
			usageFQDN,
			usageMethod,
		)
		if err != nil {
			return formatCommandError(err)
		}
		if !isHumanOutput() {
			raw, rawErr := usageAPI.RawHistory(
				window.Interval,
				window.FormattedStart,
				window.FormattedEnd,
				usageAPIKey,
				usageFQDN,
				usageMethod,
			)
			if rawErr != nil {
				return formatCommandError(rawErr)
			}
			return getFormatter().Success("usage.history", raw)
		}
		return getFormatter().Success("usage.history", breakdown)
	},
}

var usageRPSCmd = &cobra.Command{
	Use:   "rps",
	Short: "Requests-per-second over time",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		window, err := resolveUsageWindow(usageInterval, usageFrom, usageTo)
		if err != nil {
			return err
		}
		if err := validateUsageLookback(client, window); err != nil {
			return err
		}
		if window.UsedDefaults && window.DefaultLabel != "" && !quiet {
			_, _ = fmt.Fprintf(
				cmd.ErrOrStderr(),
				"Using default usage window (%s): %s to %s\n",
				window.DefaultLabel,
				window.FormattedStart,
				window.FormattedEnd,
			)
		}

		rps, err := api.NewUsageAPI(client).RPS(
			window.Interval,
			window.FormattedStart,
			window.FormattedEnd,
			usageAPIKey,
			usageFQDN,
		)
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("usage.rps", rps)
	},
}

var usageMethodsCmd = &cobra.Command{
	Use:   "methods",
	Short: "Usage grouped by RPC method",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		window, err := resolveUsageWindow(usageInterval, usageFrom, usageTo)
		if err != nil {
			return err
		}
		if err := validateUsageLookback(client, window); err != nil {
			return err
		}
		if window.UsedDefaults && window.DefaultLabel != "" && !quiet {
			_, _ = fmt.Fprintf(
				cmd.ErrOrStderr(),
				"Using default usage window (%s): %s to %s\n",
				window.DefaultLabel,
				window.FormattedStart,
				window.FormattedEnd,
			)
		}

		methods, err := api.NewUsageAPI(client).MethodBreakdown(
			window.Interval,
			window.FormattedStart,
			window.FormattedEnd,
			usageAPIKey,
			usageFQDN,
		)
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("usage.methods", methods)
	},
}

var usageCostsCmd = &cobra.Command{
	Use:   "costs",
	Short: "Estimated cost breakdown for the selected usage window",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		window, err := resolveUsageWindow(usageInterval, usageFrom, usageTo)
		if err != nil {
			return err
		}
		if err := validateUsageLookback(client, window); err != nil {
			return err
		}
		if window.UsedDefaults && window.DefaultLabel != "" && !quiet {
			_, _ = fmt.Fprintf(
				cmd.ErrOrStderr(),
				"Using default usage window (%s): %s to %s\n",
				window.DefaultLabel,
				window.FormattedStart,
				window.FormattedEnd,
			)
		}

		accountAPI := api.NewAccountAPI(client)
		sub, err := accountAPI.Subscription()
		if err != nil {
			return formatCommandError(err)
		}
		info, err := accountAPI.Info()
		if err != nil {
			return formatCommandError(err)
		}
		discount, err := accountAPI.Discount()
		if err != nil {
			return formatCommandError(err)
		}

		usageAPI := api.NewUsageAPI(client)
		filteredRows, err := usageAPI.RawHistory(
			window.Interval,
			window.FormattedStart,
			window.FormattedEnd,
			usageAPIKey,
			usageFQDN,
			usageMethod,
		)
		if err != nil {
			return formatCommandError(err)
		}

		hasFilters := usageAPIKey != "" || usageFQDN != "" || usageMethod != ""
		basisRows := filteredRows
		if hasFilters {
			earliest := api.EarliestBillingPeriodStart(window.Start, window.End, info.CurrentSubscription)
			basisRows, err = usageAPI.RawHistory(
				window.Interval,
				earliest.Format(time.RFC3339),
				window.FormattedEnd,
				"",
				"",
				"",
			)
			if err != nil {
				return formatCommandError(err)
			}
		}

		monthlyQuota := 0
		if sub.MonthlyQuota != nil {
			monthlyQuota = *sub.MonthlyQuota
		}

		report := api.CalculateUsageCostReport(
			*sub,
			monthlyQuota,
			discount,
			info.CurrentSubscription,
			window.Start,
			window.End,
			filteredRows,
			basisRows,
		)
		return getFormatter().Success("usage.costs", report)
	},
}

var usageLimitsCmd = &cobra.Command{
	Use:   "limits",
	Short: "Show plan-based usage query limits",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		sub, err := api.NewAccountAPI(client).Subscription()
		if err != nil {
			return formatCommandError(err)
		}
		_, lookbackLabel, tierName := planLookback(sub.ID)
		return getFormatter().Success("usage.limits", map[string]string{
			"plan":                sub.EffectivePlanName(),
			"tier":                tierName,
			"max_lookback":        lookbackLabel,
			"supported_intervals": "minute,hour,day",
		})
	},
}

func init() {
	for _, sub := range []*cobra.Command{usageHistoryCmd, usageRPSCmd, usageMethodsCmd, usageCostsCmd} {
		sub.Flags().StringVar(&usageInterval, "interval", "hour", "Aggregation interval (minute, hour, day). Default: hour.")
		sub.Flags().StringVar(&usageFrom, "from", "", "Start time (RFC3339). Example: 2026-02-27T00:00:00Z")
		sub.Flags().StringVar(&usageTo, "to", "", "End time (RFC3339). Example: 2026-02-27T23:59:59Z")
		sub.Flags().StringVar(&usageAPIKey, "api-key", "", "Filter by API key value")
		sub.Flags().StringVar(&usageFQDN, "fqdn", "", "Filter by endpoint hostname (FQDN)")
	}
	usageHistoryCmd.Flags().StringVar(&usageMethod, "method", "", "Filter by RPC method")
	usageCostsCmd.Flags().StringVar(&usageMethod, "method", "", "Filter by RPC method")

	usageCmd.AddCommand(usageSummaryCmd, usageHistoryCmd, usageRPSCmd, usageMethodsCmd, usageCostsCmd, usageLimitsCmd)
	rootCmd.AddCommand(usageCmd)
}
