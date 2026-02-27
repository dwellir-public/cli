package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/glamour"
	"github.com/jedib0t/go-pretty/v6/table"
	"golang.org/x/term"

	"github.com/dwellir-public/cli/internal/api"
)

type HumanFormatter struct {
	w          io.Writer
	mdRenderer *glamour.TermRenderer
}

type endpointTableRow struct {
	chain     string
	ecosystem string
	network   string
	nodeType  string
	protocol  string
	endpoint  string
}

func NewHumanFormatter(w io.Writer) *HumanFormatter {
	return &HumanFormatter{w: w}
}

func (f *HumanFormatter) Success(command string, data interface{}) error {
	switch command {
	case "keys.list":
		return f.writeKeysList(data)
	case "keys.create", "keys.update", "keys.enable", "keys.disable":
		return f.writeSingleKey(data)
	case "keys.delete":
		return f.Write(data)
	case "usage.summary":
		return f.writeUsageSummary(data)
	case "usage.history":
		return f.writeUsageHistory(data)
	case "usage.rps":
		return f.writeUsageRPS(data)
	case "usage.methods":
		return f.writeUsageBreakdown(data)
	case "usage.costs":
		return f.writeUsageCosts(data)
	case "logs.errors":
		return f.writeLogsErrors(data)
	case "logs.stats":
		return f.writeLogsStats(data)
	case "logs.facets":
		return f.writeLogsFacets(data)
	case "endpoints.list", "endpoints.search", "endpoints.get":
		return f.writeEndpoints(data)
	case "account.info":
		return f.writeAccountInfo(data)
	case "account.subscription":
		return f.writeSubscription(data)
	case "docs.list", "docs.search":
		return f.writeDocsEntries(data)
	case "docs.get":
		return f.writeDocsPage(data)
	case "auth.login", "auth.logout", "auth.status", "config.set", "config.get", "config.list", "version", "update":
		return f.Write(data)
	default:
		return f.Write(data)
	}
}

func (f *HumanFormatter) Error(code string, message string, help string) error {
	if _, err := fmt.Fprintf(f.w, "Error: %s\n", message); err != nil {
		return err
	}
	if help != "" {
		if _, err := fmt.Fprintf(f.w, "\n%s\n", help); err != nil {
			return err
		}
	}
	return &RenderedError{Message: message}
}

func (f *HumanFormatter) Write(data interface{}) error {
	switch v := data.(type) {
	case map[string]string:
		rows := make([][2]string, 0, len(v))
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			rows = append(rows, [2]string{humanizeKey(key), v[key]})
		}
		return f.renderKeyValueRows(rows)
	case map[string]interface{}:
		return f.writeKeyValue(v)
	default:
		enc := json.NewEncoder(f.w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
}

func (f *HumanFormatter) writeKeysList(data interface{}) error {
	keys, ok := data.([]api.APIKey)
	if !ok {
		return f.Write(data)
	}
	if len(keys) == 0 {
		_, err := fmt.Fprintln(f.w, "No API keys found.")
		return err
	}
	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"API Key", "Name", "Enabled", "Daily Quota", "Monthly Quota", "Created At", "Updated At"})
	for _, key := range keys {
		tw.AppendRow(f.formatTableRow(table.Row{
			key.APIKey,
			key.Name,
			yesNo(key.Enabled),
			formatQuota(key.DailyQuota),
			formatQuota(key.MonthlyQuota),
			key.CreatedAt,
			key.UpdatedAt,
		}))
	}
	return f.renderTable(tw)
}

func (f *HumanFormatter) writeSingleKey(data interface{}) error {
	key, ok := data.(*api.APIKey)
	if !ok {
		if direct, directOK := data.(api.APIKey); directOK {
			key = &direct
			ok = true
		}
	}
	if !ok || key == nil {
		return f.Write(data)
	}
	return f.renderKeyValueRows([][2]string{
		{"API key", key.APIKey},
		{"Name", key.Name},
		{"Enabled", yesNo(key.Enabled)},
		{"Daily quota", formatQuota(key.DailyQuota)},
		{"Monthly quota", formatQuota(key.MonthlyQuota)},
		{"Created at", key.CreatedAt},
		{"Updated at", key.UpdatedAt},
	})
}

func (f *HumanFormatter) writeKeyValue(data map[string]interface{}) error {
	rows := make([][2]string, 0, len(data))
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		rows = append(rows, [2]string{humanizeKey(key), fmt.Sprint(data[key])})
	}
	return f.renderKeyValueRows(rows)
}

func (f *HumanFormatter) writeUsageSummary(data interface{}) error {
	summary, ok := data.(*api.UsageSummary)
	if !ok {
		if direct, directOK := data.(api.UsageSummary); directOK {
			summary = &direct
			ok = true
		}
	}
	if !ok || summary == nil {
		return f.Write(data)
	}
	rows := [][2]string{
		{"Total requests", fmt.Sprintf("%d", summary.TotalRequests)},
		{"Total responses", fmt.Sprintf("%d", summary.TotalResponses)},
		{"Rate limited", fmt.Sprintf("%d", summary.RateLimited)},
	}
	if summary.BillingStart != "" {
		rows = append(rows, [2]string{"Billing start", summary.BillingStart})
	}
	if summary.BillingEnd != "" {
		rows = append(rows, [2]string{"Billing end", summary.BillingEnd})
	}
	return f.renderKeyValueRows(rows)
}

func (f *HumanFormatter) writeUsageHistory(data interface{}) error {
	if breakdown, ok := data.([]api.UsageBreakdown); ok {
		return f.writeUsageBreakdown(breakdown)
	}

	history, ok := data.([]api.UsageHistory)
	if !ok {
		return f.Write(data)
	}
	if len(history) == 0 {
		_, err := fmt.Fprintln(f.w, "No usage history found.")
		return err
	}
	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Timestamp", "Requests", "Responses"})
	for _, h := range history {
		tw.AppendRow(f.formatTableRow(table.Row{h.Timestamp, h.Requests, h.Responses}))
	}
	return f.renderTable(tw)
}

func (f *HumanFormatter) writeUsageRPS(data interface{}) error {
	points, ok := data.([]api.RPSData)
	if !ok {
		return f.Write(data)
	}
	if len(points) == 0 {
		_, err := fmt.Fprintln(f.w, "No RPS data found.")
		return err
	}
	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Timestamp", "RPS"})
	for _, point := range points {
		tw.AppendRow(f.formatTableRow(table.Row{point.Timestamp, fmt.Sprintf("%.2f", point.RPS)}))
	}
	return f.renderTable(tw)
}

func (f *HumanFormatter) writeUsageBreakdown(data interface{}) error {
	breakdown, ok := data.([]api.UsageBreakdown)
	if !ok {
		return f.Write(data)
	}
	if len(breakdown) == 0 {
		_, err := fmt.Fprintln(f.w, "No usage data found.")
		return err
	}
	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Group", "Requests", "Responses", "Rate Limited"})
	for _, row := range breakdown {
		tw.AppendRow(f.formatTableRow(table.Row{row.Group, row.Requests, row.Responses, row.RateLimited}))
	}
	return f.renderTable(tw)
}

func (f *HumanFormatter) writeUsageCosts(data interface{}) error {
	report, ok := data.(api.CostReport)
	if !ok {
		if ptr, ptrOK := data.(*api.CostReport); ptrOK && ptr != nil {
			report = *ptr
			ok = true
		}
	}
	if !ok {
		return f.Write(data)
	}

	if !report.Supported {
		if report.UnsupportedHint != "" {
			_, err := fmt.Fprintf(f.w, "%s\n", report.UnsupportedHint)
			return err
		}
		_, err := fmt.Fprintln(f.w, "Your plan does not support usage-based cost breakdown.")
		return err
	}

	if err := f.renderKeyValueRows([][2]string{
		{"Plan", report.PlanName},
		{"Interval start", report.IntervalStart},
		{"Interval end", report.IntervalEnd},
		{"Total responses", fmt.Sprintf("%d", report.TotalResponses)},
		{"Total cost (USD)", fmt.Sprintf("$%.2f", report.TotalCost)},
	}); err != nil {
		return err
	}

	if len(report.ByDomain) > 0 {
		if _, err := fmt.Fprintln(f.w); err != nil {
			return err
		}
		tw := table.NewWriter()
		tw.AppendHeader(table.Row{"Domain", "Responses", "Cost (USD)"})
		for _, row := range report.ByDomain {
			tw.AppendRow(f.formatTableRow(table.Row{row.Group, row.Responses, fmt.Sprintf("$%.2f", row.Cost)}))
		}
		if err := f.renderTable(tw); err != nil {
			return err
		}
	}

	if len(report.Segments) > 0 {
		if _, err := fmt.Fprintln(f.w); err != nil {
			return err
		}
		tw := table.NewWriter()
		tw.AppendHeader(table.Row{"Segment Start", "Segment End", "Responses", "Cost (USD)", "Type"})
		for _, segment := range report.Segments {
			tw.AppendRow(f.formatTableRow(table.Row{
				segment.Start,
				segment.End,
				segment.Responses,
				fmt.Sprintf("$%.2f", segment.Cost),
				segment.CostType,
			}))
		}
		return f.renderTable(tw)
	}

	return nil
}

func (f *HumanFormatter) writeLogsErrors(data interface{}) error {
	logs, ok := data.([]api.ErrorLog)
	if !ok {
		return f.Write(data)
	}
	if len(logs) == 0 {
		_, err := fmt.Fprintln(f.w, "No error logs found.")
		return err
	}
	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Timestamp", "Status", "RPC Methods", "Endpoint", "Message"})
	for _, row := range logs {
		tw.AppendRow(f.formatTableRow(table.Row{
			row.Timestamp,
			fmt.Sprintf("%d %s", row.StatusCode, row.StatusLabel),
			row.RPCMethods,
			row.FQDN,
			row.ErrorMessage,
		}))
	}
	return f.renderTable(tw)
}

func (f *HumanFormatter) writeLogsStats(data interface{}) error {
	stats, ok := data.([]api.ErrorStats)
	if !ok {
		return f.Write(data)
	}
	if len(stats) == 0 {
		_, err := fmt.Fprintln(f.w, "No log statistics found.")
		return err
	}
	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Status Code", "Count"})
	for _, row := range stats {
		tw.AppendRow(f.formatTableRow(table.Row{row.StatusCode, row.Count}))
	}
	return f.renderTable(tw)
}

func (f *HumanFormatter) writeLogsFacets(data interface{}) error {
	facets, ok := data.(*api.ErrorFacets)
	if !ok {
		if direct, directOK := data.(api.ErrorFacets); directOK {
			facets = &direct
			ok = true
		}
	}
	if !ok || facets == nil {
		return f.Write(data)
	}

	sections := []struct {
		title   string
		entries []api.FacetEntry
	}{
		{title: "FQDNs", entries: facets.FQDNs},
		{title: "RPC Methods", entries: facets.RPCMethods},
		{title: "Origins", entries: facets.Origins},
		{title: "API Keys", entries: facets.APIKeys},
	}

	hasContent := false
	for _, section := range sections {
		if len(section.entries) == 0 {
			continue
		}
		hasContent = true
		if _, err := fmt.Fprintf(f.w, "%s\n", section.title); err != nil {
			return err
		}
		tw := table.NewWriter()
		tw.AppendHeader(table.Row{"Value", "Count"})
		for _, row := range section.entries {
			tw.AppendRow(f.formatTableRow(table.Row{row.Value, row.Count}))
		}
		if err := f.renderTable(tw); err != nil {
			return err
		}
	}

	if !hasContent {
		_, err := fmt.Fprintln(f.w, "No facet data found.")
		return err
	}
	return nil
}

func (f *HumanFormatter) writeEndpoints(data interface{}) error {
	chains, ok := data.([]api.Chain)
	if !ok {
		return f.Write(data)
	}

	rows := make([]endpointTableRow, 0)
	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Chain", "Ecosystem", "Network", "Node Type", "Protocol", "Endpoint"})

	for _, chain := range chains {
		for _, network := range chain.Networks {
			for _, node := range network.Nodes {
				if node.HTTPS != "" {
					rows = append(rows, endpointTableRow{
						chain:     chain.Name,
						ecosystem: chain.Ecosystem,
						network:   network.Name,
						nodeType:  node.NodeType.Name,
						protocol:  "https",
						endpoint:  node.HTTPS,
					})
				}
				if node.WSS != "" {
					rows = append(rows, endpointTableRow{
						chain:     chain.Name,
						ecosystem: chain.Ecosystem,
						network:   network.Name,
						nodeType:  node.NodeType.Name,
						protocol:  "wss",
						endpoint:  node.WSS,
					})
				}
			}
		}
	}

	if len(rows) == 0 {
		_, err := fmt.Fprintln(f.w, "No endpoints found.")
		return err
	}

	if width := terminalWidthFromWriter(f.w); width > 0 && width < 130 {
		return f.writeEndpointsCompact(rows, width)
	}

	for _, row := range rows {
		tw.AppendRow(f.formatTableRow(table.Row{
			row.chain,
			row.ecosystem,
			row.network,
			row.nodeType,
			row.protocol,
			row.endpoint,
		}))
	}

	return f.renderTable(tw)
}

func (f *HumanFormatter) writeAccountInfo(data interface{}) error {
	info, ok := data.(*api.AccountInfo)
	if !ok {
		if direct, directOK := data.(api.AccountInfo); directOK {
			info = &direct
			ok = true
		}
	}
	if !ok || info == nil {
		return f.Write(data)
	}

	rows := [][2]string{}
	if info.Name != "" {
		rows = append(rows, [2]string{"Name", info.Name})
	}
	if info.ServerLocation != "" {
		rows = append(rows, [2]string{"Server location", info.ServerLocation})
	}
	if info.TaxID != "" {
		rows = append(rows, [2]string{"Tax ID", info.TaxID})
	}
	if info.Subscription != nil {
		if strings.TrimSpace(info.Subscription.PlanName) != "" {
			rows = append(rows, [2]string{"Subscription plan", info.Subscription.PlanName})
		}
		rows = append(rows,
			[2]string{"Rate limit", fmt.Sprintf("%d", info.Subscription.RateLimit)},
			[2]string{"Burst limit", fmt.Sprintf("%d", info.Subscription.BurstLimit)},
			[2]string{"API keys limit", fmt.Sprintf("%d", info.Subscription.APIKeysLimit)},
			[2]string{"Daily quota", formatQuota(info.Subscription.DailyQuota)},
			[2]string{"Monthly quota", formatQuota(info.Subscription.MonthlyQuota)},
		)
	}
	if len(rows) == 0 {
		_, err := fmt.Fprintln(f.w, "No account information available.")
		return err
	}
	return f.renderKeyValueRows(rows)
}

func (f *HumanFormatter) writeSubscription(data interface{}) error {
	sub, ok := data.(*api.SubscriptionInfo)
	if !ok {
		if direct, directOK := data.(api.SubscriptionInfo); directOK {
			sub = &direct
			ok = true
		}
	}
	if !ok || sub == nil {
		return f.Write(data)
	}
	rows := make([][2]string, 0, 6)
	if strings.TrimSpace(sub.PlanName) != "" {
		rows = append(rows, [2]string{"Plan", sub.PlanName})
	}
	rows = append(rows,
		[2]string{"Rate limit", fmt.Sprintf("%d", sub.RateLimit)},
		[2]string{"Burst limit", fmt.Sprintf("%d", sub.BurstLimit)},
		[2]string{"API keys limit", fmt.Sprintf("%d", sub.APIKeysLimit)},
		[2]string{"Daily quota", formatQuota(sub.DailyQuota)},
		[2]string{"Monthly quota", formatQuota(sub.MonthlyQuota)},
	)
	return f.renderKeyValueRows(rows)
}

func (f *HumanFormatter) writeDocsEntries(data interface{}) error {
	entries, ok := data.([]api.DocsEntry)
	if !ok {
		return f.Write(data)
	}
	if len(entries) == 0 {
		_, err := fmt.Fprintln(f.w, "No docs pages found.")
		return err
	}

	if width := terminalWidthFromWriter(f.w); width > 0 && width < 120 {
		for _, entry := range entries {
			title := truncateWithEllipsis(entry.Title, 72)
			section := truncateWithEllipsis(entry.Section, 40)
			description := truncateWithEllipsis(entry.Description, 92)
			if _, err := fmt.Fprintf(f.w, "%s (%s) · %s\n", title, entry.Slug, section); err != nil {
				return err
			}
			if description != "" {
				if _, err := fmt.Fprintf(f.w, "  %s\n", description); err != nil {
					return err
				}
			}
		}
		return nil
	}

	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Title", "Slug", "Section", "Description"})
	for _, entry := range entries {
		tw.AppendRow(f.formatTableRow(table.Row{entry.Title, entry.Slug, entry.Section, entry.Description}))
	}
	return f.renderTable(tw)
}

func (f *HumanFormatter) writeDocsPage(data interface{}) error {
	page, ok := data.(api.DocsPage)
	if !ok {
		if ptr, ptrOK := data.(*api.DocsPage); ptrOK && ptr != nil {
			page = *ptr
			ok = true
		}
	}
	if !ok {
		return f.Write(data)
	}

	if err := f.renderKeyValueRows([][2]string{
		{"Title", page.Title},
		{"Slug", page.Slug},
		{"URL", page.URL},
	}); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f.w); err != nil {
		return err
	}

	rendered, err := f.renderMarkdown(page.Content)
	if err != nil {
		_, writeErr := fmt.Fprintln(f.w, page.Content)
		return writeErr
	}
	if !strings.HasSuffix(rendered, "\n") {
		rendered += "\n"
	}
	_, writeErr := io.WriteString(f.w, rendered)
	return writeErr
}

func (f *HumanFormatter) renderMarkdown(content string) (string, error) {
	if f.mdRenderer == nil {
		renderer, err := glamour.NewTermRenderer(
			glamour.WithStandardStyle("notty"),
			glamour.WithWordWrap(0),
		)
		if err != nil {
			return "", err
		}
		f.mdRenderer = renderer
	}
	return f.mdRenderer.Render(content)
}

func (f *HumanFormatter) renderTable(tw table.Writer) error {
	style := table.StyleLight
	style.Options = table.OptionsNoBordersAndSeparators
	tw.SetStyle(style)
	tw.SuppressTrailingSpaces()
	out := tw.Render()
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	_, err := io.WriteString(f.w, out)
	return err
}

func (f *HumanFormatter) renderKeyValueRows(rows [][2]string) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(f.w, "No data.")
		return err
	}
	tw := table.NewWriter()
	for _, row := range rows {
		tw.AppendRow(f.formatTableRow(table.Row{row[0], row[1]}))
	}
	return f.renderTable(tw)
}

func formatQuota(v *int) string {
	if v == nil {
		return "Unlimited"
	}
	return formatInt64(int64(*v))
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func humanizeKey(raw string) string {
	parts := strings.Split(strings.ReplaceAll(raw, "_", " "), " ")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func (f *HumanFormatter) writeEndpointsCompact(rows []endpointTableRow, width int) error {
	_ = width // reserved for future width-aware tuning
	for _, row := range rows {
		if _, err := fmt.Fprintf(
			f.w,
			"%s · %s · %s · %s · %s\n  %s\n",
			truncateWithEllipsis(row.chain, 22),
			truncateWithEllipsis(row.ecosystem, 12),
			truncateWithEllipsis(row.network, 24),
			truncateWithEllipsis(row.nodeType, 12),
			row.protocol,
			row.endpoint,
		); err != nil {
			return err
		}
	}
	return nil
}

func terminalWidthFromWriter(w io.Writer) int {
	file, ok := w.(*os.File)
	if !ok {
		return 0
	}
	width, _, err := term.GetSize(int(file.Fd()))
	if err != nil || width <= 0 {
		return 0
	}
	return width
}

func truncateWithEllipsis(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	if limit == 1 {
		return "…"
	}
	runes := []rune(value)
	return string(runes[:limit-1]) + "…"
}

func (f *HumanFormatter) formatTableRow(row table.Row) table.Row {
	formatted := make(table.Row, 0, len(row))
	for _, cell := range row {
		formatted = append(formatted, formatHumanValue(cell))
	}
	return formatted
}

func formatHumanValue(v interface{}) interface{} {
	switch t := v.(type) {
	case int:
		return formatInt64(int64(t))
	case int8:
		return formatInt64(int64(t))
	case int16:
		return formatInt64(int64(t))
	case int32:
		return formatInt64(int64(t))
	case int64:
		return formatInt64(t)
	case uint:
		return formatUint64(uint64(t))
	case uint8:
		return formatUint64(uint64(t))
	case uint16:
		return formatUint64(uint64(t))
	case uint32:
		return formatUint64(uint64(t))
	case uint64:
		return formatUint64(t)
	case string:
		return formatNumericString(t)
	default:
		return v
	}
}

func formatNumericString(raw string) string {
	if raw == "" {
		return raw
	}
	if raw[0] == '-' {
		if _, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return "-" + formatUintString(strings.TrimPrefix(raw, "-"))
		}
		return raw
	}
	if _, err := strconv.ParseUint(raw, 10, 64); err == nil {
		return formatUintString(raw)
	}
	return raw
}

func formatInt64(v int64) string {
	raw := strconv.FormatInt(v, 10)
	if strings.HasPrefix(raw, "-") {
		return "-" + formatUintString(strings.TrimPrefix(raw, "-"))
	}
	return formatUintString(raw)
}

func formatUint64(v uint64) string {
	return formatUintString(strconv.FormatUint(v, 10))
}

func formatUintString(raw string) string {
	if len(raw) <= 3 {
		return raw
	}
	parts := make([]string, 0, (len(raw)+2)/3)
	for len(raw) > 3 {
		parts = append(parts, raw[len(raw)-3:])
		raw = raw[:len(raw)-3]
	}
	parts = append(parts, raw)
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, ",")
}
