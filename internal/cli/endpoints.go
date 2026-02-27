package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
	"github.com/dwellir-public/cli/internal/auth"
	"github.com/dwellir-public/cli/internal/config"
)

var (
	epProtocol  string
	epNodeType  string
	epEcosystem string
	epNetwork   string
)

var endpointsCmd = &cobra.Command{
	Use:   "endpoints",
	Short: "Browse and search blockchain endpoints",
	Long: `Browse and search blockchain endpoints.

Use filter flags with any subcommand:
  --ecosystem  Filter by ecosystem (evm, substrate, cosmos, move)
  --node-type  Filter by node type (full, archive)
  --protocol   Filter by protocol (https, wss)
  --network    Filter by network (mainnet, testnet, or network name)

Examples:
  dwellir endpoints list --ecosystem evm --network mainnet
  dwellir endpoints search base --node-type archive --protocol https
  dwellir endpoints get ethereum --network sepolia`,
}

var endpointsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available endpoints",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			f := getFormatter()
			return f.Error("not_authenticated", err.Error(), "")
		}
		ep := api.NewEndpointsAPI(client)
		chains, err := ep.Search("", epEcosystem, epNodeType, epProtocol, epNetwork)
		if err != nil {
			return formatCommandError(err)
		}
		f := getFormatter()
		return f.Success("endpoints.list", chains)
	},
}

var endpointsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search endpoints by chain or network name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			f := getFormatter()
			return f.Error("not_authenticated", err.Error(), "")
		}
		ep := api.NewEndpointsAPI(client)
		chains, err := ep.Search(args[0], epEcosystem, epNodeType, epProtocol, epNetwork)
		if err != nil {
			return formatCommandError(err)
		}
		f := getFormatter()
		return f.Success("endpoints.search", chains)
	},
}

var endpointsGetCmd = &cobra.Command{
	Use:   "get <chain>",
	Short: "Get endpoints for a specific chain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			f := getFormatter()
			return f.Error("not_authenticated", err.Error(), "")
		}
		ep := api.NewEndpointsAPI(client)
		chains, err := ep.Search(args[0], epEcosystem, epNodeType, epProtocol, epNetwork)
		if err != nil {
			return formatCommandError(err)
		}
		f := getFormatter()
		if len(chains) == 0 {
			return f.Error("not_found", "No endpoints found for '"+args[0]+"'.", "Run 'dwellir endpoints list' to see all available chains.")
		}
		return f.Success("endpoints.get", chains)
	},
}

func init() {
	endpointsCmd.PersistentFlags().StringVar(&epEcosystem, "ecosystem", "", "Filter by ecosystem (evm, substrate, cosmos, move)")
	endpointsCmd.PersistentFlags().StringVar(&epNodeType, "node-type", "", "Filter by node type (full, archive)")
	endpointsCmd.PersistentFlags().StringVar(&epProtocol, "protocol", "", "Filter by protocol (https, wss)")
	endpointsCmd.PersistentFlags().StringVar(&epNetwork, "network", "", "Filter by network (mainnet, testnet, or network name)")
	endpointsCmd.AddCommand(endpointsListCmd, endpointsSearchCmd, endpointsGetCmd)
	rootCmd.AddCommand(endpointsCmd)
}

// newAPIClient creates a Marly API client from the resolved auth token.
func newAPIClient() (*api.Client, error) {
	configDir := config.DefaultConfigDir()
	cwd, _ := os.Getwd()
	token, err := auth.ResolveToken(tokenFlag, profile, cwd, configDir)
	if err != nil {
		return nil, err
	}

	baseURL := os.Getenv("DWELLIR_API_URL")
	if baseURL == "" {
		baseURL = "https://dashboard.dwellir.com/marly-api"
	}

	client := api.NewClient(baseURL, token)

	client.OnTokenRefresh = func(newToken string) {
		envProfile := os.Getenv("DWELLIR_PROFILE")
		profileName := config.ResolveProfileName(profile, envProfile, cwd, configDir)
		p, _ := config.LoadProfile(configDir, profileName)
		if p != nil {
			p.Token = newToken
			_ = config.SaveProfile(configDir, p)
		}
	}

	return client, nil
}
