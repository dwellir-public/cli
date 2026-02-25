package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/config"
	"github.com/dwellir-public/cli/internal/output"
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
			return err
		}
		if err := cfg.Set(args[0], args[1]); err != nil {
			return err
		}
		f := getFormatter()
		return f.Success("config.set", map[string]string{args[0]: args[1]})
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultConfigDir())
		if err != nil {
			return err
		}
		val := cfg.Get(args[0])
		if val == "" {
			return fmt.Errorf("unknown config key: %s", args[0])
		}
		f := getFormatter()
		return f.Success("config.get", map[string]string{args[0]: val})
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all config values",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultConfigDir())
		if err != nil {
			return err
		}
		f := getFormatter()
		return f.Success("config.list", cfg.All())
	},
}

func init() {
	configCmd.AddCommand(configSetCmd, configGetCmd, configListCmd)
	rootCmd.AddCommand(configCmd)
}

func getFormatter() output.Formatter {
	cfg, _ := config.Load(config.DefaultConfigDir())
	format := cfg.Output
	if jsonOutput {
		format = "json"
	}
	if humanOutput {
		format = "human"
	}
	return output.New(format, rootCmd.OutOrStdout())
}
