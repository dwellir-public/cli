package cli

import (
	"errors"
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

const endpointAutoKeySentinel = "__AUTO__"

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
	Args:  endpointsListArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		selectorOverride, err := endpointOptionalKeySelectorFromArgs(cmd, args)
		if err != nil {
			return err
		}

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
		chains, err = applyEndpointKey(cmd, client, chains, selectorOverride)
		if err != nil {
			return formatEndpointKeyError(err)
		}
		f := getFormatter()
		return f.Success("endpoints.list", chains)
	},
}

var endpointsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search endpoints by chain or network name",
	Args:  endpointsSearchArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		selectorOverride, err := endpointOptionalKeySelectorFromArgs(cmd, args[1:])
		if err != nil {
			return err
		}

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
		chains, err = applyEndpointKey(cmd, client, chains, selectorOverride)
		if err != nil {
			return formatEndpointKeyError(err)
		}
		f := getFormatter()
		return f.Success("endpoints.search", chains)
	},
}

var endpointsGetCmd = &cobra.Command{
	Use:   "get <chain>",
	Short: "Get endpoints for a specific chain",
	Args:  endpointsGetArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		chainLookup := args[0]
		selectorOverride, err := endpointOptionalKeySelectorFromArgs(cmd, args[1:])
		if err != nil {
			return err
		}

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
		chains, err = applyEndpointKey(cmd, client, chains, selectorOverride)
		if err != nil {
			return formatEndpointKeyError(err)
		}
		f := getFormatter()
		if len(chains) == 0 {
			return f.Error("not_found", "No endpoints found for '"+chainLookup+"'.", "Run 'dwellir endpoints list' to see all available chains.")
		}
		return f.Success("endpoints.get", chains)
	},
}

func init() {
	endpointsCmd.PersistentFlags().StringVar(&epEcosystem, "ecosystem", "", "Filter by ecosystem (evm, substrate, cosmos, move)")
	endpointsCmd.PersistentFlags().StringVar(&epNodeType, "node-type", "", "Filter by node type (full, archive)")
	endpointsCmd.PersistentFlags().StringVar(&epProtocol, "protocol", "", "Filter by protocol (https, wss)")
	endpointsCmd.PersistentFlags().StringVar(&epNetwork, "network", "", "Filter by network (mainnet, testnet, or network name)")
	endpointsCmd.PersistentFlags().StringVar(&epKeyName, "key", "", "Insert API key into endpoint URLs (optional: pass key name or key value)")
	if keyFlag := endpointsCmd.PersistentFlags().Lookup("key"); keyFlag != nil {
		keyFlag.NoOptDefVal = endpointAutoKeySentinel
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

type endpointKeyError struct {
	message string
	help    string
}

func (e endpointKeyError) Error() string {
	return e.message
}

func applyEndpointKey(cmd *cobra.Command, client *api.Client, chains []api.Chain, selectorOverride string) ([]api.Chain, error) {
	if !cmd.Flags().Changed("key") || len(chains) == 0 {
		return chains, nil
	}

	keys, err := api.NewKeysAPI(client).List()
	if err != nil {
		return nil, err
	}

	selector := strings.TrimSpace(epKeyName)
	if strings.TrimSpace(selectorOverride) != "" {
		selector = strings.TrimSpace(selectorOverride)
	}
	selectedKey, err := selectEndpointKey(keys, selector)
	if err != nil {
		return nil, err
	}

	return injectEndpointKey(chains, selectedKey), nil
}

func endpointsListArgs(cmd *cobra.Command, args []string) error {
	_, err := endpointOptionalKeySelectorFromArgs(cmd, args)
	return err
}

func endpointsSearchArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return getFormatter().Error(
			"validation_error",
			"Missing required argument <query>.",
			"Example: dwellir endpoints search base --network mainnet",
		)
	}
	if len(args) > 2 {
		return getFormatter().Error(
			"validation_error",
			fmt.Sprintf("Too many arguments for endpoints search (got %d).", len(args)),
			"Usage: dwellir endpoints search <query> [--key [name]]",
		)
	}
	_, err := endpointOptionalKeySelectorFromArgs(cmd, args[1:])
	return err
}

func endpointsGetArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return getFormatter().Error(
			"validation_error",
			"Missing required argument <chain>.",
			"Example: dwellir endpoints get base --network mainnet",
		)
	}
	if len(args) > 2 {
		return getFormatter().Error(
			"validation_error",
			fmt.Sprintf("Too many arguments for endpoints get (got %d).", len(args)),
			"Usage: dwellir endpoints get <chain> [--key [name]]",
		)
	}
	_, err := endpointOptionalKeySelectorFromArgs(cmd, args[1:])
	return err
}

func endpointOptionalKeySelectorFromArgs(cmd *cobra.Command, args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}

	if len(args) > 1 {
		return "", getFormatter().Error(
			"validation_error",
			fmt.Sprintf("Unexpected arguments: %s", strings.Join(args, " ")),
			"Use --key with zero or one value.\nExamples:\n  dwellir endpoints get base --key\n  dwellir endpoints get base --key my-key",
		)
	}

	if !cmd.Flags().Changed("key") || epKeyName != endpointAutoKeySentinel {
		return "", getFormatter().Error(
			"validation_error",
			fmt.Sprintf("Unexpected argument: %s", args[0]),
			"Run `dwellir endpoints --help` to see valid syntax.",
		)
	}

	return strings.TrimSpace(args[0]), nil
}

func selectEndpointKey(keys []api.APIKey, selector string) (string, error) {
	selector = strings.TrimSpace(selector)
	if len(keys) == 0 {
		return "", endpointKeyError{
			message: "No API keys found to inject into endpoint URLs.",
			help:    "Run 'dwellir keys create --name <name>' to create a key first.",
		}
	}

	if selector == endpointAutoKeySentinel || selector == "" {
		if len(keys) == 1 {
			return keys[0].APIKey, nil
		}
		return "", endpointKeyError{
			message: fmt.Sprintf("Found %d API keys; please choose one with --key <name>.", len(keys)),
			help:    "Run 'dwellir keys list' to see available keys.",
		}
	}

	var matched []api.APIKey
	for _, key := range keys {
		if strings.EqualFold(strings.TrimSpace(key.Name), selector) || key.APIKey == selector {
			matched = append(matched, key)
		}
	}

	switch len(matched) {
	case 1:
		return matched[0].APIKey, nil
	case 0:
		return "", endpointKeyError{
			message: fmt.Sprintf("No API key matched %q.", selector),
			help:    "Run 'dwellir keys list' and pass a key name with --key <name>.",
		}
	default:
		return "", endpointKeyError{
			message: fmt.Sprintf("Multiple API keys matched %q; please use a unique key name.", selector),
			help:    "Run 'dwellir keys list' to choose a unique key name.",
		}
	}
}

func injectEndpointKey(chains []api.Chain, keyValue string) []api.Chain {
	for chainIdx := range chains {
		for netIdx := range chains[chainIdx].Networks {
			for nodeIdx := range chains[chainIdx].Networks[netIdx].Nodes {
				node := &chains[chainIdx].Networks[netIdx].Nodes[nodeIdx]
				node.HTTPS = strings.ReplaceAll(node.HTTPS, "<key>", keyValue)
				node.WSS = strings.ReplaceAll(node.WSS, "<key>", keyValue)
			}
		}
	}
	return chains
}

func formatEndpointKeyError(err error) error {
	var keyErr endpointKeyError
	if errors.As(err, &keyErr) {
		return getFormatter().Error("validation_error", keyErr.message, keyErr.help)
	}
	return formatCommandError(err)
}
