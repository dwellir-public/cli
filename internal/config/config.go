package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Config struct {
	Output         string `json:"output"`
	DefaultProfile string `json:"default_profile"`
	configDir      string
}

var validKeys = map[string]bool{
	"output":          true,
	"default_profile": true,
}

func Load(configDir string) (*Config, error) {
	cfg := &Config{
		Output:         "human",
		DefaultProfile: "default",
		configDir:      configDir,
	}

	path := filepath.Join(configDir, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	cfg.configDir = configDir
	return cfg, nil
}

func (c *Config) Set(key, value string) error {
	if !validKeys[key] {
		return fmt.Errorf("unknown config key: %s (valid keys: output, default_profile)", key)
	}
	switch key {
	case "output":
		if value != "json" && value != "human" {
			return fmt.Errorf("output must be 'json' or 'human'")
		}
		c.Output = value
	case "default_profile":
		c.DefaultProfile = value
	}
	return c.Save()
}

func (c *Config) Get(key string) string {
	switch key {
	case "output":
		return c.Output
	case "default_profile":
		return c.DefaultProfile
	default:
		return ""
	}
}

func (c *Config) All() map[string]string {
	return map[string]string{
		"output":          c.Output,
		"default_profile": c.DefaultProfile,
	}
}

func (c *Config) Save() error {
	if err := os.MkdirAll(c.configDir, 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	path := filepath.Join(c.configDir, "config.json")
	return os.WriteFile(path, data, 0o600)
}

// DefaultConfigDir returns the XDG-compliant config directory.
func DefaultConfigDir() string {
	if dir := os.Getenv("DWELLIR_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dwellir")
}

func Stderr() io.Writer {
	return os.Stderr
}
