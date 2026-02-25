package telemetry

import "testing"

func TestTrackCommand(t *testing.T) {
	Init("test-key", "test-user", "test-org", false)
	defer Close()

	TrackCommand("keys.list", map[string]interface{}{
		"format":    "json",
		"exit_code": 0,
	})
}

func TestAnonymousMode(t *testing.T) {
	Init("test-key", "test-user", "test-org", true)
	defer Close()

	TrackCommand("endpoints.search", nil)
}
