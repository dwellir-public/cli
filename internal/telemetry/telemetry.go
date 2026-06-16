package telemetry

import (
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/posthog/posthog-go"
)

var (
	client    posthogClient
	userID    string
	orgID     string
	orgName   string
	deviceID  string
	anonymous bool
	version   string
)

type posthogClient interface {
	posthog.EnqueueClient
	Close() error
}

var posthogAPIKey = ""
var posthogEndpoint = "https://eu.i.posthog.com"

func Enabled() bool {
	return posthogAPIKey != "" || strings.TrimSpace(os.Getenv("DWELLIR_POSTHOG_KEY")) != ""
}

func Init(ver string, user string, org string, organizationName string, device string, anon bool) {
	version = ver
	userID = user
	orgID = org
	orgName = organizationName
	deviceID = device
	anonymous = anon

	apiKey := posthogAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("DWELLIR_POSTHOG_KEY")
	}
	if apiKey == "" {
		return
	}

	endpoint := resolveEndpoint()

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

func resolveEndpoint() string {
	if endpoint := strings.TrimSpace(os.Getenv("DWELLIR_POSTHOG_HOST")); endpoint != "" {
		return endpoint
	}
	if endpoint := strings.TrimSpace(os.Getenv("DWELLIR_POSTHOG_ENDPOINT")); endpoint != "" {
		return endpoint
	}
	return strings.TrimSpace(posthogEndpoint)
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
	if !anonymous && orgName != "" {
		props.Set("org_name", orgName)
	}
	return props
}

func groups() posthog.Groups {
	if anonymous || orgID == "" {
		return nil
	}
	return posthog.NewGroups().Set("organization", orgID)
}

func groupProperties() posthog.Properties {
	props := posthog.NewProperties()
	if orgName != "" {
		props.Set("name", orgName)
	}
	props.Set("version", version)
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
	if !anonymous && orgID != "" {
		_ = client.Enqueue(posthog.GroupIdentify{
			Type:       "organization",
			Key:        orgID,
			DistinctId: distinctID(),
			Properties: groupProperties(),
		})
	}
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
		Groups:     groups(),
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
		Groups:     groups(),
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
		Groups: groups(),
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
		Groups: groups(),
	})
}

func Close() {
	if client != nil {
		_ = client.Close()
	}
}
