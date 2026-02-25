package telemetry

import (
	"os"
	"runtime"
	"time"

	"github.com/posthog/posthog-go"
)

var (
	client    posthog.Client
	userID    string
	orgID     string
	anonymous bool
	version   string
)

const posthogAPIKey = ""

func Init(ver string, user string, org string, anon bool) {
	version = ver
	userID = user
	orgID = org
	anonymous = anon

	apiKey := posthogAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("DWELLIR_POSTHOG_KEY")
	}
	if apiKey == "" {
		return
	}

	var err error
	client, err = posthog.NewWithConfig(apiKey, posthog.Config{
		BatchSize: 10,
		Interval:  30 * time.Second,
	})
	if err != nil {
		return
	}
}

func distinctID() string {
	if anonymous || userID == "" {
		if id := os.Getenv("DWELLIR_DEVICE_ID"); id != "" {
			return "anon:" + id
		}
		return "anon:unknown"
	}
	return userID
}

func baseProperties() posthog.Properties {
	return posthog.NewProperties().
		Set("os", runtime.GOOS).
		Set("arch", runtime.GOARCH).
		Set("version", version).
		Set("org_id", orgID)
}

func TrackCommand(command string, extra map[string]interface{}) {
	if client == nil {
		return
	}
	props := baseProperties().Set("command", command)
	for k, v := range extra {
		props.Set(k, v)
	}
	_ = client.Enqueue(posthog.Capture{
		DistinctId: distinctID(),
		Event:      "cli_command",
		Properties: props,
	})
}

func TrackInstall(method string) {
	if client == nil {
		return
	}
	_ = client.Enqueue(posthog.Capture{
		DistinctId: distinctID(),
		Event:      "cli_installed",
		Properties: baseProperties().Set("install_method", method),
	})
}

func TrackAuth(method string, success bool) {
	if client == nil {
		return
	}
	_ = client.Enqueue(posthog.Capture{
		DistinctId: distinctID(),
		Event:      "cli_auth",
		Properties: baseProperties().
			Set("method", method).
			Set("success", success),
	})
}

func TrackUpdate(fromVersion string, toVersion string) {
	if client == nil {
		return
	}
	_ = client.Enqueue(posthog.Capture{
		DistinctId: distinctID(),
		Event:      "cli_updated",
		Properties: baseProperties().
			Set("from_version", fromVersion).
			Set("to_version", toVersion),
	})
}

func Close() {
	if client != nil {
		_ = client.Close()
	}
}
