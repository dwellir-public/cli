package cli

import (
	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "View account and subscription info",
}

var accountInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Organization info, plan, billing status",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		info, err := api.NewAccountAPI(client).Info()
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("account.info", info)
	},
}

var accountSubscriptionCmd = &cobra.Command{
	Use:   "subscription",
	Short: "Current subscription details",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		sub, err := api.NewAccountAPI(client).Subscription()
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("account.subscription", sub)
	},
}

func init() {
	accountCmd.AddCommand(accountInfoCmd, accountSubscriptionCmd)
	rootCmd.AddCommand(accountCmd)
}
