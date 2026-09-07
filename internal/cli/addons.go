package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
)

var (
	addonsListPremium   bool
	addonsListUnlimited bool
	addonsListChain     string
)

var addonsCmd = &cobra.Command{
	Use:   "addons",
	Short: "Browse endpoint add-ons and see which ones you have",
	Long: `Add-ons unlock premium endpoints and unmetered RPS tiers.

  dwellir addons list      What you can buy
  dwellir addons status    What you have, plus premium-endpoint trials

Purchase and cancellation are not available from the CLI yet.`,
}

var addonsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Add-ons available to buy",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}

		catalog, err := api.NewAddonsAPI(client).Catalog()
		if err != nil {
			return formatCommandError(err)
		}

		// The account supplies ownership and trial state. It is fetched second
		// so a plain auth failure surfaces against the catalog call first.
		account, err := api.NewAccountAPI(client).Info()
		if err != nil {
			return formatCommandError(err)
		}

		entries := api.BuildAddonCatalog(catalog, account)
		entries = api.FilterAddonCatalog(entries, addonsListPremium, addonsListUnlimited, addonsListChain)
		return getFormatter().Success("addons.list", entries)
	},
}

var addonsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Add-ons on your subscription, plus premium-endpoint trials",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}

		// Fetch the account first. If the token itself is bad this fails here,
		// which keeps the fallback below from misreading a dead token as
		// "marly rejects CLI tokens on the billing route".
		account, err := api.NewAccountAPI(client).Info()
		if err != nil {
			return formatCommandError(err)
		}

		active, source, notices, err := resolveActiveAddOns(api.NewAddonsAPI(client), account)
		if err != nil {
			return formatCommandError(err)
		}

		status := api.BuildAddonsStatus(active, account, source)
		status.Notices = notices
		return getFormatter().Success("addons.status", status)
	},
}

// resolveActiveAddOns reads the caller's add-on instances, preferring the
// billing route and falling back to the account payload.
//
// GET /v4/billing/addons/active is still cookie-only in marly, so a CLI token
// gets 401 or 403 there. Marly builds both responses from the same
// SubscriptionAddOn model, so the account payload carries the same instances and
// the same fields, and the fallback keeps `addons status` fully useful.
//
// The substitution is still reported as a typed notice rather than hidden. The
// two sources can drift, and a caller comparing this output against the
// dashboard needs to know which one answered.
func resolveActiveAddOns(
	addons *api.AddonsAPI,
	account *api.AccountInfo,
) ([]api.ActiveAddOn, string, []api.AddonNotice, error) {
	active, err := addons.Active()
	if err == nil {
		return active, api.AddonsSourceBilling, nil, nil
	}

	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || (apiErr.StatusCode != 401 && apiErr.StatusCode != 403) {
		return nil, "", nil, err
	}

	notice := api.AddonNotice{
		Code:    "addons_active_unavailable",
		Message: "The billing add-on route rejected this CLI token, so add-ons were read from your organization account instead.",
		Help:    "GET /v4/billing/addons/active still requires a dashboard session. The account reports the same add-on instances and fields, so this list should be complete.",
	}
	return api.ActiveAddOnsFromAccount(account), api.AddonsSourceAccount, []api.AddonNotice{notice}, nil
}

func init() {
	addonsListCmd.Flags().BoolVar(&addonsListPremium, "premium", false, "Only premium endpoint access add-ons")
	addonsListCmd.Flags().BoolVar(&addonsListUnlimited, "unlimited", false, "Only unlimited RPS tier add-ons")
	addonsListCmd.Flags().StringVar(&addonsListChain, "chain", "", "Filter by chain name, endpoint slug, FQDN, or URL")
	addonsCmd.AddCommand(addonsListCmd, addonsStatusCmd)

	if addonsEnabled() {
		rootCmd.AddCommand(addonsCmd)
	}
}
