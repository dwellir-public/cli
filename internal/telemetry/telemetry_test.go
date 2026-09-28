package telemetry

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/posthog/posthog-go"
)

type fakePostHogClient struct {
	messages []posthog.Message
}

func (f *fakePostHogClient) Enqueue(msg posthog.Message) error {
	f.messages = append(f.messages, msg)
	return nil
}

func (f *fakePostHogClient) Close() error {
	return nil
}

func TestTrackCommand(t *testing.T) {
	Init("test-key", "test-user", "test-org", "Test Org", "test-device", false)
	defer Close()

	TrackCommand("keys.list", map[string]interface{}{
		"format":    "json",
		"exit_code": 0,
	})
}

func TestAnonymousMode(t *testing.T) {
	Init("test-key", "test-user", "test-org", "Test Org", "test-device", true)
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

func TestIdentifySendsOrganizationGroupIdentify(t *testing.T) {
	fake := &fakePostHogClient{}
	client = fake
	userID = "user-123"
	orgID = "acct-123"
	orgName = "Acme"
	deviceID = "device-123"
	anonymous = false
	version = "test"
	t.Cleanup(func() {
		client = nil
		userID = ""
		orgID = ""
		orgName = ""
		deviceID = ""
		anonymous = false
		version = ""
	})

	Identify(nil)

	if len(fake.messages) != 2 {
		t.Fatalf("expected identify and group identify messages, got %d", len(fake.messages))
	}
	group, ok := fake.messages[1].(posthog.GroupIdentify)
	if !ok {
		t.Fatalf("second message = %T, want posthog.GroupIdentify", fake.messages[1])
	}
	if group.Type != "organization" {
		t.Fatalf("group type = %q, want organization", group.Type)
	}
	if group.Key != "acct-123" {
		t.Fatalf("group key = %q, want acct-123", group.Key)
	}
	if group.Properties["name"] != "Acme" {
		t.Fatalf("group name property = %#v, want Acme", group.Properties["name"])
	}
}

func TestTrackCommandAssociatesOrganizationGroup(t *testing.T) {
	fake := &fakePostHogClient{}
	client = fake
	userID = "user-123"
	orgID = "acct-123"
	orgName = "Acme"
	deviceID = "device-123"
	anonymous = false
	version = "test"
	t.Cleanup(func() {
		client = nil
		userID = ""
		orgID = ""
		orgName = ""
		deviceID = ""
		anonymous = false
		version = ""
	})

	TrackCommand("version", nil)

	if len(fake.messages) != 1 {
		t.Fatalf("expected one capture message, got %d", len(fake.messages))
	}
	capture, ok := fake.messages[0].(posthog.Capture)
	if !ok {
		t.Fatalf("message = %T, want posthog.Capture", fake.messages[0])
	}
	if capture.Groups["organization"] != "acct-123" {
		t.Fatalf("organization group = %#v, want acct-123", capture.Groups["organization"])
	}
	if capture.Properties["org_id"] != "acct-123" {
		t.Fatalf("org_id property = %#v, want acct-123", capture.Properties["org_id"])
	}
	if capture.Properties["org_name"] != "Acme" {
		t.Fatalf("org_name property = %#v, want Acme", capture.Properties["org_name"])
	}
}

func TestIngestionOnlyForProductionAPI(t *testing.T) {
	for _, tc := range []struct {
		name, api string
		sends     bool
	}{
		{"default production", "", true},
		{"production dashboard", "https://dashboard.dwellir.com/marly-api/", true},
		{"production backend", "https://marly.dwellir.com", true},
		{"staging", "https://marly-staging.dwellir.com", false},
		{"local", "http://localhost:8000", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				_, _ = io.WriteString(w, `{"status":1}`)
			}))
			t.Cleanup(server.Close)
			t.Setenv("DWELLIR_API_URL", tc.api)
			t.Setenv("DWELLIR_POSTHOG_KEY", "test-key")
			t.Setenv("DWELLIR_POSTHOG_HOST", server.URL)
			Init("test", "user-123", "42", "Acme", "device", false)
			TrackCommand("account.info", nil)
			Close()
			if got := requests.Load() > 0; got != tc.sends {
				t.Fatalf("ingestion occurred=%v, want %v", got, tc.sends)
			}
		})
	}
}
