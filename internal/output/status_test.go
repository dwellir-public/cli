package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dwellir-public/cli/internal/api"
)

func strPtr(value string) *string { return &value }

func int64Ptr(value int64) *int64 { return &value }

func TestHumanStatusOutput(t *testing.T) {
	degraded := api.EndpointStatus{
		Network:           api.StatusNetworkRef{Ref: "lisk/sepolia"},
		Endpoint:          api.StatusEndpointRef{FQDN: "api-lisk-sepolia.n.dwellir.com"},
		Status:            "degraded",
		Cause:             strPtr("lag"),
		Since:             strPtr("2026-10-12T09:41:00.000Z"),
		Uptime24h:         floatPtr(100),
		LaggingSamples24h: 360,
		Samples24h:        1440,
	}

	tests := []struct {
		name     string
		command  string
		data     interface{}
		want     []string
		wantNone []string
	}{
		{
			name:    "overview with a degraded endpoint",
			command: "status",
			data: api.StatusOverview{
				Summary:   api.StatusSummary{Operational: 180, Degraded: 1},
				Listed:    api.StatusListedNonOperational,
				Endpoints: []api.EndpointStatus{degraded},
			},
			want: []string{
				"Some endpoints are degraded: 180 operational, 1 degraded, 0 outage.",
				"lisk/sepolia", "api-lisk-sepolia.n.dwellir.com", "lag", "2026-10-12 09:41 UTC", "100.00%", "25.00%",
			},
		},
		{
			name:    "overview with everything operational",
			command: "status",
			data:    api.StatusOverview{Summary: api.StatusSummary{Operational: 3}, Listed: api.StatusListedNonOperational, Endpoints: []api.EndpointStatus{}},
			want:    []string{"All endpoints operational: 3 operational", "Add --all"},
		},
		{
			name:    "overview with filters that match nothing",
			command: "status",
			data:    api.StatusOverview{Filtered: true, Listed: api.StatusListedAll, Endpoints: []api.EndpointStatus{}},
			want:    []string{"No endpoints match those filters"},
		},
		{
			name:    "components",
			command: "status.components",
			data: &api.StatusComponents{Chains: []api.StatusComponentChain{{
				Slug: "ethereum",
				Networks: []api.StatusComponentNetwork{{
					Ref: "ethereum/mainnet", Endpoints: []api.StatusEndpointRef{{FQDN: "api-ethereum-mainnet.n.dwellir.com"}},
				}},
			}}},
			want: []string{"ethereum/mainnet", "api-ethereum-mainnet.n.dwellir.com"},
		},
		{
			name:    "incidents",
			command: "status.history",
			data: &api.StatusIncidents{Incidents: []api.StatusIncident{
				{
					Status: "resolved", Severity: "degraded", PeakSeverity: "outage", Cause: strPtr("errors"),
					StartedAt: "2026-10-11T08:00:00.000Z", DurationSeconds: int64Ptr(720),
					Network: api.StatusNetworkRef{Ref: "base/mainnet"}, Endpoint: api.StatusEndpointRef{FQDN: "api-base-mainnet.n.dwellir.com"},
				},
				{Status: "open", Severity: "outage", PeakSeverity: "outage", StartedAt: "2026-10-12T08:00:00.000Z"},
			}},
			want: []string{"2026-10-11 08:00 UTC", "resolved", "outage", "errors", "base/mainnet", "12m", "ongoing"},
		},
		{
			name:    "no incidents",
			command: "status.history",
			data:    &api.StatusIncidents{Since: "2026-10-05T12:00:00.000Z", Incidents: []api.StatusIncident{}},
			want:    []string{"No incidents since 2026-10-05 12:00 UTC."},
		},
		{
			name:    "uptime",
			command: "status.uptime",
			data: &api.StatusUptime{Range: "7d", Endpoints: []api.EndpointUptimeStat{
				{Network: "ethereum/mainnet", Endpoint: "api-ethereum-mainnet.n.dwellir.com", Uptime: floatPtr(99.5), LaggingShare: floatPtr(2.5), SampleCount: 10080},
				{Network: "ethereum/sepolia", Endpoint: "api-ethereum-sepolia.n.dwellir.com"},
			}},
			want: []string{"Uptime over 7d", "99.50%", "2.50%", "10,080", "-"},
		},
		{
			name:     "feed URL",
			command:  "status.feed",
			data:     api.StatusFeed{URL: "https://status.dwellir.com/history.atom?chain=base"},
			want:     []string{"https://status.dwellir.com/history.atom?chain=base"},
			wantNone: []string{"/feed subscribe"},
		},
		{
			name:    "feed Slack command",
			command: "status.feed",
			data:    api.StatusFeed{URL: "https://x/history.atom", SlackCommand: "/feed subscribe https://x/history.atom"},
			want:    []string{"/feed subscribe https://x/history.atom", "Slack RSS app"},
		},
		{
			name:    "subscription warns that the secret is shown once",
			command: "status.subscribe",
			data: &api.Subscription{
				ID: "sub-1", Kind: "slack", Status: "pending", SigningSecret: "whsec_abc",
				UnsubscribeURL: "https://status.dwellir.com/unsubscribe?token=tok", Message: "We sent a test event.",
			},
			want: []string{"slack", "pending", "whsec_abc", "unsubscribe?token=tok", "We sent a test event.", "only once"},
		},
		{
			name:    "unsubscribe",
			command: "status.unsubscribe",
			data:    &api.UnsubscribeResult{Deleted: true},
			want:    []string{"Subscription deleted."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := NewHumanFormatter(&buf).Success(tt.command, tt.data); err != nil {
				t.Fatalf("Success() error: %v", err)
			}
			out := buf.String()
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("expected %q in output:\n%s", want, out)
				}
			}
			for _, unwanted := range tt.wantNone {
				if strings.Contains(out, unwanted) {
					t.Errorf("did not expect %q in output:\n%s", unwanted, out)
				}
			}
		})
	}
}

func TestFormatCompactDuration(t *testing.T) {
	tests := []struct {
		seconds int64
		want    string
	}{
		{seconds: 45, want: "45s"},
		{seconds: 720, want: "12m"},
		{seconds: 7500, want: "2h 5m"},
		{seconds: 273600, want: "3d 4h"},
	}
	for _, tt := range tests {
		if got := formatCompactDuration(tt.seconds); got != tt.want {
			t.Errorf("formatCompactDuration(%d) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}
