package output

import (
	"fmt"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"

	"github.com/dwellir-public/cli/internal/api"
)

// writeStatusCommand renders the data types of the `dwellir status` commands.
// It reports false for any other data.
func (f *HumanFormatter) writeStatusCommand(data interface{}) (bool, error) {
	switch value := data.(type) {
	case api.StatusOverview:
		return true, f.writeStatusOverview(value)
	case *api.StatusComponents:
		return true, f.writeStatusComponents(value)
	case *api.StatusIncidents:
		return true, f.writeStatusIncidents(value)
	case *api.StatusUptime:
		return true, f.writeStatusUptime(value)
	case api.StatusFeed:
		return true, f.writeStatusFeed(value)
	case *api.Subscription:
		return true, f.writeStatusSubscription(value)
	case *api.UnsubscribeResult:
		_, err := fmt.Fprintln(f.w, "Subscription deleted.")
		return true, err
	}
	return false, nil
}

func (f *HumanFormatter) writeStatusOverview(overview api.StatusOverview) error {
	if _, err := fmt.Fprintln(f.w, statusSummaryLine(overview.Summary)); err != nil {
		return err
	}
	if len(overview.Endpoints) == 0 {
		message := "No endpoints match those filters. Run 'dwellir status components' to list valid values."
		if overview.Listed == api.StatusListedNonOperational {
			message = "Add --all to list every endpoint."
		}
		_, err := fmt.Fprintln(f.w, message)
		return err
	}

	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Status", "Cause", "Network", "Endpoint", "Since", "Uptime 24h", "Lagging 24h"})
	for _, endpoint := range overview.Endpoints {
		tw.AppendRow(table.Row{
			endpoint.Status,
			orDash(derefString(endpoint.Cause)),
			endpoint.Network.Ref,
			endpoint.Endpoint.FQDN,
			formatStatusTime(derefString(endpoint.Since)),
			formatPercent(endpoint.Uptime24h),
			formatSampleShare(endpoint.LaggingSamples24h, endpoint.Samples24h),
		})
	}
	if _, err := fmt.Fprintln(f.w); err != nil {
		return err
	}
	return f.renderTable(tw)
}

func statusSummaryLine(summary api.StatusSummary) string {
	counts := fmt.Sprintf("%d operational, %d degraded, %d outage.", summary.Operational, summary.Degraded, summary.Outage)
	switch {
	case summary.Outage > 0:
		return "Some endpoints have an outage: " + counts
	case summary.Degraded > 0:
		return "Some endpoints are degraded: " + counts
	default:
		return "All endpoints operational: " + counts
	}
}

func (f *HumanFormatter) writeStatusComponents(components *api.StatusComponents) error {
	if len(components.Chains) == 0 {
		_, err := fmt.Fprintln(f.w, "No components found.")
		return err
	}
	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Chain", "Network", "Endpoint"})
	for _, chain := range components.Chains {
		for _, network := range chain.Networks {
			for _, endpoint := range network.Endpoints {
				tw.AppendRow(table.Row{chain.Slug, network.Ref, endpoint.FQDN})
			}
			if len(network.Endpoints) == 0 {
				tw.AppendRow(table.Row{chain.Slug, network.Ref, "-"})
			}
		}
	}
	return f.renderTable(tw)
}

func (f *HumanFormatter) writeStatusIncidents(incidents *api.StatusIncidents) error {
	if len(incidents.Incidents) == 0 {
		_, err := fmt.Fprintf(f.w, "No incidents since %s.\n", formatStatusTime(incidents.Since))
		return err
	}
	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Started", "Status", "Severity", "Peak", "Cause", "Network", "Endpoint", "Duration"})
	for _, incident := range incidents.Incidents {
		tw.AppendRow(table.Row{
			formatStatusTime(incident.StartedAt),
			incident.Status,
			incident.Severity,
			incident.PeakSeverity,
			orDash(derefString(incident.Cause)),
			incident.Network.Ref,
			incident.Endpoint.FQDN,
			formatIncidentDuration(incident),
		})
	}
	return f.renderTable(tw)
}

func (f *HumanFormatter) writeStatusUptime(uptime *api.StatusUptime) error {
	if len(uptime.Endpoints) == 0 {
		_, err := fmt.Fprintln(f.w, "No endpoints match those filters. Run 'dwellir status components' to list valid values.")
		return err
	}
	if _, err := fmt.Fprintf(f.w, "Uptime over %s\n\n", uptime.Range); err != nil {
		return err
	}
	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Network", "Endpoint", "Uptime", "Lagging", "Samples"})
	for _, endpoint := range uptime.Endpoints {
		tw.AppendRow(table.Row{
			endpoint.Network,
			endpoint.Endpoint,
			formatPercent(endpoint.Uptime),
			formatPercent(endpoint.LaggingShare),
			formatInt64(endpoint.SampleCount),
		})
	}
	return f.renderTable(tw)
}

func (f *HumanFormatter) writeStatusFeed(feed api.StatusFeed) error {
	if feed.SlackCommand == "" {
		_, err := fmt.Fprintln(f.w, feed.URL)
		return err
	}
	_, err := fmt.Fprintf(f.w, "%s\n\nRun this command in the Slack channel. It needs the Slack RSS app.\n", feed.SlackCommand)
	return err
}

func (f *HumanFormatter) writeStatusSubscription(subscription *api.Subscription) error {
	if err := f.renderKeyValueRows([][2]string{
		{"ID", subscription.ID},
		{"Kind", subscription.Kind},
		{"Status", subscription.Status},
		{"Signing secret", subscription.SigningSecret},
		{"Unsubscribe URL", subscription.UnsubscribeURL},
	}); err != nil {
		return err
	}
	if subscription.Message != "" {
		if _, err := fmt.Fprintf(f.w, "\n%s\n", subscription.Message); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(f.w, "\nWarning: the API shows the signing secret and the unsubscribe URL only once. Store both now.")
	return err
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func formatPercent(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f%%", *value)
}

func formatSampleShare(part, total int64) string {
	if total <= 0 {
		return "-"
	}
	share := float64(part) / float64(total) * 100
	return formatPercent(&share)
}

// formatStatusTime prints an RFC 3339 timestamp as UTC to the minute. Other
// values print unchanged.
func formatStatusTime(value string) string {
	if value == "" {
		return "-"
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return parsed.UTC().Format("2006-01-02 15:04 UTC")
}

func formatIncidentDuration(incident api.StatusIncident) string {
	if incident.DurationSeconds == nil {
		if incident.Status == "open" {
			return "ongoing"
		}
		return "-"
	}
	duration := formatCompactDuration(*incident.DurationSeconds)
	if incident.Status == "open" {
		return duration + " (ongoing)"
	}
	return duration
}

// formatCompactDuration prints seconds as "45s", "12m", "2h 5m", or "3d 4h".
func formatCompactDuration(seconds int64) string {
	switch {
	case seconds < 60:
		return fmt.Sprintf("%ds", max(seconds, 0))
	case seconds < 3600:
		return fmt.Sprintf("%dm", seconds/60)
	case seconds < 86400:
		return fmt.Sprintf("%dh %dm", seconds/3600, seconds%3600/60)
	default:
		return fmt.Sprintf("%dd %dh", seconds/86400, seconds%86400/3600)
	}
}
