package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDevicePollingPendingThenSuccess(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"authorization_pending"}`)
			return
		}
		fmt.Fprint(w, `{"access_token":"test-private-token"}`)
	}))
	defer server.Close()
	token, err := pollDevice(context.Background(), server.Client(), server.URL, "device", time.Millisecond)
	if err != nil || token != "test-private-token" || calls != 2 {
		t.Fatalf("poll failed: %v, calls=%d", err, calls)
	}
}

func TestDevicePollingDenialAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"access_denied"}`)
	}))
	defer server.Close()
	if _, err := pollDevice(context.Background(), server.Client(), server.URL, "device", time.Millisecond); err == nil {
		t.Fatal("accepted denied grant")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := pollDevice(ctx, server.Client(), server.URL, "device", time.Millisecond); err == nil {
		t.Fatal("accepted canceled context")
	}
}
