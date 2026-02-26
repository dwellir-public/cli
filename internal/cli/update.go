package cli

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"
)

const repoSlug = "dwellir-public/cli"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update CLI to latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := getFormatter()

		latest, found, err := selfupdate.DetectLatest(cmd.Context(), selfupdate.ParseSlug(repoSlug))
		if err != nil {
			return f.Error("update_failed", fmt.Sprintf("Failed to check for updates: %v", err), "")
		}
		if !found {
			return f.Error("update_failed", "No release found.", "")
		}

		upToDate, err := isLatestVersion(Version, latest.Version())
		if err != nil {
			return f.Error("update_failed", fmt.Sprintf("Failed to compare versions: %v", err), "")
		}
		if upToDate {
			return f.Success("update", map[string]string{
				"status":  "up_to_date",
				"version": Version,
			})
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "Updating to v%s...\n", latest.Version())
		cmdPath, err := os.Executable()
		if err != nil {
			return f.Error("update_failed", fmt.Sprintf("Unable to resolve executable path: %v", err), "")
		}
		if err := selfupdate.UpdateTo(cmd.Context(), latest.AssetURL, latest.AssetName, cmdPath); err != nil {
			help := "Try downloading manually from GitHub releases."
			if hint := detectManagedInstallHint(cmdPath); hint != "" {
				help = hint
			}
			return f.Error("update_failed", fmt.Sprintf("Update failed: %v", err), help)
		}

		return f.Success("update", map[string]string{
			"status":       "updated",
			"from_version": Version,
			"to_version":   latest.Version(),
		})
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func isLatestVersion(currentVersion string, latestVersion string) (bool, error) {
	current, ok, err := parseSemanticVersion(currentVersion)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	latest, ok, err := parseSemanticVersion(latestVersion)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("invalid latest version: %q", latestVersion)
	}

	return !latest.GreaterThan(current), nil
}

func parseSemanticVersion(version string) (*semver.Version, bool, error) {
	cleaned := strings.TrimSpace(version)
	cleaned = strings.TrimPrefix(cleaned, "v")
	if cleaned == "" || cleaned == "dev" || cleaned == "unknown" {
		return nil, false, nil
	}

	parsed, err := semver.NewVersion(cleaned)
	if err != nil {
		return nil, false, nil
	}
	return parsed, true, nil
}

func detectManagedInstallHint(executablePath string) string {
	return detectManagedInstallHintWith(executablePath, exec.LookPath, runCommandOutput)
}

func detectManagedInstallHintWith(
	executablePath string,
	lookPath func(file string) (string, error),
	runCmd func(name string, args ...string) (string, error),
) string {
	path := strings.TrimSpace(executablePath)
	if path == "" {
		if discovered, err := lookPath("dwellir"); err == nil {
			path = strings.TrimSpace(discovered)
		}
	}

	if _, err := lookPath("pacman"); err == nil && path != "" {
		if out, runErr := runCmd("pacman", "-Qo", path); runErr == nil {
			if pkg := parsePacmanOwnedPackage(out); pkg != "" {
				return fmt.Sprintf(
					"This installation is managed by pacman/AUR (%s). Run `yay -Syu %s` (or `sudo pacman -Syu %s`).",
					pkg,
					pkg,
					pkg,
				)
			}
			return "This installation is managed by pacman/AUR. Run `yay -Syu dwellir-cli-bin` (or `sudo pacman -Syu dwellir-cli-bin`)."
		}
	}

	if _, err := lookPath("brew"); err == nil {
		if _, runErr := runCmd("brew", "list", "--formula", "dwellir"); runErr == nil {
			return "This installation is managed by Homebrew. Run `brew upgrade dwellir`."
		}
	}

	return ""
}

func parsePacmanOwnedPackage(output string) string {
	line := strings.TrimSpace(output)
	if line == "" {
		return ""
	}

	re := regexp.MustCompile(` is owned by ([^ ]+) `)
	matches := re.FindStringSubmatch(line)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func runCommandOutput(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}
