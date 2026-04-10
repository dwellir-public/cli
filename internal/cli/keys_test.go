package cli

import (
	"reflect"
	"testing"

	"github.com/dwellir-public/cli/internal/api"
)

func TestDeleteSuccessPayloadHidesInternalEndpoints(t *testing.T) {
	payload := deleteSuccessPayload("abc-123", &api.DeleteKeyResult{
		CleanupPending:       true,
		UnreachableEndpoints: []string{"http://dead-haproxy:5555"},
	})

	want := map[string]interface{}{
		"key":    "abc-123",
		"status": "cleanup_pending",
	}

	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}
