package cli

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
)

const statusURLEnvVar = "DWELLIR_STATUS_URL"

var (
	statusBaseURLFlag  string
	statusCurrentAll   bool
	statusCurrentFlags statusFilterFlags
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Dwellir endpoint status, incidents, and uptime",
	Long: `Read the public status API at https://status.dwellir.com. No login needed.

With no subcommand, show the current status. Without filters, list only
endpoints that are not operational. Add --all to list every endpoint.

Filters:
  --chain     Chain slug, such as ethereum
  --network   Network ref <chain>/<network>, such as ethereum/mainnet
  --endpoint  Endpoint FQDN or URL, such as api-ethereum-mainnet.n.dwellir.com

Repeat a filter flag or separate values with commas. An endpoint matches when
any value matches. Run 'dwellir status components' to list valid values.

Examples:
  dwellir status
  dwellir status --chain base,ethereum
  dwellir status history --since 24h --status open
  dwellir status uptime --network ethereum/mainnet --range 30d
  dwellir status feed --chain base --slack
  dwellir status subscribe --url https://hooks.slack.com/services/T/B/X --chain base`,
	Args: cobra.NoArgs,
	RunE: runStatusCurrent,
}

// statusFilterFlags holds one command's --chain, --network and --endpoint
// values. Each command owns its own set so values never leak between commands.
type statusFilterFlags struct {
	chains    []string
	networks  []string
	endpoints []string
}

func (f *statusFilterFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringSliceVar(&f.chains, "chain", nil, "Chain slug (repeatable, comma-separated)")
	cmd.Flags().StringSliceVar(&f.networks, "network", nil, "Network ref <chain>/<network> (repeatable, comma-separated)")
	cmd.Flags().StringSliceVar(&f.endpoints, "endpoint", nil, "Endpoint FQDN or URL (repeatable, comma-separated)")
}

func (f *statusFilterFlags) filter() api.StatusFilter {
	return api.NewStatusFilter(f.chains, f.networks, f.endpoints)
}

func runStatusCurrent(cmd *cobra.Command, args []string) error {
	client, err := newStatusAPI()
	if err != nil {
		return err
	}
	filter := statusCurrentFlags.filter()
	current, err := client.Current(filter)
	if err != nil {
		return statusCommandError(err)
	}
	return getFormatter().Success("status", api.NewStatusOverview(current, !filter.IsEmpty(), statusCurrentAll))
}

// resolveStatusBaseURL picks --status-url, then DWELLIR_STATUS_URL, then the
// public default.
func resolveStatusBaseURL() (string, error) {
	raw := strings.TrimSpace(statusBaseURLFlag)
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv(statusURLEnvVar))
	}
	if raw == "" {
		raw = api.DefaultStatusBaseURL
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("status URL %q must be an absolute http or https URL", raw)
	}
	return strings.TrimSuffix(raw, "/"), nil
}

func newStatusAPI() (*api.StatusAPI, error) {
	baseURL, err := resolveStatusBaseURL()
	if err != nil {
		return nil, statusFail("validation_error", err.Error(),
			"Set --status-url or "+statusURLEnvVar+" to a URL such as "+api.DefaultStatusBaseURL+".")
	}
	return api.NewStatusAPI(baseURL, nil), nil
}

// statusFail renders an error and attaches the exit code for its error code.
func statusFail(code, message, help string) error {
	return withExitCode(getFormatter().Error(code, message, help), code)
}

// statusCommandError maps a status API failure to an error code and exit code:
// HTTP 400 is a validation error (4), 404 is not found (3), and anything else
// exits 1.
func statusCommandError(err error) error {
	return statusFail(classifyStatusError(err))
}

func classifyStatusError(err error) (code, message, help string) {
	var apiErr *api.StatusAPIError
	if errors.As(err, &apiErr) {
		return classifyStatusAPIError(apiErr)
	}
	if strings.HasPrefix(err.Error(), "request failed") {
		return "network_error", err.Error(),
			"Check the network connection and --status-url or " + statusURLEnvVar + ", then retry."
	}
	return "error", err.Error(), ""
}

func classifyStatusAPIError(apiErr *api.StatusAPIError) (code, message, help string) {
	message = apiErr.Message
	if message == "" {
		message = fmt.Sprintf("The status API returned HTTP %d.", apiErr.StatusCode)
	}
	switch {
	case apiErr.StatusCode == 400:
		return "validation_error", message,
			"Check the flag values. Run 'dwellir status components' to list valid chains, networks, and endpoints."
	case apiErr.StatusCode == 404:
		return "not_found", fmt.Sprintf("The status API returned HTTP 404 at %s.", statusBaseURLForHelp()),
			"Check --status-url or " + statusURLEnvVar + ". The default is " + api.DefaultStatusBaseURL + "."
	case apiErr.StatusCode == 409:
		return "conflict", message,
			"The URL already has a live subscription. Run 'dwellir status unsubscribe <unsubscribe-url>' first, or use another URL."
	case apiErr.StatusCode == 429:
		return "rate_limited", message, ""
	case apiErr.StatusCode >= 500:
		return "status_unavailable", message, "The status API failed. Retry later."
	default:
		return "api_error", message, ""
	}
}

func statusBaseURLForHelp() string {
	baseURL, err := resolveStatusBaseURL()
	if err != nil {
		return api.DefaultStatusBaseURL
	}
	return baseURL
}

// statusEnumFlag checks a flag against its allowed values.
func statusEnumFlag(flag, value string, allowed ...string) error {
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}
	return statusFail("validation_error",
		fmt.Sprintf("--%s must be one of %s, not %q.", flag, strings.Join(allowed, ", "), value), "")
}

func init() {
	statusCmd.PersistentFlags().StringVar(&statusBaseURLFlag, "status-url", "",
		"Status API base URL (env "+statusURLEnvVar+", default "+api.DefaultStatusBaseURL+")")
	statusCurrentFlags.register(statusCmd)
	statusCmd.Flags().BoolVar(&statusCurrentAll, "all", false, "List operational endpoints too")

	statusCmd.AddCommand(
		statusComponentsCmd,
		statusHistoryCmd,
		statusUptimeCmd,
		statusFeedCmd,
		statusSubscribeCmd,
		statusUnsubscribeCmd,
	)

	if statusEnabled() {
		rootCmd.AddCommand(statusCmd)
	}
}
