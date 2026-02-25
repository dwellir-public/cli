package cli

import (
	"fmt"

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

		if latest.LessOrEqual(Version) {
			return f.Success("update", map[string]string{
				"status":  "up_to_date",
				"version": Version,
			})
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "Updating to v%s...\n", latest.Version())
		if err := selfupdate.UpdateTo(cmd.Context(), latest.AssetURL, latest.AssetName, ""); err != nil {
			return f.Error("update_failed", fmt.Sprintf("Update failed: %v", err), "Try downloading manually from GitHub releases.")
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
