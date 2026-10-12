package cli

import (
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
)

var (
	statusFeedFlags      statusFilterFlags
	statusFeedFormat     string
	statusFeedSlack      bool
	statusFeedNoValidate bool

	statusSubscribeFlags       statusFilterFlags
	statusSubscribeURL         string
	statusSubscribeMinSeverity string
)

var statusFeedCmd = &cobra.Command{
	Use:   "feed",
	Short: "Print an Atom or RSS feed URL for status changes",
	Long: `Print a feed URL for status changes that match the filters.

The CLI checks filter values against 'dwellir status components' and fails on
unknown values. Add --no-validate to skip the check.

Add --slack to print the Slack command. Run it in a channel that has the
Slack RSS app.

Examples:
  dwellir status feed --chain base
  dwellir status feed --network ethereum/mainnet --format rss
  dwellir status feed --chain base --slack`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := statusEnumFlag("format", statusFeedFormat, "atom", "rss"); err != nil {
			return err
		}
		client, err := newStatusAPI()
		if err != nil {
			return err
		}
		filter := statusFeedFlags.filter()
		if !statusFeedNoValidate && !filter.IsEmpty() {
			components, err := client.Components()
			if err != nil {
				return statusFeedValidationError(err)
			}
			if unknown := findUnknownFilterValues(filter, components); len(unknown) > 0 {
				return unknownFilterError(unknown, true)
			}
		}

		feed := api.StatusFeed{
			Format:    statusFeedFormat,
			URL:       api.FeedURL(client.BaseURL(), statusFeedFormat, filter),
			Filter:    filter,
			Validated: !statusFeedNoValidate && !filter.IsEmpty(),
		}
		if statusFeedSlack {
			feed.SlackCommand = "/feed subscribe " + feed.URL
		}
		return getFormatter().Success("status.feed", feed)
	},
}

// statusFeedValidationError reports a failed component fetch and names the
// way around it.
func statusFeedValidationError(err error) error {
	code, message, help := classifyStatusError(err)
	help = strings.TrimSpace(help + "\nAdd --no-validate to build the URL without checking filter values.")
	return statusFail(code, "Cannot check filter values: "+message, help)
}

var statusSubscribeCmd = &cobra.Command{
	Use:   "subscribe",
	Short: "Send status changes to a webhook, Slack, or Discord URL",
	Long: `Create a subscription that POSTs status changes to an HTTPS URL.

The API detects Slack incoming webhooks and Discord webhooks from the URL and
formats messages for them. Other URLs get signed JSON events.

The reply holds the signing secret and the unsubscribe URL. The API shows them
only once, so store both.

The subscription starts as pending. The API sends a test event first, and the
subscription turns active when that delivery succeeds.

Examples:
  dwellir status subscribe --url https://example.com/hooks/dwellir
  dwellir status subscribe --url https://hooks.slack.com/services/T/B/X --chain base --min-severity outage`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		target := strings.TrimSpace(statusSubscribeURL)
		if err := validateSubscriptionURL(target); err != nil {
			return err
		}
		if err := statusEnumFlag("min-severity", statusSubscribeMinSeverity, "degraded", "outage"); err != nil {
			return err
		}
		client, err := newStatusAPI()
		if err != nil {
			return err
		}
		filter := statusSubscribeFlags.filter()
		subscription, err := client.Subscribe(api.SubscriptionRequest{
			URL:         target,
			Chains:      filter.Chains,
			Networks:    filter.Networks,
			Endpoints:   filter.Endpoints,
			MinSeverity: statusSubscribeMinSeverity,
		})
		if err != nil {
			return statusCommandError(err)
		}
		return getFormatter().Success("status.subscribe", subscription)
	},
}

var statusUnsubscribeCmd = &cobra.Command{
	Use:   "unsubscribe <token|unsubscribe-url>",
	Short: "Delete a subscription",
	Long: `Delete a subscription. Pass the unsubscribe URL from 'dwellir status subscribe',
or the token from that URL.

Examples:
  dwellir status unsubscribe 'https://status.dwellir.com/unsubscribe?token=abc123'
  dwellir status unsubscribe abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := api.ParseUnsubscribeToken(args[0])
		if err != nil {
			return statusFail("validation_error", "Cannot read the unsubscribe token: "+err.Error()+".",
				"Pass the unsubscribe URL from 'dwellir status subscribe', or the token from that URL.")
		}
		client, err := newStatusAPI()
		if err != nil {
			return err
		}
		result, err := client.Unsubscribe(token)
		if err != nil {
			return statusCommandError(err)
		}
		if !result.Deleted {
			return statusFail("not_found", "No subscription matches that token.",
				"The subscription may be deleted already. Check the token or the unsubscribe URL.")
		}
		return getFormatter().Success("status.unsubscribe", result)
	},
}

// validateSubscriptionURL catches the obvious mistakes before the request. The
// API runs the full checks, such as blocking private hosts.
func validateSubscriptionURL(target string) error {
	help := "Pass an HTTPS webhook URL, such as a Slack incoming webhook or a Discord webhook."
	if target == "" {
		return statusFail("validation_error", "--url is required.", help)
	}
	u, err := url.Parse(target)
	if err != nil || u.Host == "" {
		return statusFail("validation_error", "--url is not a valid absolute URL.", help)
	}
	if u.Scheme != "https" {
		return statusFail("validation_error", "--url must use https.", help)
	}
	return nil
}

func init() {
	statusFeedFlags.register(statusFeedCmd)
	statusFeedCmd.Flags().StringVar(&statusFeedFormat, "format", "atom", "Feed format: atom or rss")
	statusFeedCmd.Flags().BoolVar(&statusFeedSlack, "slack", false, "Print the Slack /feed subscribe command")
	statusFeedCmd.Flags().BoolVar(&statusFeedNoValidate, "no-validate", false, "Skip the check of filter values against the component list")

	statusSubscribeFlags.register(statusSubscribeCmd)
	statusSubscribeCmd.Flags().StringVar(&statusSubscribeURL, "url", "", "HTTPS URL that receives events (required)")
	statusSubscribeCmd.Flags().StringVar(&statusSubscribeMinSeverity, "min-severity", "degraded",
		"Lowest severity to send: degraded (degradations and outages) or outage")
}
