package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/dwellir-public/cli/internal/config"
)

type profileContext struct {
	Name   string
	Source string
}

func resolveProfileContext(profileFlag string, cwd string, configDir string) profileContext {
	if name := strings.TrimSpace(profileFlag); name != "" {
		return profileContext{Name: name, Source: "flag"}
	}

	if name := strings.TrimSpace(os.Getenv("DWELLIR_PROFILE")); name != "" {
		return profileContext{Name: name, Source: "env"}
	}

	if name := profileFromDwellirJSON(cwd); name != "" {
		return profileContext{Name: name, Source: "dwellir_json"}
	}

	configPath := filepath.Join(configDir, "config.json")
	if _, err := os.Stat(configPath); err == nil {
		cfg, loadErr := config.Load(configDir)
		if loadErr == nil {
			if name := strings.TrimSpace(cfg.DefaultProfile); name != "" {
				return profileContext{Name: name, Source: "config_default"}
			}
		}
	}

	return profileContext{Name: "default", Source: "fallback_default"}
}

func profileFromDwellirJSON(cwd string) string {
	if path := nearestDwellirJSONPath(cwd); path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			var parsed struct {
				Profile string `json:"profile"`
			}
			if json.Unmarshal(data, &parsed) == nil {
				return strings.TrimSpace(parsed.Profile)
			}
		}
	}
	return ""
}

func nearestDwellirJSONPath(cwd string) string {
	dir := strings.TrimSpace(cwd)
	if dir == "" {
		return ""
	}

	for {
		path := filepath.Join(dir, ".dwellir.json")
		if _, err := os.Stat(path); err == nil {
			return path
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
