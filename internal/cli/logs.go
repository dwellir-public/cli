package cli

import (
	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
	"github.com/dwellir-public/cli/internal/output"
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
		apiKey, err := keyFilterValue(client, logKey)
		if err != nil {
			return err
		}
		filters := buildLogFilters(apiKey)
		page, err := api.NewLogsAPI(client).Errors(filters)
		if err != nil {
			return formatCommandError(err)
		}
		rows := nameErrorLogs(client, page.Items)
		return getFormatter().Success("logs.errors", withNotice(output.Page{
			Items:      rows,
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
		}, len(rows) > 0, maskedKeysNotice))
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
		apiKey, err := keyFilterValue(client, logKey)
		if err != nil {
			return err
		}
		filters := buildLogFilters(apiKey)
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
		apiKey, err := keyFilterValue(client, logKey)
		if err != nil {
			return err
		}
		filters := buildLogFilters(apiKey)
		facets, err := api.NewLogsAPI(client).Facets(filters)
		if err != nil {
			return formatCommandError(err)
		}
		facets = nameKeyFacets(client, facets)
		return getFormatter().Success("logs.facets", withNotice(facets, facets != nil && len(facets.APIKeys) > 0, maskedKeysNotice))
	},
}

func buildLogFilters(apiKey string) map[string]interface{} {
	filters := map[string]interface{}{}
	if apiKey != "" {
		filters["api_key"] = apiKey
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
		cmd.Flags().StringVar(&logKey, "key", "", "Filter by API key name or value")
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
