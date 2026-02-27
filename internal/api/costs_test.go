package api

import (
	"testing"
	"time"
)

func TestPlanAllowsOverages(t *testing.T) {
	if PlanAllowsOverages(90) {
		t.Fatalf("expected internal plan to disallow overages")
	}
	if !PlanAllowsOverages(2) {
		t.Fatalf("expected developer plan to allow overages")
	}
}

func TestCalculateUsageCostReportBasic(t *testing.T) {
	plan := SubscriptionInfo{
		ID:       2,
		PlanName: "Developer",
	}
	start := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	rows := []UsageHistory{
		{
			Timestamp: start.Format(time.RFC3339),
			Domain:    "api-base-mainnet.n.dwellir.com",
			Responses: 1_000_000,
			Requests:  1_000_000,
		},
	}

	report := CalculateUsageCostReport(
		plan,
		25_000_000,
		nil,
		nil,
		start,
		end,
		rows,
		rows,
	)

	if !report.Supported {
		t.Fatalf("expected report to be supported")
	}
	if report.TotalCost <= 0 {
		t.Fatalf("expected positive cost, got %f", report.TotalCost)
	}
	if len(report.ByDomain) != 1 {
		t.Fatalf("expected one domain breakdown row, got %d", len(report.ByDomain))
	}
}
