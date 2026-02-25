package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/auth"
	"github.com/dwellir-public/cli/internal/config"
)

const defaultDashboardURL = "https://dashboard.dwellir.com"

var tokenFlag string

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Dwellir",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate via browser or token",
	Long:  "Opens a browser window for authentication.\n\nFor headless/CI: dwellir auth login --token <TOKEN>",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		f := getFormatter()

		if tokenFlag != "" {
			profileName := profile
			if profileName == "" {
				profileName = "default"
			}
			p := &config.Profile{
				Name:  profileName,
				Token: tokenFlag,
			}
			if err := config.SaveProfile(configDir, p); err != nil {
				return err
			}
			return f.Success("auth.login", map[string]string{
				"status":  "authenticated",
				"profile": profileName,
				"method":  "token",
			})
		}

		dashboardURL := os.Getenv("DWELLIR_DASHBOARD_URL")
		if dashboardURL == "" {
			dashboardURL = defaultDashboardURL
		}

		p, err := auth.Login(configDir, profile, dashboardURL)
		if err != nil {
			return f.Error("auth_failed", err.Error(), "")
		}

		return f.Success("auth.login", map[string]string{
			"status":  "authenticated",
			"profile": p.Name,
			"user":    p.User,
			"org":     p.Org,
			"method":  "browser",
		})
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear local credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		f := getFormatter()
		profileName := profile
		if profileName == "" {
			profileName = "default"
		}
		profilePath := filepath.Join(configDir, "profiles", profileName+".json")
		_ = os.Remove(profilePath)
		return f.Success("auth.logout", map[string]string{
			"status":  "logged_out",
			"profile": profileName,
		})
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current auth state",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		f := getFormatter()

		cwd, _ := os.Getwd()
		envProfile := os.Getenv("DWELLIR_PROFILE")
		profileName := config.ResolveProfileName(profile, envProfile, cwd, configDir)

		p, err := config.LoadProfile(configDir, profileName)
		if err != nil {
			return f.Error("not_authenticated", "No active session.", "Run 'dwellir auth login' to authenticate.")
		}

		return f.Success("auth.status", map[string]string{
			"profile": profileName,
			"user":    p.User,
			"org":     p.Org,
		})
	},
}

var authTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Print current access token",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		cwd, _ := os.Getwd()
		token, err := auth.ResolveToken(tokenFlag, profile, cwd, configDir)
		if err != nil {
			f := getFormatter()
			return f.Error("not_authenticated", err.Error(), "")
		}
		fmt.Fprintln(cmd.OutOrStdout(), token)
		return nil
	},
}

func init() {
	authLoginCmd.Flags().StringVar(&tokenFlag, "token", "", "Authenticate with a CLI token directly (for headless/CI)")
	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authStatusCmd, authTokenCmd)
	rootCmd.AddCommand(authCmd)
}
