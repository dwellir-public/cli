package api

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const effectiveIncludedRatePerMillion = 2.0

type CostType string

const (
	CostIncluded           CostType = "included"
	CostOverage            CostType = "overage"
	CostIncludedAndOverage CostType = "included_and_overage"
)

type PlanPricingConfig struct {
	BaseCost          float64
	OveragePerMillion float64
	AllowsOverages    bool
}

type BillingSegment struct {
	Start                  time.Time
	End                    time.Time
	BillingPeriodStart     time.Time
	BillingPeriodEnd       time.Time
	CumulativeUsageAtStart int
}

type CostSegmentBreakdown struct {
	Start           string   `json:"start"`
	End             string   `json:"end"`
	Responses       int      `json:"responses"`
	TotalResponses  int      `json:"total_responses"`
	Cost            float64  `json:"cost"`
	CostType        CostType `json:"cost_type"`
	Calculation     string   `json:"calculation"`
	IncludedAtStart int      `json:"included_remaining"`
}

type CostByGroup struct {
	Group     string  `json:"group"`
	Responses int     `json:"responses"`
	Cost      float64 `json:"cost"`
}

type CostReport struct {
	PlanID          int                    `json:"plan_id"`
	PlanName        string                 `json:"plan_name"`
	IntervalStart   string                 `json:"interval_start"`
	IntervalEnd     string                 `json:"interval_end"`
	TotalResponses  int                    `json:"total_responses"`
	TotalCost       float64                `json:"total_cost"`
	Supported       bool                   `json:"supported"`
	UnsupportedHint string                 `json:"unsupported_hint,omitempty"`
	Segments        []CostSegmentBreakdown `json:"segments,omitempty"`
	ByDomain        []CostByGroup          `json:"by_domain,omitempty"`
}

var planPricingByID = map[int]PlanPricingConfig{
	1:  {BaseCost: 0, OveragePerMillion: 0, AllowsOverages: false},
	2:  {BaseCost: 49, OveragePerMillion: 5, AllowsOverages: true},
	3:  {BaseCost: 299, OveragePerMillion: 3, AllowsOverages: true},
	4:  {BaseCost: 999, OveragePerMillion: 2, AllowsOverages: true},
	5:  {BaseCost: 0, OveragePerMillion: 0, AllowsOverages: false},
	11: {BaseCost: 0, OveragePerMillion: 2, AllowsOverages: true},
	21: {BaseCost: 0, OveragePerMillion: 2, AllowsOverages: true},
	22: {BaseCost: 0, OveragePerMillion: 2, AllowsOverages: true},
	23: {BaseCost: 0, OveragePerMillion: 2, AllowsOverages: true},
	24: {BaseCost: 386, OveragePerMillion: 3, AllowsOverages: true},
	25: {BaseCost: 11700, OveragePerMillion: 0.78, AllowsOverages: true},
	26: {BaseCost: 0, OveragePerMillion: 2, AllowsOverages: true},
	27: {BaseCost: 0, OveragePerMillion: 2, AllowsOverages: true},
	28: {BaseCost: 0, OveragePerMillion: 2, AllowsOverages: true},
	29: {BaseCost: 0, OveragePerMillion: 2, AllowsOverages: true},
	30: {BaseCost: 0, OveragePerMillion: 2, AllowsOverages: true},
	31: {BaseCost: 1800, OveragePerMillion: 1.8, AllowsOverages: true},
	90: {BaseCost: 0, OveragePerMillion: 0, AllowsOverages: false},
	99: {BaseCost: 0, OveragePerMillion: 0, AllowsOverages: false},
}

func PlanAllowsOverages(planID int) bool {
	cfg, ok := planPricingByID[planID]
	if !ok {
		return true
	}
	return cfg.AllowsOverages
}

func PlanPricing(planID int) PlanPricingConfig {
	cfg, ok := planPricingByID[planID]
	if ok {
		return cfg
	}
	return PlanPricingConfig{
		BaseCost:          0,
		OveragePerMillion: 2,
		AllowsOverages:    true,
	}
}

func CalculateUsageCostReport(
	plan SubscriptionInfo,
	monthlyQuota int,
	discount *DiscountInfo,
	currentSub *CurrentSubscriptionWindow,
	intervalStart time.Time,
	intervalEnd time.Time,
	filtered []UsageHistory,
	basis []UsageHistory,
) CostReport {
	report := CostReport{
		PlanID:         plan.ID,
		PlanName:       plan.EffectivePlanName(),
		IntervalStart:  intervalStart.Format(time.RFC3339),
		IntervalEnd:    intervalEnd.Format(time.RFC3339),
		TotalResponses: sumResponses(filtered),
		Supported:      PlanAllowsOverages(plan.ID),
	}
	if !report.Supported {
		report.UnsupportedHint = "Your plan does not have usage-based costs. Upgrade to Developer or higher."
		return report
	}

	pricing := PlanPricing(plan.ID)
	segments := splitIntervalByBillingPeriods(
		intervalStart,
		intervalEnd,
		parseSubscriptionDate(currentSub.GetRenewalDate()),
		parseSubscriptionDate(currentSub.GetStartDate()),
	)

	byDomain := map[string]*CostByGroup{}
	totalCost := 0.0

	for idx := range segments {
		segment := &segments[idx]
		usageBefore := sumResponsesInRange(basis, segment.BillingPeriodStart, segment.Start)
		segment.CumulativeUsageAtStart = usageBefore

		segmentBasis := filterRowsInRange(basis, segment.Start, segment.End)
		segmentFiltered := filterRowsInRange(filtered, segment.Start, segment.End)

		totalSegmentResponses := sumResponses(segmentBasis)
		filteredResponses := sumResponses(segmentFiltered)

		costTotal, costType, calc := calculateSegmentCost(
			totalSegmentResponses,
			*segment,
			pricing,
			monthlyQuota,
			discount,
		)

		filteredCost := 0.0
		if totalSegmentResponses > 0 {
			filteredCost = costTotal * (float64(filteredResponses) / float64(totalSegmentResponses))
		}
		totalCost += filteredCost

		report.Segments = append(report.Segments, CostSegmentBreakdown{
			Start:           segment.Start.Format(time.RFC3339),
			End:             segment.End.Format(time.RFC3339),
			Responses:       filteredResponses,
			TotalResponses:  totalSegmentResponses,
			Cost:            filteredCost,
			CostType:        costType,
			Calculation:     calc,
			IncludedAtStart: max(0, monthlyQuota-segment.CumulativeUsageAtStart),
		})

		domainCounts := map[string]int{}
		for _, row := range segmentFiltered {
			domain := strings.TrimSpace(row.Domain)
			if domain == "" {
				domain = "unknown"
			}
			domainCounts[domain] += row.Responses
		}
		for domain, responses := range domainCounts {
			entry := byDomain[domain]
			if entry == nil {
				entry = &CostByGroup{Group: domain}
				byDomain[domain] = entry
			}
			entry.Responses += responses
			if filteredResponses > 0 {
				entry.Cost += filteredCost * (float64(responses) / float64(filteredResponses))
			}
		}
	}

	report.TotalCost = totalCost
	for _, item := range byDomain {
		report.ByDomain = append(report.ByDomain, *item)
	}
	sort.Slice(report.ByDomain, func(i, j int) bool {
		if report.ByDomain[i].Cost == report.ByDomain[j].Cost {
			return report.ByDomain[i].Group < report.ByDomain[j].Group
		}
		return report.ByDomain[i].Cost > report.ByDomain[j].Cost
	})
	sort.Slice(report.Segments, func(i, j int) bool {
		return report.Segments[i].Start < report.Segments[j].Start
	})
	return report
}

func EarliestBillingPeriodStart(intervalStart, intervalEnd time.Time, currentSub *CurrentSubscriptionWindow) time.Time {
	segments := splitIntervalByBillingPeriods(
		intervalStart,
		intervalEnd,
		parseSubscriptionDate(currentSub.GetRenewalDate()),
		parseSubscriptionDate(currentSub.GetStartDate()),
	)
	if len(segments) == 0 {
		return intervalStart
	}
	return segments[0].BillingPeriodStart
}

func splitIntervalByBillingPeriods(
	intervalStart time.Time,
	intervalEnd time.Time,
	renewalDate *time.Time,
	subscriptionStartDate *time.Time,
) []BillingSegment {
	segments := make([]BillingSegment, 0, 8)
	current := intervalStart
	const maxSegments = 24

	for i := 0; i < maxSegments && current.Before(intervalEnd); i++ {
		window := calculateBillingCycleWindow(current, renewalDate, subscriptionStartDate)
		segmentEnd := window.End
		if segmentEnd.After(intervalEnd) {
			segmentEnd = intervalEnd
		}
		if !segmentEnd.After(current) {
			break
		}
		segments = append(segments, BillingSegment{
			Start:              current,
			End:                segmentEnd,
			BillingPeriodStart: window.Start,
			BillingPeriodEnd:   window.End,
		})
		current = segmentEnd
	}
	return segments
}

type billingWindow struct {
	Start time.Time
	End   time.Time
}

func calculateBillingCycleWindow(now time.Time, renewalDate *time.Time, startDate *time.Time) billingWindow {
	if renewalDate != nil {
		return usageWindowFromAnchor(now, *renewalDate)
	}
	if startDate != nil && !startDate.After(now) {
		return usageWindowFromAnchor(now, *startDate)
	}
	start := truncateToUTCMonth(now)
	return billingWindow{Start: start, End: start.AddDate(0, 1, 0)}
}

func usageWindowFromAnchor(now time.Time, anchorInput time.Time) billingWindow {
	anchor := anchorInput
	for !anchor.After(now) {
		anchor = anchor.AddDate(0, 1, 0)
	}
	start := anchor.AddDate(0, -1, 0)
	for now.Before(start) {
		anchor = anchor.AddDate(0, -1, 0)
		start = anchor.AddDate(0, -1, 0)
	}
	return billingWindow{Start: start, End: anchor}
}

func calculateSegmentCost(
	responses int,
	segment BillingSegment,
	pricing PlanPricingConfig,
	includedResponses int,
	discount *DiscountInfo,
) (float64, CostType, string) {
	if responses <= 0 {
		return 0, CostIncluded, "No responses"
	}

	daysInPeriod := daysInRange(segment.BillingPeriodStart, segment.BillingPeriodEnd)
	daysInSegment := daysInRange(segment.Start, segment.End)

	cumulativeAtEnd := segment.CumulativeUsageAtStart + responses
	includedAtStart := max(0, includedResponses-segment.CumulativeUsageAtStart)

	includedInSegment := 0
	overageInSegment := 0

	if segment.CumulativeUsageAtStart >= includedResponses {
		overageInSegment = responses
	} else if cumulativeAtEnd <= includedResponses {
		includedInSegment = responses
	} else {
		includedInSegment = includedAtStart
		overageInSegment = responses - includedInSegment
	}

	includedCost := 0.0
	overageCost := 0.0
	calcParts := make([]string, 0, 2)

	if includedInSegment > 0 {
		dailyBase := pricing.BaseCost / float64(daysInPeriod)
		segmentBase := dailyBase * float64(daysInSegment)
		effective := (float64(includedInSegment) / 1_000_000.0) * effectiveIncludedRatePerMillion
		includedCost = maxFloat(segmentBase, effective)
		calcParts = append(calcParts, fmt.Sprintf(
			"%d days × ($%.2f ÷ %d) = $%.2f",
			daysInSegment,
			pricing.BaseCost,
			daysInPeriod,
			includedCost,
		))
	}
	if overageInSegment > 0 {
		overageCost = (float64(overageInSegment) / 1_000_000.0) * pricing.OveragePerMillion
		calcParts = append(calcParts, fmt.Sprintf(
			"%.2fM × $%.2f/M = $%.2f",
			float64(overageInSegment)/1_000_000.0,
			pricing.OveragePerMillion,
			overageCost,
		))
	}

	total := includedCost + overageCost
	if discount != nil && discount.Applied {
		discountScope := total
		if !discount.Meta.ApplyToAddOns {
			discountScope = includedCost
		}
		switch discount.Type {
		case "percentage":
			discountAmount := discountScope * discount.Percentage
			total -= discountAmount
		case "flat":
			discountAmount := minFloat(discount.FlatAmount, discountScope)
			total -= discountAmount
		}
	}

	costType := CostIncluded
	switch {
	case includedInSegment > 0 && overageInSegment > 0:
		costType = CostIncludedAndOverage
	case overageInSegment > 0:
		costType = CostOverage
	}

	return total, costType, strings.Join(calcParts, " + ")
}

func parseSubscriptionDate(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			utcRaw := t.UTC()
			utc := time.Date(utcRaw.Year(), utcRaw.Month(), utcRaw.Day(), 0, 0, 0, 0, time.UTC)
			return &utc
		}
	}
	return nil
}

func truncateToUTCMonth(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func filterRowsInRange(rows []UsageHistory, start, end time.Time) []UsageHistory {
	out := make([]UsageHistory, 0, len(rows))
	for _, row := range rows {
		ts, err := time.Parse(time.RFC3339, row.Timestamp)
		if err != nil {
			continue
		}
		if (ts.Equal(start) || ts.After(start)) && ts.Before(end) {
			out = append(out, row)
		}
	}
	return out
}

func sumResponsesInRange(rows []UsageHistory, start, end time.Time) int {
	total := 0
	for _, row := range rows {
		ts, err := time.Parse(time.RFC3339, row.Timestamp)
		if err != nil {
			continue
		}
		if (ts.Equal(start) || ts.After(start)) && ts.Before(end) {
			total += row.Responses
		}
	}
	return total
}

func sumResponses(rows []UsageHistory) int {
	total := 0
	for _, row := range rows {
		total += row.Responses
	}
	return total
}

func daysInRange(start, end time.Time) int {
	d := int(end.Sub(start).Hours() / 24)
	if d < 1 {
		return 1
	}
	return d
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (c *CurrentSubscriptionWindow) GetStartDate() string {
	if c == nil {
		return ""
	}
	return c.StartDate
}

func (c *CurrentSubscriptionWindow) GetRenewalDate() string {
	if c == nil {
		return ""
	}
	return c.RenewalDate
}
