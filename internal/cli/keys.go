package cli

import (
	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
)

var (
	keyName         string
	keyDailyQuota   int
	keyMonthlyQuota int
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage API keys",
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
			return err
		}
		return getFormatter().Success("keys.list", keys)
	},
}

var keysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		input := api.CreateKeyInput{Name: keyName}
		if keyDailyQuota > 0 {
			input.DailyQuota = &keyDailyQuota
		}
		if keyMonthlyQuota > 0 {
			input.MonthlyQuota = &keyMonthlyQuota
		}
		key, err := api.NewKeysAPI(client).Create(input)
		if err != nil {
			return err
		}
		return getFormatter().Success("keys.create", key)
	},
}

var keysUpdateCmd = &cobra.Command{
	Use:   "update <key-id>",
	Short: "Update an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
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
		key, err := api.NewKeysAPI(client).Update(args[0], input)
		if err != nil {
			return err
		}
		return getFormatter().Success("keys.update", key)
	},
}

var keysDeleteCmd = &cobra.Command{
	Use:   "delete <key-id>",
	Short: "Delete an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		err = api.NewKeysAPI(client).Delete(args[0])
		if err != nil {
			return err
		}
		return getFormatter().Success("keys.delete", map[string]string{"key": args[0], "status": "deleted"})
	},
}

var keysEnableCmd = &cobra.Command{
	Use:   "enable <key-id>",
	Short: "Enable an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		key, err := api.NewKeysAPI(client).Enable(args[0])
		if err != nil {
			return err
		}
		return getFormatter().Success("keys.enable", key)
	},
}

var keysDisableCmd = &cobra.Command{
	Use:   "disable <key-id>",
	Short: "Disable an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		key, err := api.NewKeysAPI(client).Disable(args[0])
		if err != nil {
			return err
		}
		return getFormatter().Success("keys.disable", key)
	},
}

func init() {
	keysCreateCmd.Flags().StringVar(&keyName, "name", "", "Key name")
	keysCreateCmd.Flags().IntVar(&keyDailyQuota, "daily-quota", 0, "Daily request quota")
	keysCreateCmd.Flags().IntVar(&keyMonthlyQuota, "monthly-quota", 0, "Monthly request quota")

	keysUpdateCmd.Flags().StringVar(&keyName, "name", "", "New key name")
	keysUpdateCmd.Flags().IntVar(&keyDailyQuota, "daily-quota", 0, "Daily request quota")
	keysUpdateCmd.Flags().IntVar(&keyMonthlyQuota, "monthly-quota", 0, "Monthly request quota")

	keysCmd.AddCommand(keysListCmd, keysCreateCmd, keysUpdateCmd, keysDeleteCmd, keysEnableCmd, keysDisableCmd)
	rootCmd.AddCommand(keysCmd)
}
