package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version info",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := getFormatter()
		return f.Success("version", map[string]string{
			"version":    Version,
			"commit":     Commit,
			"build_date": BuildDate,
			"go_version": runtime.Version(),
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
		})
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.Version = fmt.Sprintf("%s (%s)", Version, Commit)
}
