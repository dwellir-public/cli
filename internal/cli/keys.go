package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/dwellir-public/cli/internal/api"
	"github.com/dwellir-public/cli/internal/config"
	"github.com/dwellir-public/cli/internal/output"
)

var (
	keyName         string
	keyDailyQuota   int
	keyMonthlyQuota int
	keyEnvFile      string
	keyEnvVar       string
	keyEnvReplace   bool
	keyRotateYes    bool
)

// stdinIsTerminal reports whether keys rotate can ask for confirmation.
var stdinIsTerminal = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

var envVarName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage API keys",
	Long: `Manage API keys.

The CLI never prints a full API key. Output shows keys masked, for example
abcd****wxyz. Select a key by name, by masked value or by full value.

To use a key:
  dwellir keys copy <name>    Copy the key to the clipboard
  dwellir keys env <name>     Write the key to .env
  dwellir project setup       Write the key and endpoint URLs to .env`,
}

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		keys, err := api.NewKeysAPI(client).List()
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("keys.list", withNotice(keys, len(keys) > 0, maskedKeysNotice))
	},
}

var keysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return getFormatter().Error(
				"validation_error",
				fmt.Sprintf("Unexpected arguments for keys create (got %d).", len(args)),
				"Example: dwellir keys create --name \"CI key\"",
			)
		}
		if !cmd.Flags().Changed("name") || strings.TrimSpace(keyName) == "" {
			return getFormatter().Error(
				"validation_error",
				"Missing required flag --name.",
				"Example: dwellir keys create --name \"CI key\"",
			)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		input := api.CreateKeyInput{Name: strings.TrimSpace(keyName)}
		if keyDailyQuota > 0 {
			input.DailyQuota = &keyDailyQuota
		}
		if keyMonthlyQuota > 0 {
			input.MonthlyQuota = &keyMonthlyQuota
		}
		key, err := api.NewKeysAPI(client).Create(input)
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("keys.create", output.Notice{Data: key, Text: maskedKeysNotice})
	},
}

var keysUpdateCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update an API key",
	Args:  keyNameArg(`dwellir keys update old-name --name "My Key"`),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		current, err := findKey(client, args[0])
		if err != nil {
			return err
		}
		input := api.UpdateKeyInput{}
		if cmd.Flags().Changed("name") {
			input.Name = &keyName
		}
		if cmd.Flags().Changed("daily-quota") {
			input.DailyQuota = &keyDailyQuota
		}
		if cmd.Flags().Changed("monthly-quota") {
			input.MonthlyQuota = &keyMonthlyQuota
		}
		key, err := api.NewKeysAPI(client).Update(current, input)
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("keys.update", output.Notice{Data: key, Text: maskedKeysNotice})
	},
}

var keysDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete an API key",
	Args:  keyNameArg("dwellir keys delete ci-key"),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		key, err := findKey(client, args[0])
		if err != nil {
			return err
		}
		result, err := api.NewKeysAPI(client).Delete(key.APIKey)
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("keys.delete", output.Notice{Data: deleteSuccessPayload(key, result), Text: deletedKeyNotice})
	},
}

func deleteSuccessPayload(key api.APIKey, result *api.DeleteKeyResult) map[string]interface{} {
	payload := map[string]interface{}{"key": key.APIKey.Masked(), "name": key.Name, "status": "deleted"}
	if result != nil && result.CleanupPending {
		payload["status"] = "cleanup_pending"
	}
	return payload
}

const rotatedKeyNotice = "The old key value no longer works. Update every client that used it. " +
	"The new value is masked and is never printed. To use it, run 'dwellir keys copy <name>' to copy it to the clipboard, " +
	"or 'dwellir keys env <name> --replace' to update DWELLIR_API_KEY in .env."

var keysRotateCmd = &cobra.Command{
	Use:   "rotate <name>",
	Short: "Give an API key a new value",
	Long: `Give an API key a new value.

The old value stops working at once. Every client that still uses it fails
until you give it the new value. The key keeps its name, quotas, enabled
state and IP whitelists.

Rotating needs confirmation: pass --yes, or pass --human in a terminal
to be asked first. Without either, the command rotates nothing.

The CLI prints the new value masked. To use it:
  dwellir keys copy <name>             Copy the new key to the clipboard
  dwellir keys env <name> --replace    Update DWELLIR_API_KEY in .env

Example:
  dwellir keys rotate ci-key
  dwellir keys rotate ci-key --yes --json`,
	Args: keyNameArg("dwellir keys rotate ci-key"),
	RunE: withRenderedExitCodes(func(cmd *cobra.Command, args []string) error {
		// AGENTS.md allows a prompt only in an explicit --human run. Human is
		// also the default format, so the flag itself must be set: a plain
		// run never waits for input.
		canPrompt := humanOutput && isHumanOutput() && stdinIsTerminal()
		if !keyRotateYes && !canPrompt {
			return getFormatter().Error("validation_error",
				"Rotating a key needs confirmation.",
				"The current key value stops working at once, and every client that still uses it fails. "+
					"Pass --yes to confirm, or --human in a terminal to be asked, for example: dwellir keys rotate <name> --yes")
		}
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		key, err := findKey(client, args[0])
		if err != nil {
			return err
		}
		if !keyRotateYes {
			confirmed, err := confirmRotate(cmd, key)
			if err != nil {
				return formatCommandError(err)
			}
			if !confirmed {
				return getFormatter().Success("keys.rotate", map[string]interface{}{
					"name": key.Name, "key": key.APIKey.Masked(), "status": "cancelled",
				})
			}
		}
		rotated, err := api.NewKeysAPI(client).Rotate(key.APIKey)
		if err != nil {
			return formatRotateError(err, key)
		}
		return getFormatter().Success("keys.rotate", output.Notice{Data: rotated, Text: rotatedKeyNotice})
	}),
}

func confirmRotate(cmd *cobra.Command, key api.APIKey) (bool, error) {
	return confirmPrompt(cmd, fmt.Sprintf(
		"Rotate API key %q (%s)? The current value stops working at once, and every client that still uses it will fail. [y/N]: ",
		key.Name, key.APIKey.Masked()))
}

// confirmPrompt writes prompt to stderr and reports whether the answer is yes.
// An empty answer or end of input means no.
func confirmPrompt(cmd *cobra.Command, prompt string) (bool, error) {
	_, _ = fmt.Fprint(cmd.ErrOrStderr(), prompt)
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// formatRotateError reports a failed rotation. A 4xx answer means the API
// refused the request, so the key did not change. Anything else (a 5xx, a
// lost response, a success without the new value) may come after the key
// rotated, so the hint says to check the key list instead of retrying: a
// retry by name would rotate the new value too.
func formatRotateError(err error, key api.APIKey) error {
	var apiErr *api.APIError
	isAPIErr := errors.As(err, &apiErr)
	if isAPIErr && apiErr.StatusCode < 500 {
		return formatCommandError(err)
	}
	code, message := "error", err.Error()
	switch {
	case isAPIErr:
		code, message = "api_error", fmt.Sprintf("Request failed with HTTP %d.", apiErr.StatusCode)
	case strings.HasPrefix(err.Error(), "request failed"):
		code = "network_error"
	}
	return getFormatter().Error(code, message, fmt.Sprintf(
		"The key may have rotated anyway. Do not retry yet: run 'dwellir keys list' and check whether %q still shows %s. "+
			"A different value means the rotation happened; use 'dwellir keys copy %s' to get it.",
		key.Name, key.APIKey.Masked(), key.Name))
}

var keysEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Enable an API key",
	Args:  keyNameArg("dwellir keys enable ci-key"),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		current, err := findKey(client, args[0])
		if err != nil {
			return err
		}
		key, err := api.NewKeysAPI(client).Enable(current)
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("keys.enable", output.Notice{Data: key, Text: maskedKeysNotice})
	},
}

var keysDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Disable an API key",
	Args:  keyNameArg("dwellir keys disable ci-key"),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		current, err := findKey(client, args[0])
		if err != nil {
			return err
		}
		key, err := api.NewKeysAPI(client).Disable(current)
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("keys.disable", output.Notice{Data: key, Text: maskedKeysNotice})
	},
}

var keysCopyCmd = &cobra.Command{
	Use:   "copy <name>",
	Short: "Copy an API key to the clipboard without printing it",
	Args:  keyNameArg("dwellir keys copy ci-key"),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		key, err := findKey(client, args[0])
		if err != nil {
			return err
		}
		tool, err := copyToClipboard(key.APIKey.Reveal())
		if err != nil {
			return getFormatter().Error("clipboard_unavailable", "Could not copy the key: "+err.Error()+".", clipboardHelp)
		}
		return getFormatter().Success("keys.copy", map[string]interface{}{
			"name": key.Name, "key": key.APIKey.Masked(), "copied_with": tool,
		})
	},
}

var keysEnvCmd = &cobra.Command{
	Use:   "env <name>",
	Short: "Write an API key to an environment file without printing it",
	Long: `Write an API key to an environment file without printing it.

The file gets mode 0600 and an ignore rule in .gitignore. Files that Git
tracks are rejected. An existing variable needs --replace.

Example:
  dwellir keys env my-key
  dwellir keys env my-key --env-file .env.local --var MY_DWELLIR_KEY`,
	Args: keyNameArg("dwellir keys env ci-key"),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !envVarName.MatchString(keyEnvVar) {
			return getFormatter().Error("validation_error", "Invalid variable name for --var.", "Use letters, digits and underscores, for example DWELLIR_API_KEY.")
		}
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := validateUntrackedEnv(dir, keyEnvFile); err != nil {
			return getFormatter().Error("validation_error", err.Error(),
				"Credentials must not go into a file that Git tracks. Run 'git rm --cached "+keyEnvFile+"', or pass --env-file .env.local.")
		}
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		key, err := findKey(client, args[0])
		if err != nil {
			return err
		}
		values := map[string]string{keyEnvVar: key.APIKey.Reveal()}
		if err := config.WriteProjectEnv(dir, keyEnvFile, values, keyEnvReplace); err != nil {
			return getFormatter().Error("configuration_error", err.Error(), projectEnvHelp+" Pass --var to write another variable.")
		}
		return getFormatter().Success("keys.env", map[string]interface{}{
			"name": key.Name, "key": key.APIKey.Masked(), "file": keyEnvFile, "variable": keyEnvVar,
		})
	},
}

func init() {
	keysCreateCmd.Flags().StringVar(&keyName, "name", "", "Key name (required)")
	keysCreateCmd.Flags().IntVar(&keyDailyQuota, "daily-quota", 0, "Daily request quota")
	keysCreateCmd.Flags().IntVar(&keyMonthlyQuota, "monthly-quota", 0, "Monthly request quota")

	keysUpdateCmd.Flags().StringVar(&keyName, "name", "", "New key name")
	keysUpdateCmd.Flags().IntVar(&keyDailyQuota, "daily-quota", 0, "Daily request quota")
	keysUpdateCmd.Flags().IntVar(&keyMonthlyQuota, "monthly-quota", 0, "Monthly request quota")

	keysEnvCmd.Flags().StringVar(&keyEnvFile, "env-file", ".env", "Environment file: .env or .env.local")
	keysEnvCmd.Flags().StringVar(&keyEnvVar, "var", "DWELLIR_API_KEY", "Variable name")
	keysEnvCmd.Flags().BoolVar(&keyEnvReplace, "replace", false, "Replace an existing value")

	keysRotateCmd.Flags().BoolVarP(&keyRotateYes, "yes", "y", false, "Skip the confirmation prompt")

	keysCmd.AddCommand(keysListCmd, keysCreateCmd, keysUpdateCmd, keysDeleteCmd, keysRotateCmd, keysEnableCmd, keysDisableCmd, keysCopyCmd, keysEnvCmd)
	rootCmd.AddCommand(keysCmd)
}
