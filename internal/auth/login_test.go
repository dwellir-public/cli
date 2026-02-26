package auth

import (
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
