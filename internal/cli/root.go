package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/dwellir-public/cli/internal/config"
	"github.com/dwellir-public/cli/internal/output"
	"github.com/dwellir-public/cli/internal/telemetry"
)

var (
	jsonOutput    bool
	humanOutput   bool
	toonOutput    bool
	profile       string
	quiet         bool
	anonTelemetry bool
)

var stdoutIsTerminal = func() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

type telemetryTracker interface {
	Init(ver string, user string, org string, device string, anon bool)
	Identify(extra map[string]interface{})
	TrackCommand(command string, extra map[string]interface{})
	Close()
}

type posthogTelemetryTracker struct{}

func (posthogTelemetryTracker) Init(ver string, user string, org string, device string, anon bool) {
	telemetry.Init(ver, user, org, device, anon)
}

func (posthogTelemetryTracker) Identify(extra map[string]interface{}) {
	telemetry.Identify(extra)
}

func (posthogTelemetryTracker) TrackCommand(command string, extra map[string]interface{}) {
	telemetry.TrackCommand(command, extra)
}

func (posthogTelemetryTracker) Close() {
	telemetry.Close()
}

var telemetryClient telemetryTracker = posthogTelemetryTracker{}

var currentRun telemetryRunState

type telemetryRunState struct {
	startedAt    time.Time
	command      string
	outputFormat string
	telemetryOn  bool
	identified   bool
	anonymous    bool
}

var rootCmd = &cobra.Command{
	Use:   "dwellir",
	Short: "Dwellir CLI — Blockchain RPC infrastructure from your terminal",
	Long: `Dwellir CLI provides full access to the Dwellir platform.

Manage API keys, browse blockchain endpoints, view usage analytics,
and debug error logs — all from the command line.

Get started:
  dwellir auth login       Authenticate with your Dwellir account
  dwellir docs search rpc  Search Dwellir documentation
  dwellir endpoints list   Browse available blockchain endpoints
  dwellir keys list        List your API keys

Documentation: https://dwellir.com/docs
Dashboard:     https://dashboard.dwellir.com`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&humanOutput, "human", false, "Output as human-readable (default)")
	rootCmd.PersistentFlags().BoolVar(&toonOutput, "toon", false, "Output as TOON")
	rootCmd.PersistentFlags().StringVar(&profile, "profile", "", "Use a specific auth profile")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&anonTelemetry, "anon-telemetry", false, "Anonymize telemetry data")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		startTelemetryRun(cmd)
	}
	rootCmd.PersistentPostRun = func(cmd *cobra.Command, args []string) {
		trackTelemetryRunResult(true, "")
	}
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

func Execute() error {
	currentRun = telemetryRunState{
		startedAt:    time.Now(),
		command:      inferCommandFromArgs(os.Args[1:]),
		outputFormat: outputFormatFromArgs(os.Args[1:]),
	}
	defer telemetryClient.Close()

	if err := rootCmd.Execute(); err != nil {
		errorCode := "error"
		var renderedErr *output.RenderedError
		if errors.As(err, &renderedErr) && renderedErr != nil && renderedErr.Code != "" {
			errorCode = renderedErr.Code
		} else {
			errorCode, _, _ = classifyExecutionError(err)
		}
		trackTelemetryRunResult(false, errorCode)

		if output.IsRenderedError(err) {
			return err
		}
		code, message, help := classifyExecutionError(err)
		f := getFormatter()
		if explicit := explicitOutputFromArgs(os.Args[1:]); explicit != "" {
			f = output.New(explicit, rootCmd.OutOrStdout())
		}
		return f.Error(code, message, help)
	}
	return nil
}

func explicitOutputFromArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	format := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			format = "json"
		case "--human":
			format = "human"
		case "--toon":
			format = "toon"
		}
	}
	return format
}

func buildFormatter(format string) output.Formatter {
	return output.New(format, rootCmd.OutOrStdout())
}

func resolvedOutputFormat() string {
	configDir := config.DefaultConfigDir()
	format := "human"
	cfg, err := config.Load(configDir)
	if err == nil && cfg != nil && cfg.Output != "" {
		format = cfg.Output
	}
	if shouldAutoSelectStructuredOutput() && !configFileExists(configDir) {
		format = "toon"
	}
	if jsonOutput {
		format = "json"
	}
	if humanOutput {
		format = "human"
	}
	if toonOutput {
		format = "toon"
	}
	return format
}

func isHumanOutput() bool {
	return resolvedOutputFormat() == "human"
}

func getFormatter() output.Formatter {
	return buildFormatter(resolvedOutputFormat())
}

func isAgentEnvironment() bool {
	markers := [...]string{
		"CODEX_CI",
		"CODEX_THREAD_ID",
		"CLAUDECODE",
		"CLAUDE_CODE_ENTRYPOINT",
		"OPENCODE",
		"CURSOR_AGENT",
	}
	for _, key := range markers {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

func shouldAutoSelectStructuredOutput() bool {
	return isAgentEnvironment() && !stdoutIsTerminal()
}

func configFileExists(configDir string) bool {
	_, err := os.Stat(filepath.Join(configDir, "config.json"))
	return err == nil
}

func startTelemetryRun(cmd *cobra.Command) {
	if currentRun.startedAt.IsZero() {
		currentRun.startedAt = time.Now()
	}
	currentRun.command = normalizeCommandPath(cmd.CommandPath())
	currentRun.outputFormat = resolvedOutputFormat()
	initializeTelemetry(profile, anonTelemetry)
}

func trackTelemetryRunResult(success bool, errorCode string) {
	if !currentRun.telemetryOn {
		initializeTelemetryFromArgs(os.Args[1:])
	}

	command := currentRun.command
	if command == "" {
		command = inferCommandFromArgs(os.Args[1:])
	}
	format := currentRun.outputFormat
	if format == "" {
		format = outputFormatFromArgs(os.Args[1:])
	}

	props := map[string]interface{}{
		"success":         success,
		"output_format":   format,
		"duration_ms":     time.Since(currentRun.startedAt).Milliseconds(),
		"agent_env":       isAgentEnvironment(),
		"stdout_terminal": stdoutIsTerminal(),
		"arg_count":       len(os.Args) - 1,
		"identified_user": currentRun.identified,
		"is_anonymous":    currentRun.anonymous,
	}
	if !success {
		if errorCode != "" {
			props["error_code"] = errorCode
		}
		if unknown := extractUnknownCommand(os.Args[1:]); unknown != "" {
			props["unknown_command"] = unknown
		}
	}

	telemetryClient.TrackCommand(command, props)
}

func outputFormatFromArgs(args []string) string {
	if explicit := explicitOutputFromArgs(args); explicit != "" {
		return explicit
	}
	return resolvedOutputFormat()
}

func initializeTelemetryFromArgs(args []string) {
	overrideProfile := profileFromArgs(args)
	overrideAnon := anonTelemetry
	if parsed, ok := anonTelemetryFromArgs(args); ok {
		overrideAnon = parsed
	}
	initializeTelemetry(overrideProfile, overrideAnon)
}

func initializeTelemetry(profileOverride string, anon bool) {
	configDir := config.DefaultConfigDir()
	user, org, profileName := resolveTelemetryIdentity(configDir, profileOverride)
	deviceID := ensureTelemetryDeviceID(configDir)
	telemetryClient.Init(Version, user, org, deviceID, anon)
	telemetryClient.Identify(map[string]interface{}{
		"profile":         profileName,
		"is_anonymous":    anon || user == "",
		"agent_env":       isAgentEnvironment(),
		"stdout_terminal": stdoutIsTerminal(),
	})
	currentRun.telemetryOn = true
	currentRun.identified = user != ""
	currentRun.anonymous = anon || user == ""
}

func resolveTelemetryIdentity(configDir string, profileOverride string) (user string, org string, profileName string) {
	cwd, _ := os.Getwd()
	envProfile := os.Getenv("DWELLIR_PROFILE")
	resolvedProfile := profile
	if strings.TrimSpace(profileOverride) != "" {
		resolvedProfile = strings.TrimSpace(profileOverride)
	}
	profileName = config.ResolveProfileName(resolvedProfile, envProfile, cwd, configDir)

	p, err := config.LoadProfile(configDir, profileName)
	if err != nil || p == nil {
		return "", "", profileName
	}
	return strings.TrimSpace(p.User), strings.TrimSpace(p.Org), profileName
}

func ensureTelemetryDeviceID(configDir string) string {
	path := filepath.Join(configDir, "device_id")
	if b, err := os.ReadFile(path); err == nil {
		if existing := strings.TrimSpace(string(b)); existing != "" {
			return existing
		}
	}
	deviceID := uuid.NewString()
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return deviceID
	}
	if err := os.WriteFile(path, []byte(deviceID+"\n"), 0o600); err != nil {
		return deviceID
	}
	return deviceID
}

func inferCommandFromArgs(args []string) string {
	if cmd, _, err := rootCmd.Find(args); err == nil && cmd != nil {
		name := normalizeCommandPath(cmd.CommandPath())
		if name != "" {
			return name
		}
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if trimmed := strings.TrimSpace(arg); trimmed != "" {
			return trimmed
		}
	}
	return "root"
}

func normalizeCommandPath(path string) string {
	clean := strings.TrimSpace(path)
	clean = strings.TrimPrefix(clean, "dwellir")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return "root"
	}
	clean = strings.Join(strings.Fields(clean), ".")
	return strings.ToLower(clean)
}

func extractUnknownCommand(args []string) string {
	if _, _, err := rootCmd.Find(args); err == nil {
		return ""
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return strings.TrimSpace(arg)
	}
	return ""
}

func profileFromArgs(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "--profile" && i+1 < len(args) {
			return strings.TrimSpace(args[i+1])
		}
		if strings.HasPrefix(arg, "--profile=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, "--profile="))
		}
	}
	return ""
}

func anonTelemetryFromArgs(args []string) (bool, bool) {
	for _, raw := range args {
		arg := strings.TrimSpace(raw)
		if arg == "--anon-telemetry" {
			return true, true
		}
		if strings.HasPrefix(arg, "--anon-telemetry=") {
			value := strings.TrimSpace(strings.TrimPrefix(arg, "--anon-telemetry="))
			return parseBoolFlag(value), true
		}
	}
	return false, false
}

func parseBoolFlag(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return false
	}
}
