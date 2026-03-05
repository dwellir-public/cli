package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
	"github.com/dwellir-public/cli/internal/auth"
	"github.com/dwellir-public/cli/internal/config"
)

var doctorVerifyAPI bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose auth, profile, and output mode configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		cwd, _ := os.Getwd()
		ctx := resolveProfileContext(profile, cwd, configDir)

		checks := make([]map[string]interface{}, 0, 6)
		checks = append(checks, buildDoctorOutputModeCheck(configDir))
		checks = append(checks, map[string]interface{}{
			"name":    "profile_resolution",
			"status":  "ok",
			"message": "Resolved active profile.",
			"details": map[string]interface{}{"profile": ctx.Name, "source": ctx.Source, "cwd": cwd},
		})

		profilePath := filepath.Join(configDir, "profiles", ctx.Name+".json")
		if _, err := os.Stat(profilePath); err == nil {
			checks = append(checks, map[string]interface{}{
				"name":    "profile_file",
				"status":  "ok",
				"message": "Profile file exists.",
				"details": map[string]interface{}{"profile_path": profilePath},
			})
		} else {
			checks = append(checks, map[string]interface{}{
				"name":    "profile_file",
				"status":  "warn",
				"message": "Profile file not found.",
				"details": map[string]interface{}{"profile_path": profilePath},
			})
		}

		token, tokenErr := auth.ResolveToken("", profile, cwd, configDir)
		if tokenErr != nil {
			checks = append(checks, map[string]interface{}{
				"name":    "token_presence",
				"status":  "error",
				"message": tokenErr.Error(),
				"details": map[string]interface{}{"token_present": false},
			})
		} else {
			checks = append(checks, map[string]interface{}{
				"name":    "token_presence",
				"status":  "ok",
				"message": "Resolved auth token.",
				"details": map[string]interface{}{"token_present": true},
			})
		}

		markers := presentAgentMarkers()
		checks = append(checks, map[string]interface{}{
			"name":    "agent_environment",
			"status":  "ok",
			"message": "Computed agent/TTY detection state.",
			"details": map[string]interface{}{
				"agent_detected":     len(markers) > 0,
				"stdout_terminal":    stdoutIsTerminal(),
				"present_markers":    markers,
				"auto_structured":    shouldAutoSelectStructuredOutput(),
				"config_file_exists": configFileExists(configDir),
			},
		})

		if doctorVerifyAPI {
			if tokenErr != nil {
				checks = append(checks, map[string]interface{}{
					"name":    "api_verification",
					"status":  "error",
					"message": "Skipped API verification because token resolution failed.",
				})
			} else {
				baseURL := os.Getenv("DWELLIR_API_URL")
				if baseURL == "" {
					baseURL = "https://dashboard.dwellir.com/marly-api"
				}
				client := api.NewClient(baseURL, token)
				if _, err := api.NewAccountAPI(client).Info(); err != nil {
					checks = append(checks, map[string]interface{}{
						"name":    "api_verification",
						"status":  "error",
						"message": err.Error(),
					})
				} else {
					checks = append(checks, map[string]interface{}{
						"name":    "api_verification",
						"status":  "ok",
						"message": "Authenticated API call succeeded.",
					})
				}
			}
		}

		summary := map[string]int{"ok": 0, "warn": 0, "error": 0}
		for _, c := range checks {
			status, _ := c["status"].(string)
			summary[status]++
		}

		return getFormatter().Success("doctor", map[string]interface{}{
			"summary": summary,
			"checks":  checks,
		})
	},
}

func buildDoctorOutputModeCheck(configDir string) map[string]interface{} {
	configOutput := ""
	if cfg, err := config.Load(configDir); err == nil && cfg != nil && configFileExists(configDir) {
		configOutput = cfg.Output
	}

	autoDefault := "human"
	autoSelected := false
	if shouldAutoSelectStructuredOutput() && !configFileExists(configDir) {
		autoDefault = "toon"
		autoSelected = true
	}

	modeSource := "default"
	if explicit := explicitOutputFromArgs(os.Args[1:]); explicit != "" {
		modeSource = "flag"
	} else if configOutput != "" {
		modeSource = "config"
	} else if autoSelected {
		modeSource = "auto"
	}

	return map[string]interface{}{
		"name":    "output_mode",
		"status":  "ok",
		"message": "Resolved output mode and precedence inputs.",
		"details": map[string]interface{}{
			"resolved_format":     resolvedOutputFormat(),
			"auto_default_format": autoDefault,
			"source":              modeSource,
			"explicit_flag":       explicitOutputFromArgs(os.Args[1:]),
			"config_output":       configOutput,
			"agent_environment":   isAgentEnvironment(),
			"agent_override":      os.Getenv("DWELLIR_AGENT"),
			"stdout_terminal":     stdoutIsTerminal(),
		},
	}
}

func presentAgentMarkers() []string {
	markers := []string{
		"CODEX_CI",
		"CODEX_THREAD_ID",
		"CLAUDECODE",
		"CLAUDE_CODE_ENTRYPOINT",
		"OPENCODE",
		"CURSOR_AGENT",
	}
	present := make([]string, 0, len(markers))
	for _, key := range markers {
		if os.Getenv(key) != "" {
			present = append(present, key)
		}
	}
	return present
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorVerifyAPI, "verify-api", false, "Perform a live authenticated API check")
	rootCmd.AddCommand(doctorCmd)
}
