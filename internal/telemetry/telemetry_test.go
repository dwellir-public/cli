package telemetry

import "testing"

func TestTrackCommand(t *testing.T) {
	Init("test-key", "test-user", "test-org", "test-device", false)
	defer Close()

	TrackCommand("keys.list", map[string]interface{}{
		"format":    "json",
		"exit_code": 0,
	})
}

func TestAnonymousMode(t *testing.T) {
	Init("test-key", "test-user", "test-org", "test-device", true)
	defer Close()

	TrackCommand("endpoints.search", nil)
}

func TestResolveEndpoint_DefaultsToEmbeddedEndpoint(t *testing.T) {
	t.Setenv("DWELLIR_POSTHOG_HOST", "")
	t.Setenv("DWELLIR_POSTHOG_ENDPOINT", "")
	old := posthogEndpoint
	posthogEndpoint = "https://eu.i.posthog.com"
	t.Cleanup(func() { posthogEndpoint = old })

	if got := resolveEndpoint(); got != "https://eu.i.posthog.com" {
		t.Fatalf("resolveEndpoint() = %q, want %q", got, "https://eu.i.posthog.com")
	}
}

func TestResolveEndpoint_HostEnvOverridesDefault(t *testing.T) {
	t.Setenv("DWELLIR_POSTHOG_HOST", "https://override.example.com")
	t.Setenv("DWELLIR_POSTHOG_ENDPOINT", "")

	if got := resolveEndpoint(); got != "https://override.example.com" {
		t.Fatalf("resolveEndpoint() = %q, want %q", got, "https://override.example.com")
	}
}

func TestResolveEndpoint_LegacyEndpointEnvUsedWhenHostUnset(t *testing.T) {
	t.Setenv("DWELLIR_POSTHOG_HOST", "")
	t.Setenv("DWELLIR_POSTHOG_ENDPOINT", "https://legacy.example.com")

	if got := resolveEndpoint(); got != "https://legacy.example.com" {
		t.Fatalf("resolveEndpoint() = %q, want %q", got, "https://legacy.example.com")
	}
}
