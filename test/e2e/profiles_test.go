//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfilesCurrentShowsConfigDefaultSource(t *testing.T) {
	configDir := t.TempDir()

	setDefault := runCLIWithConfigDir(t, configDir, "config", "set", "default_profile", "bench")
	if setDefault.exitCode != 0 {
		t.Fatalf("config set default_profile failed: %s", setDefault.stderr)
	}

	current := runCLIWithConfigDir(t, configDir, "profiles", "current", "--json")
	if current.exitCode != 0 {
		t.Fatalf("profiles current failed: %s", current.stderr)
	}

	parsed := parseJSON(t, current.stdout)
	data, ok := parsed["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing data object: %#v", parsed)
	}
	if data["profile"] != "bench" {
		t.Fatalf("profile = %#v, want %q", data["profile"], "bench")
	}
	if data["source"] != "config_default" {
		t.Fatalf("source = %#v, want %q", data["source"], "config_default")
	}
}

func TestProfilesBindAndUnbindAffectCurrentDirectory(t *testing.T) {
	configDir := t.TempDir()
	projectDir := t.TempDir()

	bind := runCLIWithConfigDirAndEnvAndDir(t, configDir, nil, projectDir, "profiles", "bind", "work", "--json")
	if bind.exitCode != 0 {
		t.Fatalf("profiles bind failed: %s", bind.stderr)
	}

	bindingPath := filepath.Join(projectDir, ".dwellir.json")
	if _, err := os.Stat(bindingPath); err != nil {
		t.Fatalf("expected binding file %s: %v", bindingPath, err)
	}

	current := runCLIWithConfigDirAndEnvAndDir(t, configDir, nil, projectDir, "profiles", "current", "--json")
	if current.exitCode != 0 {
		t.Fatalf("profiles current failed: %s", current.stderr)
	}
	parsed := parseJSON(t, current.stdout)
	data, ok := parsed["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing data object: %#v", parsed)
	}
	if data["profile"] != "work" {
		t.Fatalf("profile = %#v, want %q", data["profile"], "work")
	}
	if data["source"] != "dwellir_json" {
		t.Fatalf("source = %#v, want %q", data["source"], "dwellir_json")
	}

	unbind := runCLIWithConfigDirAndEnvAndDir(t, configDir, nil, projectDir, "profiles", "unbind", "--json")
	if unbind.exitCode != 0 {
		t.Fatalf("profiles unbind failed: %s", unbind.stderr)
	}
	if _, err := os.Stat(bindingPath); !os.IsNotExist(err) {
		t.Fatalf("expected binding file removal, stat err=%v", err)
	}
}

func TestProfilesListShowsActiveProfileAndTokenPresence(t *testing.T) {
	configDir := t.TempDir()

	loginDefault := runCLIWithConfigDir(t, configDir, "auth", "login", "--token", "default-token")
	if loginDefault.exitCode != 0 {
		t.Fatalf("auth login default failed: %s", loginDefault.stderr)
	}
	loginWork := runCLIWithConfigDir(t, configDir, "auth", "login", "--profile", "work", "--token", "work-token")
	if loginWork.exitCode != 0 {
		t.Fatalf("auth login work failed: %s", loginWork.stderr)
	}
	setDefault := runCLIWithConfigDir(t, configDir, "config", "set", "default_profile", "work")
	if setDefault.exitCode != 0 {
		t.Fatalf("config set default_profile failed: %s", setDefault.stderr)
	}

	list := runCLIWithConfigDir(t, configDir, "profiles", "list", "--json")
	if list.exitCode != 0 {
		t.Fatalf("profiles list failed: %s", list.stderr)
	}

	parsed := parseJSON(t, list.stdout)
	items, ok := parsed["data"].([]interface{})
	if !ok {
		t.Fatalf("data is not a list: %#v", parsed["data"])
	}

	seenWork := false
	for _, item := range items {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if entry["name"] == "work" {
			seenWork = true
			if entry["active"] != true {
				t.Fatalf("work profile should be active: %#v", entry)
			}
			if entry["token_present"] != true {
				t.Fatalf("work profile should report token_present=true: %#v", entry)
			}
			if entry["active_source"] != "config_default" {
				t.Fatalf("work active_source = %#v, want %q", entry["active_source"], "config_default")
			}
		}
	}
	if !seenWork {
		t.Fatalf("expected work profile in list: %#v", items)
	}
}
