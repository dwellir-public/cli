package cli

import (
	"reflect"
	"testing"

	"github.com/dwellir-public/cli/internal/api"
)

func TestDeleteSuccessPayloadHidesInternalEndpoints(t *testing.T) {
	payload := deleteSuccessPayload("abc-123", &api.DeleteKeyResult{
		CleanupPending: true,
	})

	want := map[string]interface{}{
		"key":    "abc-123",
		"status": "cleanup_pending",
	}

	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestDeleteSuccessPayloadDeletedStatus(t *testing.T) {
	payload := deleteSuccessPayload("abc-123", &api.DeleteKeyResult{})

	want := map[string]interface{}{
		"key":    "abc-123",
		"status": "deleted",
	}

	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestDeleteSuccessPayloadNilResultDefaultsToDeleted(t *testing.T) {
	payload := deleteSuccessPayload("abc-123", nil)

	want := map[string]interface{}{
		"key":    "abc-123",
		"status": "deleted",
	}

	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}
