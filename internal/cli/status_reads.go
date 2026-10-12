package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
)

var (
	statusComponentsChains []string

	statusHistoryFlags  statusFilterFlags
	statusHistorySince  string
	statusHistoryStatus string
	statusHistoryLimit  int

	statusUptimeFlags statusFilterFlags
	statusUptimeRange string
)

var statusUptimeRanges = []string{"24h", "7d", "30d", "90d", "6m", "12m"}

var statusComponentsCmd = &cobra.Command{
	Use:   "components",
	Short: "List chains, networks, and endpoints you can filter on",
	Long: `List every chain, network, and endpoint on the status page with its filter value.

Use the chain slug with --chain, the network ref with --network, and the
endpoint FQDN with --endpoint.

Examples:
  dwellir status components
  dwellir status components --chain ethereum,base`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newStatusAPI()
		if err != nil {
			return err
		}
		components, err := client.Components()
		if err != nil {
			return statusCommandError(err)
		}
		filter := api.NewStatusFilter(statusComponentsChains, nil, nil)
		if unknown := findUnknownFilterValues(filter, components); len(unknown) > 0 {
			return unknownFilterError(unknown, false)
		}
		return getFormatter().Success("status.components", filterComponentChains(components, filter.Chains))
	},
}

var statusHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "List incidents, newest first",
	Long: `List incidents that started in the time window, newest first.

--since takes a duration such as 24h, 7d, or 30d, or an ISO 8601 timestamp.

Examples:
  dwellir status history
  dwellir status history --chain base --since 30d
  dwellir status history --status open --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := statusEnumFlag("status", statusHistoryStatus, "open", "resolved", "all"); err != nil {
			return err
		}
		if statusHistoryLimit < 1 || statusHistoryLimit > 200 {
			return statusFail("validation_error", fmt.Sprintf("--limit must be from 1 to 200, not %d.", statusHistoryLimit), "")
		}
		if strings.TrimSpace(statusHistorySince) == "" {
			return statusFail("validation_error", "--since cannot be empty.", "Use a duration such as 7d, or an ISO 8601 timestamp.")
		}
		client, err := newStatusAPI()
		if err != nil {
			return err
		}
		incidents, err := client.Incidents(statusHistoryFlags.filter(), api.IncidentQuery{
			Since:  strings.TrimSpace(statusHistorySince),
			Status: statusHistoryStatus,
			Limit:  statusHistoryLimit,
		})
		if err != nil {
			return statusCommandError(err)
		}
		return getFormatter().Success("status.history", incidents)
	},
}

var statusUptimeCmd = &cobra.Command{
	Use:   "uptime",
	Short: "Show uptime and lag share per endpoint",
	Long: `Show each endpoint's uptime percentage and lagging sample share for a range.

A lagging sample trailed the chain head by 30 seconds or more.

Examples:
  dwellir status uptime --chain ethereum
  dwellir status uptime --network base/mainnet --range 30d`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := statusEnumFlag("range", statusUptimeRange, statusUptimeRanges...); err != nil {
			return err
		}
		client, err := newStatusAPI()
		if err != nil {
			return err
		}
		uptime, err := client.Uptime(statusUptimeRange, statusUptimeFlags.filter())
		if err != nil {
			return statusCommandError(err)
		}
		return getFormatter().Success("status.uptime", uptime)
	},
}

// filterComponentChains keeps the chains in slugs, or every chain when slugs is
// empty.
func filterComponentChains(components *api.StatusComponents, slugs []string) *api.StatusComponents {
	if len(slugs) == 0 {
		return components
	}
	filter := api.StatusFilter{Chains: slugs}
	out := &api.StatusComponents{GeneratedAt: components.GeneratedAt, Chains: []api.StatusComponentChain{}}
	for _, chain := range components.Chains {
		if filter.Matches(chain.Slug, "", "") {
			out.Chains = append(out.Chains, chain)
		}
	}
	return out
}

func init() {
	statusComponentsCmd.Flags().StringSliceVar(&statusComponentsChains, "chain", nil, "Chain slug (repeatable, comma-separated)")

	statusHistoryFlags.register(statusHistoryCmd)
	statusHistoryCmd.Flags().StringVar(&statusHistorySince, "since", "7d", "Start of the window: a duration (24h, 7d, 30d) or an ISO 8601 timestamp")
	statusHistoryCmd.Flags().StringVar(&statusHistoryStatus, "status", "all", "Incident status: open, resolved, or all")
	statusHistoryCmd.Flags().IntVar(&statusHistoryLimit, "limit", 50, "Maximum incidents to return (1-200)")

	statusUptimeFlags.register(statusUptimeCmd)
	statusUptimeCmd.Flags().StringVar(&statusUptimeRange, "range", "7d", "Range: "+strings.Join(statusUptimeRanges, ", "))
}
