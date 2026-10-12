package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

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
)

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

	keysCmd.AddCommand(keysListCmd, keysCreateCmd, keysUpdateCmd, keysDeleteCmd, keysEnableCmd, keysDisableCmd, keysCopyCmd, keysEnvCmd)
	rootCmd.AddCommand(keysCmd)
}
