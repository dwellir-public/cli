package cli

import (
	"errors"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
	"github.com/dwellir-public/cli/internal/config"
)

type projectOptions struct {
	chain, network, nodeType, keyName, createKey, envFile string
	replace                                               bool
	dailyQuota, monthlyQuota                              int
}

func newProjectCommand() *cobra.Command {
	options := &projectOptions{}
	root := &cobra.Command{Use: "project", Short: "Configure a project without printing credentials"}
	setup := &cobra.Command{
		Use: "setup", Short: "Write a project key and endpoint URLs to an environment file", Args: cobra.NoArgs,
		RunE: options.run,
	}
	setup.Flags().StringVar(&options.chain, "chain", "", "Chain name or slug")
	setup.Flags().StringVar(&options.network, "network", "", "Network name")
	setup.Flags().StringVar(&options.nodeType, "node-type", "", "Endpoint node type")
	setup.Flags().StringVar(&options.keyName, "key-name", "", "Name of an existing enabled key")
	setup.Flags().StringVar(&options.createKey, "create-key", "", "Create a named key, or reuse the same enabled key")
	setup.Flags().StringVar(&options.envFile, "env-file", ".env", "Environment file: .env or .env.local")
	setup.Flags().BoolVar(&options.replace, "replace", false, "Replace existing Dwellir environment values")
	setup.Flags().IntVar(&options.dailyQuota, "daily-quota", 0, "Daily request quota for a new key")
	setup.Flags().IntVar(&options.monthlyQuota, "monthly-quota", 0, "Monthly request quota for a new key")
	root.AddCommand(setup)
	return root
}

func (options *projectOptions) validate(cmd *cobra.Command) error {
	options.keyName = strings.TrimSpace(options.keyName)
	options.createKey = strings.TrimSpace(options.createKey)
	if options.chain == "" || options.network == "" || (options.keyName == "") == (options.createKey == "") {
		return getFormatter().Error("validation_error", "Specify --chain, --network, and exactly one of --key-name or --create-key.", "Run dwellir project setup --help")
	}
	if options.dailyQuota < 0 || options.monthlyQuota < 0 {
		return getFormatter().Error("validation_error", "Quotas cannot be negative.", "")
	}
	if options.createKey == "" && (cmd.Flags().Changed("daily-quota") || cmd.Flags().Changed("monthly-quota")) {
		return getFormatter().Error("validation_error", "Quota flags apply only to --create-key.", "")
	}
	return nil
}

func (options *projectOptions) run(cmd *cobra.Command, _ []string) error {
	if err := options.validate(cmd); err != nil {
		return err
	}

	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := validateUntrackedEnv(dir, options.envFile); err != nil {
		return getFormatter().Error("validation_error", err.Error(), "")
	}
	if err := config.ValidateProjectEnv(dir, options.envFile, options.replace); err != nil {
		return getFormatter().Error("configuration_error", err.Error(), "")
	}
	client, err := newAPIClient()
	if err != nil {
		return formatCommandError(err)
	}
	chains, err := api.NewEndpointsAPI(client).Get(options.chain, "", options.nodeType, "", options.network)
	if err != nil {
		return getFormatter().Error("api_error", "Could not retrieve endpoints.", "")
	}
	node, err := projectNode(chains)
	if err != nil {
		return getFormatter().Error("validation_error", err.Error(), "Choose an exact network and --node-type.")
	}
	key, err := projectKey(api.NewKeysAPI(client), options.keyName, options.createKey, options.dailyQuota, options.monthlyQuota)
	if err != nil {
		return getFormatter().Error("api_error", err.Error(), "")
	}
	values := projectEnv(node, key)
	if err := config.WriteProjectEnv(dir, options.envFile, values, options.replace); err != nil {
		return getFormatter().Error("configuration_error", err.Error(), "")
	}
	return getFormatter().Success("project.setup", map[string]interface{}{"file": options.envFile, "configured": true, "https": node.HTTPS != "", "wss": node.WSS != ""})
}

func projectNode(chains []api.Chain) (api.Node, error) {
	var nodes []api.Node
	for _, chain := range chains {
		for _, network := range chain.Networks {
			nodes = append(nodes, network.Nodes...)
		}
	}
	if len(nodes) != 1 {
		return api.Node{}, errors.New("select exactly one endpoint")
	}
	node := nodes[0]
	if node.HTTPS == "" && node.WSS == "" {
		return api.Node{}, errors.New("the endpoint has no supported connection URL")
	}
	if err := validateProjectURL(node.HTTPS, "https"); err != nil {
		return api.Node{}, err
	}
	if err := validateProjectURL(node.WSS, "wss"); err != nil {
		return api.Node{}, err
	}

	return node, nil
}

func projectKey(keys *api.KeysAPI, name, create string, daily, monthly int) (string, error) {
	if create != "" {
		name = create
	}
	all, err := keys.List()
	if err != nil {
		return "", errors.New("could not list project keys")
	}
	var matches []api.APIKey
	for _, key := range all {
		if key.Name == name {
			matches = append(matches, key)
		}
	}
	if len(matches) > 1 {
		return "", errors.New("multiple keys have that name; choose a unique key name")
	}
	if len(matches) == 1 {
		return existingProjectKey(matches[0], create, daily, monthly)
	}

	if create == "" {
		return "", errors.New("no key has that name")
	}
	return createProjectKey(keys, name, daily, monthly)
}

func createProjectKey(keys *api.KeysAPI, name string, daily, monthly int) (string, error) {
	input := api.CreateKeyInput{Name: name}
	if daily > 0 {
		input.DailyQuota = &daily
	}
	if monthly > 0 {
		input.MonthlyQuota = &monthly
	}
	key, err := keys.Create(input)
	if err != nil {
		return "", errors.New("could not create project key; check account permissions")
	}
	if key.APIKey == "" {
		return "", errors.New("the account returned an empty key")
	}
	return key.APIKey, nil
}

func init() { rootCmd.AddCommand(newProjectCommand()) }

func validateUntrackedEnv(dir, filename string) error {
	repository := exec.Command("git", "rev-parse", "--show-toplevel")
	repository.Dir = dir
	repository.Env = append(os.Environ(), "LC_ALL=C")
	output, err := repository.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "fatal: not a git repository") {
			return nil
		}
		return errors.New("cannot verify Git tracking; check Git installation and repository access")
	}
	tracked := exec.Command("git", "ls-files", "--error-unmatch", "--", filename)
	tracked.Dir = dir
	err = tracked.Run()
	if err == nil {
		return errors.New("the environment file is tracked by Git; untrack it before storing credentials")
	}
	var status *exec.ExitError
	if errors.As(err, &status) && status.ExitCode() == 1 {
		return nil
	}
	return errors.New("cannot verify whether Git tracks the environment file")
}

func validateProjectURL(endpoint, scheme string) error {
	if endpoint == "" {
		return nil
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return errors.New("invalid endpoint URL")
	}
	if parsed.Scheme != scheme || !strings.HasSuffix(parsed.Hostname(), ".dwellir.com") || parsed.User != nil {
		return errors.New("the endpoint must be an authenticated Dwellir URL")
	}
	if !projectURLComponentsValid(parsed) {
		return errors.New("unsupported endpoint URL components")
	}
	return nil
}

func existingProjectKey(key api.APIKey, create string, daily, monthly int) (string, error) {
	if !key.Enabled {
		return "", errors.New("the selected key is disabled")
	}
	if create != "" && (!quotaMatches(daily, key.DailyQuota) || !quotaMatches(monthly, key.MonthlyQuota)) {
		return "", errors.New("existing key quotas differ; update the key explicitly before setup")
	}
	if key.APIKey == "" {
		return "", errors.New("the account returned an empty key")
	}
	return key.APIKey, nil
}

func quotaMatches(requested int, configured *int) bool {
	if requested == 0 {
		return true
	}
	return configured != nil && *configured == requested
}

func projectEnv(node api.Node, key string) map[string]string {
	values := map[string]string{"DWELLIR_API_KEY": key, "DWELLIR_RPC_URL": "", "DWELLIR_WSS_URL": ""}
	if node.HTTPS != "" {
		values["DWELLIR_RPC_URL"] = strings.ReplaceAll(node.HTTPS, "<key>", key)
	}
	if node.WSS != "" {
		values["DWELLIR_WSS_URL"] = strings.ReplaceAll(node.WSS, "<key>", key)
	}
	return values
}

func projectURLComponentsValid(parsed *url.URL) bool {
	return parsed.RawQuery == "" && parsed.Fragment == "" && (parsed.Port() == "" || parsed.Port() == "443") && strings.Contains(parsed.Path, "<key>")
}
