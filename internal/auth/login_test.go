package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildCLIAuthURLIncludesEncodedDevice(t *testing.T) {
	authURL := buildCLIAuthURL("https://dashboard.dwellir.com", 9999, "elias macbook/pro")

	if !strings.Contains(authURL, "port=9999") {
		t.Fatalf("expected port query param, got: %s", authURL)
	}
	if !strings.Contains(authURL, "device=elias+macbook%2Fpro") {
		t.Fatalf("expected URL-encoded device query param, got: %s", authURL)
	}
}

func TestBuildCLIAuthURLOmitsDeviceWhenEmpty(t *testing.T) {
	authURL := buildCLIAuthURL("https://dashboard.dwellir.com", 9999, "")

	if strings.Contains(authURL, "device=") {
		t.Fatalf("expected no device query param for empty hostname, got: %s", authURL)
	}
}

func TestLoginMuxHandlesCallbackPreflight(t *testing.T) {
	resultCh := make(chan *CallbackPayload, 1)
	errCh := make(chan error, 1)
	mux := newLoginMux("https://dashboard.dwellir.com", resultCh, errCh)

	req := httptest.NewRequest(http.MethodOptions, "/callback", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://dashboard.dwellir.com" {
		t.Fatalf("unexpected Access-Control-Allow-Origin: %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Fatalf("expected POST in Access-Control-Allow-Methods, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Content-Type") {
		t.Fatalf("expected Content-Type in Access-Control-Allow-Headers, got %q", got)
	}
}
