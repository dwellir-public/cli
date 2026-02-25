package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Profile struct {
	Name  string `json:"name"`
	Token string `json:"token"`
	Org   string `json:"org,omitempty"`
	User  string `json:"user,omitempty"`
}

type dwellirJSON struct {
	Profile string `json:"profile"`
}

func SaveProfile(configDir string, p *Profile) error {
	dir := filepath.Join(configDir, "profiles")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating profiles dir: %w", err)
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling profile: %w", err)
	}
	path := filepath.Join(dir, p.Name+".json")
	return os.WriteFile(path, data, 0o600)
}

func LoadProfile(configDir string, name string) (*Profile, error) {
	path := filepath.Join(configDir, "profiles", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading profile '%s': %w", name, err)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing profile '%s': %w", name, err)
	}
	return &p, nil
}

func ListProfiles(configDir string) ([]string, error) {
	dir := filepath.Join(configDir, "profiles")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name()[:len(e.Name())-5])
		}
	}
	return names, nil
}

// ResolveProfileName determines which profile to use based on priority:
// 1. Explicit flag (--profile)
// 2. DWELLIR_PROFILE env var
// 3. .dwellir.json in cwd or parent dirs
// 4. Config default_profile
// 5. "default"
func ResolveProfileName(flagValue string, envValue string, cwd string, configDir string) string {
	if flagValue != "" {
		return flagValue
	}
	if envValue != "" {
		return envValue
	}
	if name := findDwellirJSON(cwd); name != "" {
		return name
	}
	cfg, err := Load(configDir)
	if err == nil && cfg.DefaultProfile != "" {
		return cfg.DefaultProfile
	}
	return "default"
}

func findDwellirJSON(dir string) string {
	for {
		path := filepath.Join(dir, ".dwellir.json")
		data, err := os.ReadFile(path)
		if err == nil {
			var d dwellirJSON
			if json.Unmarshal(data, &d) == nil && d.Profile != "" {
				return d.Profile
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
