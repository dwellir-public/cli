package api

import (
	"context"
	"encoding/json"
	"errors"
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
		{APIKey: NewSecret("abc-123"), Name: "test-key", Enabled: true},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v4/organization/apikeys" {
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
		if r.URL.Path != "/v4/organization/apikeys" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var input CreateKeyInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(APIKey{APIKey: NewSecret("new-key"), Name: input.Name, Enabled: true}); err != nil {
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
		APIKey:       NewSecret("abc-123"),
		Name:         "old-name",
		Enabled:      true,
		DailyQuota:   nil,
		MonthlyQuota: nil,
	}

	var updateBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v4/organization/apikeys/abc-123":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read body: %v", err)
			}
			updateBody = string(body)
			if err := json.NewEncoder(w).Encode(APIKey{
				APIKey:       NewSecret("abc-123"),
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
	_, err := ka.Update(current, UpdateKeyInput{Name: &name})
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

func TestFindKey(t *testing.T) {
	keys := []APIKey{
		{APIKey: NewSecret("9f1c2d3e-4b5a-6789-abcd-ef0123456789"), Name: "CI"},
		{APIKey: NewSecret("1234abcd-0000-1111-2222-333344445555"), Name: "dup"},
		{APIKey: NewSecret("5678abcd-0000-1111-2222-333344446666"), Name: "dup"},
	}
	for _, selector := range []string{"ci", "9f1c****6789", "9f1c2d3e-4b5a-6789-abcd-ef0123456789"} {
		key, err := FindKey(keys, selector)
		if err != nil || key.Name != "CI" {
			t.Errorf("FindKey(%q) = %q, %v; want CI", selector, key.Name, err)
		}
	}

	var lookupErr *KeyLookupError
	if _, err := FindKey(keys, "missing"); !errors.As(err, &lookupErr) || lookupErr.Ambiguous {
		t.Errorf("expected not-found lookup error, got %v", err)
	}
	_, err := FindKey(keys, "dup")
	if !errors.As(err, &lookupErr) || !lookupErr.Ambiguous {
		t.Fatalf("expected ambiguous lookup error, got %v", err)
	}
	if got := strings.Join(lookupErr.Candidates, ", "); got != "dup (1234****5555), dup (5678****6666)" {
		t.Errorf("unexpected candidates %q", got)
	}
	if strings.Contains(err.Error(), "abcd-0000") {
		t.Errorf("lookup error leaks a key: %v", err)
	}
}

func TestUpdateRetriesTimeoutOnce(t *testing.T) {
	client := NewClient("https://example.com", "token")

	var postCount int32
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodPost && req.URL.Path == "/v4/organization/apikeys/abc-123":
				count := atomic.AddInt32(&postCount, 1)
				if count == 1 {
					return nil, timeoutErr{err: context.DeadlineExceeded}
				}
				body, _ := json.Marshal(APIKey{
					APIKey:  NewSecret("abc-123"),
					Name:    "renamed",
					Enabled: true,
				})
				return jsonResponse(req, http.StatusOK, body), nil
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, context.Canceled
			}
		}),
	}

	ka := NewKeysAPI(client)
	name := "renamed"
	key, err := ka.Update(APIKey{APIKey: NewSecret("abc-123"), Name: "key", Enabled: true}, UpdateKeyInput{Name: &name})
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

func TestDeleteKey(t *testing.T) {
	var gotMethod string
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ka := NewKeysAPI(client)

	result, err := ka.Delete(NewSecret("abc-123"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CleanupPending {
		t.Fatal("expected cleanup pending to be false")
	}

	if gotMethod != http.MethodDelete {
		t.Fatalf("expected DELETE, got %s", gotMethod)
	}
	if gotPath != "/v4/organization/apikeys/abc-123" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
}

func TestDeleteKeyReturnsCleanupPendingDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/v4/organization/apikeys/abc-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(deleteKeyResponse{
			CleanupPending:       true,
			UnreachableEndpoints: []string{"http://dead-haproxy:5555"},
		}); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ka := NewKeysAPI(client)

	result, err := ka.Delete(NewSecret("abc-123"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.CleanupPending {
		t.Fatal("expected cleanup pending to be true")
	}
}

func TestDeleteKeyTreatsEmptyAcceptedResponseAsCleanupPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ka := NewKeysAPI(client)

	result, err := ka.Delete(NewSecret("abc-123"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.CleanupPending {
		t.Fatal("expected cleanup pending to be true")
	}
}

func TestDeleteKeyErrorsNeverContainTheKey(t *testing.T) {
	key := NewSecret("9f1c2d3e-4b5a-6789-abcd-ef0123456789")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"no key at `+r.URL.Path+`"}`, http.StatusNotFound)
	}))
	ka := NewKeysAPI(NewClient(server.URL, "token"))

	result, err := ka.Delete(key)
	if err == nil || result != nil {
		t.Fatalf("expected an error and no result, got %#v, %v", result, err)
	}
	if strings.Contains(err.Error(), key.Reveal()) || !strings.Contains(err.Error(), key.Masked()) {
		t.Fatalf("expected the API error to show only the masked key, got %v", err)
	}

	// A transport error names the request URL, and the URL path holds the key.
	server.Close()
	if _, err := ka.Delete(key); err == nil || strings.Contains(err.Error(), key.Reveal()) {
		t.Fatalf("expected a transport error without the key, got %v", err)
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
