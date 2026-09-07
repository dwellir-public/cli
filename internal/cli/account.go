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

var accountPaymentMethodCmd = &cobra.Command{
	Use:   "payment-method",
	Short: "Card on file for this organization",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		method, err := api.NewAddonsAPI(client).PaymentMethod()
		if err != nil {
			return formatCommandError(err)
		}
		// A nil method is not an error: it is marly's way of saying no card is
		// on file, so the boolean is what callers should branch on.
		return getFormatter().Success("account.payment-method", api.PaymentMethodResult{
			HasPaymentMethod: method != nil,
			PaymentMethod:    method,
		})
	},
}

func init() {
	accountCmd.AddCommand(accountInfoCmd, accountSubscriptionCmd)
	if addonsEnabled() {
		// Ships with the gated add-on surface: it exists to pre-flight a
		// purchase, and it should not appear before purchasing does.
		accountCmd.AddCommand(accountPaymentMethodCmd)
	}
	rootCmd.AddCommand(accountCmd)
}
