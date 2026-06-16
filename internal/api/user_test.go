package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserCurrentFetchesCurrentUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v4/user" {
			t.Fatalf("path = %s, want /v4/user", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("Authorization = %q, want Bearer token", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "user-123",
			"name":  "Ada Lovelace",
			"email": "ada@example.com",
		})
	}))
	defer server.Close()

	user, err := NewUserAPI(NewClient(server.URL, "token")).Current()
	if err != nil {
		t.Fatalf("Current() unexpected error: %v", err)
	}
	if user.ID != "user-123" {
		t.Fatalf("ID = %q, want user-123", user.ID)
	}
	if user.Email != "ada@example.com" {
		t.Fatalf("Email = %q, want ada@example.com", user.Email)
	}
	if user.Name != "Ada Lovelace" {
		t.Fatalf("Name = %q, want Ada Lovelace", user.Name)
	}
}
