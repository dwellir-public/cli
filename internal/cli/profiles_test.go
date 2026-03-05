package cli

import (
	"testing"

	"github.com/dwellir-public/cli/internal/config"
)

func TestBuildProfilesListMarksActiveProfile(t *testing.T) {
	configDir := t.TempDir()
	if err := config.SaveProfile(configDir, &config.Profile{Name: "default", Token: "a"}); err != nil {
		t.Fatalf("save default profile: %v", err)
	}
	if err := config.SaveProfile(configDir, &config.Profile{Name: "work", Token: "b"}); err != nil {
		t.Fatalf("save work profile: %v", err)
	}

	items, err := buildProfilesList(configDir, profileContext{Name: "work", Source: "config_default"})
	if err != nil {
		t.Fatalf("buildProfilesList returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(items))
	}

	for _, item := range items {
		name, _ := item["name"].(string)
		if name == "work" {
			if item["active"] != true {
				t.Fatalf("expected work to be active: %#v", item)
			}
			if item["active_source"] != "config_default" {
				t.Fatalf("expected active_source=config_default: %#v", item)
			}
		}
	}
}
