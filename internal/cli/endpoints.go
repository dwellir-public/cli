package cli

import (
	"fmt"
	"os"
	"strings"

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
	epKeyName   string
)

var endpointsCmd = &cobra.Command{
	Use:   "endpoints",
	Short: "Browse and search blockchain endpoints",
	Long: `Browse and search blockchain endpoints.

Use filter flags with any subcommand:
  --ecosystem  Filter by ecosystem (evm, substrate, cosmos, move, hyperliquid, other)
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
	Args:  endpointsListArgs,
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
		chains = applyPremiumEndpointAccess(client, chains)
		f := getFormatter()
		return f.Success("endpoints.list", withNotice(chains, endpointsHaveKeyPlaceholder(chains), endpointKeyNotice))
	},
}

var endpointsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search endpoints by chain or network name",
	Args:  endpointsSearchArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		client, err := newAPIClient()
		if err != nil {
			f := getFormatter()
			return f.Error("not_authenticated", err.Error(), "")
		}
		ep := api.NewEndpointsAPI(client)
		chains, err := ep.Search(query, epEcosystem, epNodeType, epProtocol, epNetwork)
		if err != nil {
			return formatCommandError(err)
		}
		chains = applyPremiumEndpointAccess(client, chains)
		f := getFormatter()
		return f.Success("endpoints.search", withNotice(chains, endpointsHaveKeyPlaceholder(chains), endpointKeyNotice))
	},
}

var endpointsGetCmd = &cobra.Command{
	Use:   "get <chain>",
	Short: "Get endpoints for a specific chain",
	Args:  endpointsGetArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		chainLookup := args[0]
		client, err := newAPIClient()
		if err != nil {
			f := getFormatter()
			return f.Error("not_authenticated", err.Error(), "")
		}
		ep := api.NewEndpointsAPI(client)
		chains, err := ep.Get(chainLookup, epEcosystem, epNodeType, epProtocol, epNetwork)
		if err != nil {
			return formatCommandError(err)
		}
		chains = applyPremiumEndpointAccess(client, chains)
		f := getFormatter()
		if len(chains) == 0 {
			return f.Error("not_found", "No endpoints found for '"+chainLookup+"'.", "Run 'dwellir endpoints list' to see all available chains.")
		}
		return f.Success("endpoints.get", withNotice(chains, endpointsHaveKeyPlaceholder(chains), endpointKeyNotice))
	},
}

func init() {
	endpointsCmd.PersistentFlags().StringVar(&epEcosystem, "ecosystem", "", "Filter by ecosystem (evm, substrate, cosmos, move, hyperliquid, other)")
	endpointsCmd.PersistentFlags().StringVar(&epNodeType, "node-type", "", "Filter by node type (full, archive)")
	endpointsCmd.PersistentFlags().StringVar(&epProtocol, "protocol", "", "Filter by protocol (https, wss)")
	endpointsCmd.PersistentFlags().StringVar(&epNetwork, "network", "", "Filter by network (mainnet, testnet, or network name)")
	// --key used to put the full key into the URLs. It stays defined so old
	// scripts get a clear error instead of URLs that silently lack the key.
	endpointsCmd.PersistentFlags().StringVar(&epKeyName, "key", "", "Removed: endpoint output never contains API keys")
	if keyFlag := endpointsCmd.PersistentFlags().Lookup("key"); keyFlag != nil {
		keyFlag.NoOptDefVal = " "
		keyFlag.Hidden = true
	}
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

	return api.NewClient(resolveAPIBaseURL(), token), nil
}

func endpointsListArgs(cmd *cobra.Command, args []string) error {
	if err := rejectEndpointKeyFlag(cmd); err != nil {
		return err
	}
	if len(args) > 0 {
		return getFormatter().Error(
			"validation_error",
			fmt.Sprintf("Unexpected arguments: %s", strings.Join(args, " ")),
			"Run `dwellir endpoints --help` to see valid syntax.",
		)
	}
	return nil
}

func endpointsSearchArgs(cmd *cobra.Command, args []string) error {
	if err := rejectEndpointKeyFlag(cmd); err != nil {
		return err
	}
	if len(args) < 1 {
		return getFormatter().Error(
			"validation_error",
			"Missing required argument <query>.",
			"Example: dwellir endpoints search base --network mainnet",
		)
	}
	if len(args) > 1 {
		return getFormatter().Error(
			"validation_error",
			fmt.Sprintf("Too many arguments for endpoints search (got %d).", len(args)),
			"Usage: dwellir endpoints search <query>",
		)
	}
	return nil
}

func endpointsGetArgs(cmd *cobra.Command, args []string) error {
	if err := rejectEndpointKeyFlag(cmd); err != nil {
		return err
	}
	if len(args) < 1 {
		return getFormatter().Error(
			"validation_error",
			"Missing required argument <chain>.",
			"Example: dwellir endpoints get base --network mainnet",
		)
	}
	if len(args) > 1 {
		return getFormatter().Error(
			"validation_error",
			fmt.Sprintf("Too many arguments for endpoints get (got %d).", len(args)),
			"Usage: dwellir endpoints get <chain>",
		)
	}
	return nil
}

func rejectEndpointKeyFlag(cmd *cobra.Command) error {
	if !cmd.Flags().Changed("key") {
		return nil
	}
	return getFormatter().Error(
		"validation_error",
		"--key was removed: endpoint output never contains API keys.",
		"Endpoint URLs keep the <key> placeholder. Write the key with 'dwellir keys env <name>' and replace <key> "+
			"with $DWELLIR_API_KEY at runtime, or run 'dwellir project setup' to write full URLs to .env.",
	)
}

func applyPremiumEndpointAccess(client *api.Client, chains []api.Chain) []api.Chain {
	if len(chains) == 0 {
		return chains
	}

	info, err := api.NewAccountAPI(client).Info()
	if err != nil {
		return chains
	}

	return api.ApplyPremiumEndpointLabels(chains, info)
}

func endpointsHaveKeyPlaceholder(chains []api.Chain) bool {
	for _, chain := range chains {
		for _, network := range chain.Networks {
			for _, node := range network.Nodes {
				if strings.Contains(node.HTTPS, "<key>") || strings.Contains(node.WSS, "<key>") {
					return true
				}
			}
		}
	}
	return false
}
