//go:build e2e

package e2e

import "testing"

func findDoctorCheck(t *testing.T, parsed map[string]interface{}, name string) map[string]interface{} {
	t.Helper()
	data, ok := parsed["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("doctor response missing data object: %#v", parsed)
	}
	checks, ok := data["checks"].([]interface{})
	if !ok {
		t.Fatalf("doctor response missing checks list: %#v", data)
	}
	for _, item := range checks {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if entry["name"] == name {
			return entry
		}
	}
	t.Fatalf("doctor check %q not found in %#v", name, checks)
	return nil
}

func TestDoctorReportsMissingTokenWhenProfileIsNotAuthenticated(t *testing.T) {
	configDir := t.TempDir()

	setDefault := runCLIWithConfigDir(t, configDir, "config", "set", "default_profile", "bench")
	if setDefault.exitCode != 0 {
		t.Fatalf("config set default_profile failed: %s", setDefault.stderr)
	}

	doctor := runCLIWithConfigDir(t, configDir, "doctor", "--json")
	if doctor.exitCode != 0 {
		t.Fatalf("doctor command failed: stderr=%s stdout=%s", doctor.stderr, doctor.stdout)
	}
	parsed := parseJSON(t, doctor.stdout)
	tokenCheck := findDoctorCheck(t, parsed, "token_presence")
	if tokenCheck["status"] != "error" {
		t.Fatalf("token_presence status = %#v, want %q", tokenCheck["status"], "error")
	}

	profileCheck := findDoctorCheck(t, parsed, "profile_resolution")
	details, ok := profileCheck["details"].(map[string]interface{})
	if !ok {
		t.Fatalf("profile_resolution details missing: %#v", profileCheck)
	}
	if details["profile"] != "bench" {
		t.Fatalf("resolved profile = %#v, want %q", details["profile"], "bench")
	}
	if details["source"] != "config_default" {
		t.Fatalf("resolved source = %#v, want %q", details["source"], "config_default")
	}
}

func TestDoctorShowsAutoTOONForAgentMarkersWithoutConfig(t *testing.T) {
	configDir := t.TempDir()
	doctor := runCLIWithConfigDirAndEnv(t, configDir, map[string]string{"CODEX_CI": "1"}, "doctor", "--json")
	if doctor.exitCode != 0 {
		t.Fatalf("doctor command failed: stderr=%s stdout=%s", doctor.stderr, doctor.stdout)
	}
	parsed := parseJSON(t, doctor.stdout)
	outputCheck := findDoctorCheck(t, parsed, "output_mode")
	details, ok := outputCheck["details"].(map[string]interface{})
	if !ok {
		t.Fatalf("output_mode details missing: %#v", outputCheck)
	}
	if details["auto_default_format"] != "toon" {
		t.Fatalf("auto_default_format = %#v, want %q", details["auto_default_format"], "toon")
	}
}
