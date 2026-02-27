package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure CLI settings",
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Long:  "Set a CLI configuration value.\n\nValid keys: output (json|human), default_profile (<name>)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultConfigDir())
		if err != nil {
			return formatCommandError(err)
		}
		if err := cfg.Set(args[0], args[1]); err != nil {
			return formatCommandError(err)
		}
		f := getFormatter()
		return f.Success("config.set", map[string]string{args[0]: args[1]})
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get one config value, or all values when no key is provided",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultConfigDir())
		if err != nil {
			return formatCommandError(err)
		}
		f := getFormatter()
		if len(args) == 0 {
			return f.Success("config.get", cfg.All())
		}

		val := cfg.Get(args[0])
		if val == "" {
			return f.Error(
				"validation_error",
				fmt.Sprintf("Unknown config key %q.", args[0]),
				"Valid keys: output, default_profile\nExamples:\n  dwellir config get output\n  dwellir config get",
			)
		}
		return f.Success("config.get", map[string]string{args[0]: val})
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all config values",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultConfigDir())
		if err != nil {
			return formatCommandError(err)
		}
		f := getFormatter()
		return f.Success("config.list", cfg.All())
	},
}

func init() {
	configCmd.AddCommand(configSetCmd, configGetCmd, configListCmd)
	rootCmd.AddCommand(configCmd)
}
