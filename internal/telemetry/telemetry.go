package telemetry

import (
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/posthog/posthog-go"
)

var (
	client    posthog.Client
	userID    string
	orgID     string
	deviceID  string
	anonymous bool
	version   string
)

var posthogAPIKey = ""

func Init(ver string, user string, org string, device string, anon bool) {
	version = ver
	userID = user
	orgID = org
	deviceID = device
	anonymous = anon

	apiKey := posthogAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("DWELLIR_POSTHOG_KEY")
	}
	if apiKey == "" {
		return
	}

	endpoint := strings.TrimSpace(os.Getenv("DWELLIR_POSTHOG_HOST"))
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("DWELLIR_POSTHOG_ENDPOINT"))
	}

	var err error
	cfg := posthog.Config{
		BatchSize: 10,
		Interval:  3 * time.Second,
	}
	if endpoint != "" {
		cfg.Endpoint = endpoint
	}
	client, err = posthog.NewWithConfig(apiKey, cfg)
	if err != nil {
		return
	}
}

func distinctID() string {
	if anonymous || userID == "" {
		if deviceID != "" {
			return "anon:" + deviceID
		}
		if id := os.Getenv("DWELLIR_DEVICE_ID"); id != "" {
			return "anon:" + strings.TrimSpace(id)
		}
		return "anon:unknown"
	}
	return userID
}

func baseProperties() posthog.Properties {
	props := posthog.NewProperties().
		Set("os", runtime.GOOS).
		Set("arch", runtime.GOARCH).
		Set("version", version)
	if !anonymous && orgID != "" {
		props.Set("org_id", orgID)
	}
	return props
}

func Identify(extra map[string]interface{}) {
	if client == nil {
		return
	}
	props := baseProperties().
		Set("distinct_id", distinctID()).
		Set("is_anonymous", anonymous || userID == "")
	if !anonymous && userID != "" {
		props.Set("user_id", userID)
	}
	for k, v := range extra {
		props.Set(k, v)
	}
	_ = client.Enqueue(posthog.Identify{
		DistinctId: distinctID(),
		Properties: props,
	})
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
