package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/dwellir-public/cli/internal/api"
)

type usageWindow struct {
	Interval       string
	Start          time.Time
	End            time.Time
	UsedDefaults   bool
	DefaultLabel   string
	FormattedStart string
	FormattedEnd   string
}

func resolveUsageWindow(interval, from, to string) (usageWindow, error) {
	normalized := strings.ToLower(strings.TrimSpace(interval))
	if normalized == "" {
		normalized = "hour"
	}
	switch normalized {
	case "minute", "hour", "day":
	default:
		return usageWindow{}, getFormatter().Error(
			"validation_error",
			fmt.Sprintf("Invalid interval %q.", interval),
			"Supported intervals: minute, hour, day",
		)
	}

	var start time.Time
	var end time.Time
	usedDefaults := false
	defaultLabel := ""

	now := time.Now().UTC().Truncate(time.Minute)

	if strings.TrimSpace(to) == "" {
		end = now
		usedDefaults = true
	} else {
		parsed, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return usageWindow{}, getFormatter().Error(
				"validation_error",
				fmt.Sprintf("Invalid --to timestamp %q.", to),
				"Use RFC3339 format, e.g. 2026-02-27T23:59:59Z",
			)
		}
		end = parsed.UTC()
	}

	if strings.TrimSpace(from) == "" {
		d, label := defaultDurationForInterval(normalized)
		start = end.Add(-d)
		defaultLabel = label
		usedDefaults = true
	} else {
		parsed, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return usageWindow{}, getFormatter().Error(
				"validation_error",
				fmt.Sprintf("Invalid --from timestamp %q.", from),
				"Use RFC3339 format, e.g. 2026-02-27T00:00:00Z",
			)
		}
		start = parsed.UTC()
	}

	if !start.Before(end) {
		return usageWindow{}, getFormatter().Error(
			"validation_error",
			"`--from` must be earlier than `--to`.",
			"",
		)
	}

	return usageWindow{
		Interval:       normalized,
		Start:          start,
		End:            end,
		UsedDefaults:   usedDefaults,
		DefaultLabel:   defaultLabel,
		FormattedStart: start.Format(time.RFC3339),
		FormattedEnd:   end.Format(time.RFC3339),
	}, nil
}

func validateUsageLookback(client *api.Client, window usageWindow) error {
	sub, err := api.NewAccountAPI(client).Subscription()
	if err != nil {
		return formatCommandError(err)
	}

	maxLookback, lookbackLabel, tierName := planLookback(sub.ID)
	requestedLookback := time.Since(window.Start)
	if requestedLookback <= maxLookback {
		return nil
	}

	requiredTier, requiredLabel := requiredTierForLookback(requestedLookback)
	guidance := fmt.Sprintf("Upgrade to %s for up to %s lookback.", requiredTier, requiredLabel)
	if strings.EqualFold(requiredTier, tierName) {
		guidance = "Contact Dwellir support for extended lookback options."
	}
	return getFormatter().Error(
		"validation_error",
		fmt.Sprintf("Requested usage range exceeds your plan lookback (%s).", lookbackLabel),
		fmt.Sprintf(
			"Current plan: %s\nAllowed lookback: %s\nRequested from: %s\n%s",
			tierName,
			lookbackLabel,
			window.Start.Format(time.RFC3339),
			guidance,
		),
	)
}

func planLookback(planID int) (duration time.Duration, label string, tier string) {
	switch planID {
	case 1, 5:
		return 24 * time.Hour, "24 hours", "Starter"
	case 2:
		return 7 * 24 * time.Hour, "7 days", "Developer"
	case 3:
		return 30 * 24 * time.Hour, "30 days", "Growth"
	default:
		return 90 * 24 * time.Hour, "90 days", "Scale"
	}
}

func requiredTierForLookback(lookback time.Duration) (tier string, label string) {
	switch {
	case lookback <= 24*time.Hour:
		return "Starter", "24 hours"
	case lookback <= 7*24*time.Hour:
		return "Developer", "7 days"
	case lookback <= 30*24*time.Hour:
		return "Growth", "30 days"
	default:
		return "Scale", "90 days"
	}
}

func defaultDurationForInterval(interval string) (time.Duration, string) {
	switch interval {
	case "minute":
		return 60 * time.Minute, "past 60 minutes"
	case "day":
		return 30 * 24 * time.Hour, "past 30 days"
	default:
		return 24 * time.Hour, "past 24 hours"
	}
}
