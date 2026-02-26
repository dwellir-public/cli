package api

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestListKeys(t *testing.T) {
	keys := []APIKey{
		{APIKey: "abc-123", Name: "test-key", Enabled: true},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/user/apikeys" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode(keys); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ka := NewKeysAPI(client)
	result, err := ka.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 key, got %d", len(result))
	}
	if result[0].Name != "test-key" {
		t.Errorf("expected test-key, got %s", result[0].Name)
	}
}

func TestCreateKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var input CreateKeyInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(APIKey{APIKey: "new-key", Name: input.Name, Enabled: true}); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ka := NewKeysAPI(client)
	result, err := ka.Create(CreateKeyInput{Name: "my-key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "my-key" {
		t.Errorf("expected my-key, got %s", result.Name)
	}
}

func TestUpdateSendsFullPayload(t *testing.T) {
	current := APIKey{
		APIKey:       "abc-123",
		Name:         "old-name",
		Enabled:      true,
		DailyQuota:   nil,
		MonthlyQuota: nil,
	}

	var updateBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v3/user/apikeys":
			if err := json.NewEncoder(w).Encode([]APIKey{current}); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/user/apikey/abc-123":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read body: %v", err)
			}
			updateBody = string(body)
			if err := json.NewEncoder(w).Encode(APIKey{
				APIKey:       "abc-123",
				Name:         "new-name",
				Enabled:      true,
				DailyQuota:   nil,
				MonthlyQuota: nil,
			}); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ka := NewKeysAPI(client)
	name := "new-name"
	_, err := ka.Update("abc-123", UpdateKeyInput{Name: &name})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(updateBody, `"name":"new-name"`) {
		t.Fatalf("expected request to include name, got %s", updateBody)
	}
	if !strings.Contains(updateBody, `"enabled":true`) {
		t.Fatalf("expected request to include enabled field, got %s", updateBody)
	}
	if !strings.Contains(updateBody, `"daily_quota":null`) {
		t.Fatalf("expected request to include daily_quota field, got %s", updateBody)
	}
	if !strings.Contains(updateBody, `"monthly_quota":null`) {
		t.Fatalf("expected request to include monthly_quota field, got %s", updateBody)
	}
}

func TestUpdateMissingKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v3/user/apikeys" {
			_ = json.NewEncoder(w).Encode([]APIKey{{APIKey: "another-key"}})
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ka := NewKeysAPI(client)
	_, err := ka.Update("missing-key", UpdateKeyInput{})
	if err == nil {
		t.Fatal("expected error when key is not found")
	}
}

func TestUpdateRetriesTimeoutOnce(t *testing.T) {
	client := NewClient("https://example.com", "token")

	var postCount int32
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.URL.Path == "/v3/user/apikeys":
				body, _ := json.Marshal([]APIKey{{
					APIKey:  "abc-123",
					Name:    "key",
					Enabled: true,
				}})
				return jsonResponse(req, http.StatusOK, body), nil
			case req.Method == http.MethodPost && req.URL.Path == "/user/apikey/abc-123":
				count := atomic.AddInt32(&postCount, 1)
				if count == 1 {
					return nil, timeoutErr{err: context.DeadlineExceeded}
				}
				body, _ := json.Marshal(APIKey{
					APIKey:  "abc-123",
					Name:    "renamed",
					Enabled: true,
				})
				return jsonResponse(req, http.StatusOK, body), nil
			default:
				return nil, nil
			}
		}),
	}

	ka := NewKeysAPI(client)
	name := "renamed"
	key, err := ka.Update("abc-123", UpdateKeyInput{Name: &name})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.Name != "renamed" {
		t.Fatalf("expected renamed key, got %q", key.Name)
	}
	if got := atomic.LoadInt32(&postCount); got != 2 {
		t.Fatalf("expected 2 POST attempts, got %d", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(req *http.Request, status int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Header:     make(http.Header),
		Request:    req,
	}
}

type timeoutErr struct {
	err error
}

func (e timeoutErr) Error() string {
	return e.err.Error()
}

func (e timeoutErr) Timeout() bool {
	return true
}

func (e timeoutErr) Temporary() bool {
	return true
}

var _ net.Error = timeoutErr{}
