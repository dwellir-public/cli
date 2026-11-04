package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
	"github.com/dwellir-public/cli/internal/output"
)

var (
	whitelistName        string
	whitelistDescription string
	whitelistIPs         []string
	whitelistKeys        []string
	whitelistAllKeys     bool
	whitelistDisabled    bool
	whitelistDeleteYes   bool
)

const whitelistEnforcedNotice = "An enabled whitelist limits its API keys to the listed addresses. " +
	"Requests that use those keys from any other address fail."

const whitelistNoKeysNotice = "The whitelist restricts no API key yet. " +
	"Run 'dwellir whitelists assign <whitelist> <key>' to add one, or 'dwellir whitelists update <whitelist> --all-keys' to restrict every key."

const whitelistDeletedNotice = "Keys that only this whitelist restricted now accept requests from any address."

var whitelistsCmd = &cobra.Command{
	Use:     "whitelists",
	Aliases: []string{"whitelist"},
	Short:   "Manage IP whitelists for API keys",
	Long: `Manage IP whitelists for API keys.

An enabled whitelist limits its API keys to the listed IP addresses.
Requests that use those keys from any other address fail. A whitelist
applies to all keys (--all-keys) or to the keys you assign to it.

Select a whitelist by ID or by name. Select a key by name, by masked
value or by full value.

Example:
  dwellir whitelists list
  dwellir whitelists create --name office --ip 203.0.113.10 --ip 2001:db8::5 --key ci-key
  dwellir whitelists assign office prod-key
  dwellir whitelists disable office`,
}

var whitelistsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List IP whitelists",
	Args:  noArgs("dwellir whitelists list"),
	RunE: withRenderedExitCodes(func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		whitelists, err := api.NewWhitelistsAPI(client).List()
		if err != nil {
			return formatCommandError(err)
		}
		if whitelists == nil {
			whitelists = []api.Whitelist{}
		}
		for i := range whitelists {
			nameWhitelistKeys(client, &whitelists[i])
		}
		return getFormatter().Success("whitelists.list", whitelists)
	}),
}

var whitelistsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an IP whitelist",
	Long: `Create an IP whitelist.

Pass each address with --ip. Only single addresses are supported, as in the
dashboard: an IPv4 address becomes /32 and an IPv6 address becomes /128.

Pass --all-keys to restrict every API key, including keys created later, or
--key once for each key to restrict. Without either, the whitelist restricts
no key until you assign one.

A whitelist for specific keys is created disabled, gets its keys, and is
then enabled. If a step fails, the error lists the commands that finish
the setup.

Example:
  dwellir whitelists create --name office --ip 203.0.113.10 --ip 2001:db8::5 --all-keys
  dwellir whitelists create --name ci --ip 198.51.100.7 --key ci-key --key deploy-key
  dwellir whitelists create --name staging --ip 192.0.2.1 --disabled`,
	Args: noArgs(`dwellir whitelists create --name office --ip 203.0.113.10 --all-keys`),
	RunE: withRenderedExitCodes(runWhitelistsCreate),
}

func runWhitelistsCreate(cmd *cobra.Command, args []string) error {
	example := "Example: dwellir whitelists create --name office --ip 203.0.113.10 --all-keys"
	name := strings.TrimSpace(whitelistName)
	if name == "" {
		return getFormatter().Error("validation_error", "Missing required flag --name.", example)
	}
	if whitelistAllKeys && len(whitelistKeys) > 0 {
		return getFormatter().Error("validation_error", "--all-keys and --key cannot be used together.",
			"Pass --all-keys to restrict every key, or --key for each key to restrict.\n"+example)
	}
	entries, err := whitelistEntries(whitelistIPs)
	if err != nil {
		return err
	}
	// Brian refuses an enabled or account-wide whitelist without entries, and
	// refuses to assign a key to one.
	if len(entries) == 0 && (!whitelistDisabled || whitelistAllKeys || len(whitelistKeys) > 0) {
		return getFormatter().Error("validation_error", "Missing required flag --ip.",
			"Pass --ip once for each address. Only a disabled whitelist without keys can have no addresses.\n"+example)
	}

	client, err := newAPIClient()
	if err != nil {
		return getFormatter().Error("not_authenticated", err.Error(), "")
	}
	// Resolve every key before writing anything, so a wrong name creates nothing.
	keys, err := resolveWhitelistKeys(client, whitelistKeys)
	if err != nil {
		return err
	}

	enabled := !whitelistDisabled
	// Like the dashboard, keep a whitelist for specific keys disabled until
	// every key is assigned, so a failed step never enforces half a setup.
	staged := enabled && len(keys) > 0
	input := api.CreateWhitelistInput{
		Name:              name,
		Entries:           entries,
		ApplyToAllAPIKeys: whitelistAllKeys,
		Enabled:           enabled && !staged,
	}
	if cmd.Flags().Changed("description") {
		input.Description = &whitelistDescription
	}
	whitelistsAPI := api.NewWhitelistsAPI(client)
	created, err := whitelistsAPI.Create(input)
	if err != nil {
		// A 4xx means Marly refused it. After a 5xx or a lost response the
		// whitelist may exist, and a blind retry would create a second one.
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode < 500 {
			return formatCommandError(err)
		}
		return formatWhitelistStepError(err, fmt.Sprintf("Could not confirm that IP whitelist %q was created.", name),
			"It may exist anyway. Run 'dwellir whitelists list' before retrying, so you do not create it twice.")
	}

	whitelist := created
	for i, key := range keys {
		assigned, err := whitelistsAPI.Assign(created.ID, key.APIKey)
		if err != nil {
			return formatWhitelistStepError(err, fmt.Sprintf(
				"Created IP whitelist %q (ID %d) disabled, but could not assign key %q.", created.Name, created.ID, key.Name),
				whitelistRecoveryHelp(created.ID, keys[i:], enabled))
		}
		whitelist = assigned
	}
	if staged {
		activated, err := whitelistsAPI.SetEnabled(created.ID, true)
		if err != nil {
			return formatWhitelistStepError(err, fmt.Sprintf(
				"Created IP whitelist %q (ID %d) and assigned its keys, but could not confirm that it is enabled.", created.Name, created.ID),
				whitelistRecoveryHelp(created.ID, nil, true))
		}
		whitelist = activated
	}
	nameWhitelistKeys(client, whitelist)
	return getFormatter().Success("whitelists.create", whitelistWithNotice(*whitelist))
}

func whitelistRecoveryHelp(id int64, unassigned []api.APIKey, enable bool) string {
	var steps []string
	for _, key := range unassigned {
		steps = append(steps, fmt.Sprintf("dwellir whitelists assign %d %s", id, quoteArg(key.Name)))
	}
	if enable {
		steps = append(steps, fmt.Sprintf("dwellir whitelists enable %d", id))
	}
	if len(steps) == 0 {
		return ""
	}
	return "To finish, run: " + strings.Join(steps, "; then ") + "."
}

var whitelistsUpdateCmd = &cobra.Command{
	Use:   "update <whitelist>",
	Short: "Change the name, description, addresses or key scope of an IP whitelist",
	Long: `Change the name, description, addresses or key scope of an IP whitelist.

--ip replaces every address of the whitelist. Pass it once for each address
to keep. --all-keys restricts every API key; --all-keys=false makes the
whitelist restrict only the keys assigned to it. Assign those keys before
--all-keys=false, so they stay restricted without a gap.

Example:
  dwellir whitelists update office --name hq
  dwellir whitelists update office --ip 203.0.113.10 --ip 203.0.113.11
  dwellir whitelists update office --all-keys=false`,
	Args: whitelistArgs("dwellir whitelists update office --name hq"),
	RunE: withRenderedExitCodes(func(cmd *cobra.Command, args []string) error {
		input := api.UpdateWhitelistInput{}
		if cmd.Flags().Changed("name") {
			name := strings.TrimSpace(whitelistName)
			if name == "" {
				return getFormatter().Error("validation_error", "--name must not be empty.",
					"Example: dwellir whitelists update office --name hq")
			}
			input.Name = &name
		}
		if cmd.Flags().Changed("description") {
			input.Description = &whitelistDescription
		}
		if cmd.Flags().Changed("ip") {
			entries, err := whitelistEntries(whitelistIPs)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				return getFormatter().Error("validation_error", "--ip needs at least one address.",
					"Pass --ip once for each address to keep, for example: dwellir whitelists update office --ip 203.0.113.10")
			}
			input.Entries = entries
		}
		if cmd.Flags().Changed("all-keys") {
			input.ApplyToAllAPIKeys = &whitelistAllKeys
		}
		if input.Name == nil && input.Description == nil && input.Entries == nil && input.ApplyToAllAPIKeys == nil {
			return getFormatter().Error("validation_error", "Nothing to update.",
				"Pass --name, --description, --ip or --all-keys. To turn a whitelist on or off, run 'dwellir whitelists enable' or 'dwellir whitelists disable'.")
		}
		return updateWhitelist(args[0], "whitelists.update", input)
	}),
}

var whitelistsEnableCmd = &cobra.Command{
	Use:   "enable <whitelist>",
	Short: "Enable an IP whitelist so it restricts its keys",
	Args:  whitelistArgs("dwellir whitelists enable office"),
	RunE: withRenderedExitCodes(func(cmd *cobra.Command, args []string) error {
		enabled := true
		return updateWhitelist(args[0], "whitelists.enable", api.UpdateWhitelistInput{Enabled: &enabled})
	}),
}

var whitelistsDisableCmd = &cobra.Command{
	Use:   "disable <whitelist>",
	Short: "Disable an IP whitelist so it no longer restricts its keys",
	Args:  whitelistArgs("dwellir whitelists disable office"),
	RunE: withRenderedExitCodes(func(cmd *cobra.Command, args []string) error {
		enabled := false
		return updateWhitelist(args[0], "whitelists.disable", api.UpdateWhitelistInput{Enabled: &enabled})
	}),
}

func updateWhitelist(selector, command string, input api.UpdateWhitelistInput) error {
	client, err := newAPIClient()
	if err != nil {
		return getFormatter().Error("not_authenticated", err.Error(), "")
	}
	current, err := findWhitelist(client, selector)
	if err != nil {
		return err
	}
	updated, err := api.NewWhitelistsAPI(client).Update(current.ID, input)
	if err != nil {
		return formatCommandError(err)
	}
	nameWhitelistKeys(client, updated)
	return getFormatter().Success(command, whitelistWithNotice(*updated))
}

// whitelistWithNotice says what the whitelist now does to requests.
func whitelistWithNotice(whitelist api.Whitelist) interface{} {
	switch {
	case !whitelist.ApplyToAllAPIKeys && !whitelistHasKeys(whitelist):
		return output.Notice{Data: whitelist, Text: whitelistNoKeysNotice}
	case whitelist.Enabled:
		return output.Notice{Data: whitelist, Text: whitelistEnforcedNotice}
	default:
		return whitelist
	}
}

func whitelistHasKeys(whitelist api.Whitelist) bool {
	for _, assignment := range whitelist.AssignedAPIKeys {
		if assignment.Enabled {
			return true
		}
	}
	return false
}

var whitelistsAssignCmd = &cobra.Command{
	Use:   "assign <whitelist> <key>",
	Short: "Restrict an API key with an IP whitelist",
	Args:  whitelistKeyArgs("dwellir whitelists assign office ci-key"),
	RunE: withRenderedExitCodes(func(cmd *cobra.Command, args []string) error {
		return changeWhitelistKey(args[0], args[1], true)
	}),
}

var whitelistsUnassignCmd = &cobra.Command{
	Use:   "unassign <whitelist> <key>",
	Short: "Remove an API key from an IP whitelist",
	Args:  whitelistKeyArgs("dwellir whitelists unassign office ci-key"),
	RunE: withRenderedExitCodes(func(cmd *cobra.Command, args []string) error {
		return changeWhitelistKey(args[0], args[1], false)
	}),
}

func changeWhitelistKey(whitelistSelector, keySelector string, assign bool) error {
	client, err := newAPIClient()
	if err != nil {
		return getFormatter().Error("not_authenticated", err.Error(), "")
	}
	whitelist, err := findWhitelist(client, whitelistSelector)
	if err != nil {
		return err
	}
	key, err := findKey(client, keySelector)
	if err != nil {
		return err
	}
	whitelistsAPI := api.NewWhitelistsAPI(client)
	command, call := "whitelists.unassign", whitelistsAPI.Unassign
	if assign {
		command, call = "whitelists.assign", whitelistsAPI.Assign
	}
	updated, err := call(whitelist.ID, key.APIKey)
	if err != nil {
		return formatCommandError(err)
	}
	nameWhitelistKeys(client, updated)
	// Brian keeps manual assignments of an account-wide whitelist, and they
	// take over after --all-keys=false. Assigning first leaves no gap.
	if updated.ApplyToAllAPIKeys {
		return getFormatter().Success(command, output.Notice{Data: *updated, Text: fmt.Sprintf(
			"This whitelist applies to all API keys, so every key stays restricted. "+
				"After 'dwellir whitelists update %d --all-keys=false', only the assigned keys stay restricted.", updated.ID)})
	}
	return getFormatter().Success(command, whitelistWithNotice(*updated))
}

var whitelistsDeleteCmd = &cobra.Command{
	Use:   "delete <whitelist>",
	Short: "Delete an IP whitelist",
	Long: `Delete an IP whitelist.

Keys that only this whitelist restricted then accept requests from any
address. Deleting cannot be undone.

Deleting needs confirmation: pass --yes, or pass --human in a terminal to
be asked first. Without either, the command deletes nothing.

Example:
  dwellir whitelists delete office --human
  dwellir whitelists delete 42 --yes --json`,
	Args: whitelistArgs("dwellir whitelists delete office --yes"),
	RunE: withRenderedExitCodes(func(cmd *cobra.Command, args []string) error {
		// Same rule as keys rotate: prompt only in an explicit --human run.
		canPrompt := humanOutput && isHumanOutput() && stdinIsTerminal()
		if !whitelistDeleteYes && !canPrompt {
			return getFormatter().Error("validation_error",
				"Deleting an IP whitelist needs confirmation.",
				"Keys that only this whitelist restricted then accept requests from any address. "+
					"Pass --yes to confirm, or --human in a terminal to be asked, for example: dwellir whitelists delete <whitelist> --yes")
		}
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		whitelist, err := findWhitelist(client, args[0])
		if err != nil {
			return err
		}
		result := map[string]interface{}{"id": whitelist.ID, "name": whitelist.Name}
		if !whitelistDeleteYes {
			confirmed, err := confirmPrompt(cmd, fmt.Sprintf(
				"Delete IP whitelist %q (ID %d)? Keys that only it restricted then accept requests from any address. [y/N]: ",
				whitelist.Name, whitelist.ID))
			if err != nil {
				return formatCommandError(err)
			}
			if !confirmed {
				result["status"] = "cancelled"
				return getFormatter().Success("whitelists.delete", result)
			}
		}
		if err := api.NewWhitelistsAPI(client).Delete(whitelist.ID); err != nil {
			return formatCommandError(err)
		}
		result["status"] = "deleted"
		return getFormatter().Success("whitelists.delete", output.Notice{Data: result, Text: whitelistDeletedNotice})
	}),
}

// whitelistEntries validates and normalizes --ip values. Duplicates are an
// error, as in the dashboard.
func whitelistEntries(values []string) ([]string, error) {
	entries := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		entry, err := api.NormalizeWhitelistAddress(value)
		if err != nil {
			return nil, getFormatter().Error("validation_error", "Invalid --ip: "+err.Error()+".",
				"Pass one IPv4 or IPv6 address per --ip, for example: --ip 203.0.113.10 --ip 2001:db8::5")
		}
		if seen[entry] {
			return nil, getFormatter().Error("validation_error", fmt.Sprintf("Duplicate --ip: %s.", entry),
				"Pass each address once.")
		}
		seen[entry] = true
		entries = append(entries, entry)
	}
	return entries, nil
}

// resolveWhitelistKeys resolves --key selectors with one key list request.
func resolveWhitelistKeys(client *api.Client, selectors []string) ([]api.APIKey, error) {
	if len(selectors) == 0 {
		return nil, nil
	}
	all, err := api.NewKeysAPI(client).List()
	if err != nil {
		return nil, formatCommandError(err)
	}
	var keys []api.APIKey
	seen := map[api.Secret]bool{}
	for _, selector := range selectors {
		key, err := api.FindKey(all, selector)
		if err != nil {
			return nil, formatKeyLookupError(err)
		}
		if !seen[key.APIKey] {
			seen[key.APIKey] = true
			keys = append(keys, key)
		}
	}
	return keys, nil
}

// nameWhitelistKeys fills key names from the key list. Marly sends only key
// values, and a masked value alone is hard to recognise. The names are best
// effort: when the key list fails, keys show by masked value.
func nameWhitelistKeys(client *api.Client, whitelists ...*api.Whitelist) {
	hasKeys := false
	for _, whitelist := range whitelists {
		hasKeys = hasKeys || len(whitelist.AssignedAPIKeys) > 0
	}
	if !hasKeys {
		return
	}
	names := keyNames(client)
	for _, whitelist := range whitelists {
		for i := range whitelist.AssignedAPIKeys {
			whitelist.AssignedAPIKeys[i].Name = names[whitelist.AssignedAPIKeys[i].APIKey]
		}
	}
}

func findWhitelist(client *api.Client, selector string) (api.Whitelist, error) {
	whitelist, err := api.NewWhitelistsAPI(client).Find(selector)
	var lookupErr *api.WhitelistLookupError
	if !errors.As(err, &lookupErr) {
		if err != nil {
			return api.Whitelist{}, formatCommandError(err)
		}
		return whitelist, nil
	}
	if lookupErr.Ambiguous {
		return api.Whitelist{}, getFormatter().Error("validation_error", lookupErr.Error(),
			"Matching whitelists: "+strings.Join(lookupErr.Candidates, ", ")+". Select one by its ID.")
	}
	return api.Whitelist{}, getFormatter().Error("not_found", lookupErr.Error(),
		"Pass a whitelist ID or name. Run 'dwellir whitelists list' to see them.")
}

// formatWhitelistStepError reports a step that failed after the whitelist
// was created, with the commands that finish the setup.
func formatWhitelistStepError(err error, message, recovery string) error {
	code, _, help := commandErrorParts(err)
	if recovery != "" {
		help = recovery + "\n" + help
	}
	return getFormatter().Error(code, message, help)
}

func noArgs(example string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		return getFormatter().Error("validation_error",
			fmt.Sprintf("Unexpected arguments for %s (got %d).", cmd.CommandPath(), len(args)), "Example: "+example)
	}
}

func whitelistArgs(example string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return nil
		}
		message := "Missing required argument <whitelist>."
		if len(args) > 1 {
			message = fmt.Sprintf("Expected one whitelist, got %d arguments.", len(args))
		}
		return getFormatter().Error("validation_error", message,
			"Pass a whitelist ID or name. Run 'dwellir whitelists list' to see them.\nExample: "+example)
	}
}

func whitelistKeyArgs(example string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 2 {
			return nil
		}
		return getFormatter().Error("validation_error",
			fmt.Sprintf("Expected <whitelist> and <key>, got %d arguments.", len(args)),
			"Pass a whitelist ID or name, then a key name or masked key. "+
				"Run 'dwellir whitelists list' and 'dwellir keys list' to see them.\nExample: "+example)
	}
}

// quoteArg quotes a value for a shell command in help text when it needs it.
func quoteArg(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t'\"\\$`*?![]{}()<>|&;#~") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// isWhitelistsCommand reports whether cmd belongs to the whitelists group.
func isWhitelistsCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c == whitelistsCmd {
			return true
		}
	}
	return false
}

// addWhitelistCreateFlags and addWhitelistUpdateFlags register the flags;
// tests call them again after ResetFlags, because slice flags keep state.
func addWhitelistCreateFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&whitelistName, "name", "", "Whitelist name (required)")
	cmd.Flags().StringVar(&whitelistDescription, "description", "", "Description")
	cmd.Flags().StringSliceVar(&whitelistIPs, "ip", nil, "IPv4 or IPv6 address to allow (repeatable)")
	cmd.Flags().BoolVar(&whitelistAllKeys, "all-keys", false, "Restrict every API key, including keys created later")
	cmd.Flags().StringArrayVar(&whitelistKeys, "key", nil, "API key to restrict, by name or masked value (repeatable)")
	cmd.Flags().BoolVar(&whitelistDisabled, "disabled", false, "Create the whitelist disabled")
}

func addWhitelistUpdateFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&whitelistName, "name", "", "New name")
	cmd.Flags().StringVar(&whitelistDescription, "description", "", "New description")
	cmd.Flags().StringSliceVar(&whitelistIPs, "ip", nil, "Address to allow; replaces every address (repeatable)")
	cmd.Flags().BoolVar(&whitelistAllKeys, "all-keys", false, "Restrict every API key (--all-keys=false: only assigned keys)")
}

func addWhitelistDeleteFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&whitelistDeleteYes, "yes", "y", false, "Skip the confirmation prompt")
}

func init() {
	addWhitelistCreateFlags(whitelistsCreateCmd)
	addWhitelistUpdateFlags(whitelistsUpdateCmd)
	addWhitelistDeleteFlags(whitelistsDeleteCmd)

	whitelistsCmd.AddCommand(whitelistsListCmd, whitelistsCreateCmd, whitelistsUpdateCmd, whitelistsEnableCmd,
		whitelistsDisableCmd, whitelistsAssignCmd, whitelistsUnassignCmd, whitelistsDeleteCmd)
	rootCmd.AddCommand(whitelistsCmd)
}
