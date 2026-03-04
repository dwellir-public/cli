package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/config"
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Inspect and manage auth profiles",
}

var profilesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all local auth profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		cwd, _ := os.Getwd()
		ctx := resolveProfileContext(profile, cwd, configDir)

		items, err := buildProfilesList(configDir, ctx)
		if err != nil {
			return formatCommandError(err)
		}
		return getFormatter().Success("profiles.list", items)
	},
}

var profilesCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show currently active profile and selection source",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		cwd, _ := os.Getwd()
		ctx := resolveProfileContext(profile, cwd, configDir)

		profilePath := filepath.Join(configDir, "profiles", ctx.Name+".json")
		current := map[string]interface{}{
			"profile":        ctx.Name,
			"source":         ctx.Source,
			"cwd":            cwd,
			"config_dir":     configDir,
			"profile_path":   profilePath,
			"profile_exists": false,
			"token_present":  false,
		}
		if binding := nearestDwellirJSONPath(cwd); binding != "" {
			current["binding_path"] = binding
		}

		if p, err := config.LoadProfile(configDir, ctx.Name); err == nil && p != nil {
			current["profile_exists"] = true
			current["token_present"] = strings.TrimSpace(p.Token) != ""
			if strings.TrimSpace(p.User) != "" {
				current["user"] = strings.TrimSpace(p.User)
			}
			if strings.TrimSpace(p.Org) != "" {
				current["org"] = strings.TrimSpace(p.Org)
			}
		}

		return getFormatter().Success("profiles.current", current)
	},
}

var profilesBindCmd = &cobra.Command{
	Use:   "bind <name>",
	Short: "Bind current directory to a profile via .dwellir.json",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := os.Getwd()
		name := strings.TrimSpace(args[0])
		if name == "" {
			return getFormatter().Error("validation_error", "Profile name cannot be empty.", "Use: dwellir profiles bind <name>")
		}

		path := filepath.Join(cwd, ".dwellir.json")
		payload := map[string]string{"profile": name}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return formatCommandError(err)
		}
		data = append(data, '\n')
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return formatCommandError(err)
		}

		return getFormatter().Success("profiles.bind", map[string]interface{}{
			"status":       "bound",
			"profile":      name,
			"binding_path": path,
		})
	},
}

var profilesUnbindCmd = &cobra.Command{
	Use:   "unbind",
	Short: "Remove profile binding from current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := os.Getwd()
		path := filepath.Join(cwd, ".dwellir.json")
		err := os.Remove(path)
		removed := err == nil
		if err != nil && !os.IsNotExist(err) {
			return formatCommandError(err)
		}

		return getFormatter().Success("profiles.unbind", map[string]interface{}{
			"status":       "unbound",
			"binding_path": path,
			"removed":      removed,
		})
	},
}

func buildProfilesList(configDir string, ctx profileContext) ([]map[string]interface{}, error) {
	names, err := config.ListProfiles(configDir)
	if err != nil {
		return nil, err
	}
	sort.Strings(names)

	items := make([]map[string]interface{}, 0, len(names))
	for _, name := range names {
		p, loadErr := config.LoadProfile(configDir, name)
		if loadErr != nil {
			return nil, loadErr
		}

		item := map[string]interface{}{
			"name":          name,
			"path":          filepath.Join(configDir, "profiles", name+".json"),
			"token_present": strings.TrimSpace(p.Token) != "",
			"active":        name == ctx.Name,
		}
		if name == ctx.Name {
			item["active_source"] = ctx.Source
		}
		if strings.TrimSpace(p.User) != "" {
			item["user"] = strings.TrimSpace(p.User)
		}
		if strings.TrimSpace(p.Org) != "" {
			item["org"] = strings.TrimSpace(p.Org)
		}
		items = append(items, item)
	}

	return items, nil
}

func init() {
	profilesCmd.AddCommand(profilesListCmd, profilesCurrentCmd, profilesBindCmd, profilesUnbindCmd)
	rootCmd.AddCommand(profilesCmd)
}
