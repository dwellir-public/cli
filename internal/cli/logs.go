package cli

import (
	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
)

var (
	logKey        string
	logEndpoint   string
	logStatusCode int
	logRPCMethod  string
	logFrom       string
	logTo         string
	logLimit      int
	logCursor     string
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View error logs",
}

var logsErrorsCmd = &cobra.Command{
	Use:   "errors",
	Short: "List error logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		filters := buildLogFilters()
		logs, err := api.NewLogsAPI(client).Errors(filters)
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("logs.errors", logs)
	},
}

var logsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Error metrics and classifications",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		filters := buildLogFilters()
		stats, err := api.NewLogsAPI(client).Stats(filters)
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("logs.stats", stats)
	},
}

var logsFacetsCmd = &cobra.Command{
	Use:   "facets",
	Short: "Error facet aggregations",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		filters := buildLogFilters()
		facets, err := api.NewLogsAPI(client).Facets(filters)
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("logs.facets", facets)
	},
}

func buildLogFilters() map[string]interface{} {
	filters := map[string]interface{}{}
	if logKey != "" {
		filters["api_key"] = logKey
	}
	if logEndpoint != "" {
		filters["fqdn"] = logEndpoint
	}
	if logStatusCode != 0 {
		filters["status_code"] = logStatusCode
	}
	if logRPCMethod != "" {
		filters["rpc_method"] = logRPCMethod
	}
	if logFrom != "" {
		filters["from"] = logFrom
	}
	if logTo != "" {
		filters["to"] = logTo
	}
	if logLimit > 0 {
		filters["limit"] = logLimit
	}
	if logCursor != "" {
		filters["cursor"] = logCursor
	}
	return filters
}

func init() {
	for _, cmd := range []*cobra.Command{logsErrorsCmd, logsStatsCmd, logsFacetsCmd} {
		cmd.Flags().StringVar(&logKey, "key", "", "Filter by API key")
		cmd.Flags().StringVar(&logEndpoint, "endpoint", "", "Filter by FQDN")
		cmd.Flags().IntVar(&logStatusCode, "status-code", 0, "Filter by HTTP status code")
		cmd.Flags().StringVar(&logRPCMethod, "rpc-method", "", "Filter by RPC method")
		cmd.Flags().StringVar(&logFrom, "from", "", "Start time (RFC3339)")
		cmd.Flags().StringVar(&logTo, "to", "", "End time (RFC3339)")
	}
	logsErrorsCmd.Flags().IntVar(&logLimit, "limit", 50, "Max results")
	logsErrorsCmd.Flags().StringVar(&logCursor, "cursor", "", "Pagination cursor")

	logsCmd.AddCommand(logsErrorsCmd, logsStatsCmd, logsFacetsCmd)
	rootCmd.AddCommand(logsCmd)
}
