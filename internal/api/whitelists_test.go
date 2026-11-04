package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// marlyWhitelist matches pymarly.api_models.IPWhitelist.
const marlyWhitelist = `{
	"id": 7,
	"name": "office",
	"description": null,
	"enabled": true,
	"apply_to_all_api_keys": false,
	"entries": [{"id": 1, "cidr": "203.0.113.10/32", "description": null, "enabled": true}],
	"assigned_api_keys": [{"api_key_id": 11, "api_key": "` + testKey + `", "assignment_kind": "manual", "enabled": true}]
}`

func TestNormalizeWhitelistAddress(t *testing.T) {
	valid := map[string]string{
		"203.0.113.10":         "203.0.113.10/32",
		" 203.0.113.10/32 ":    "203.0.113.10/32",
		"2001:db8::5":          "2001:db8::5/128",
		"2001:DB8:0:0::5/128":  "2001:db8::5/128",
		"::ffff:203.0.113.10":  "::ffff:203.0.113.10/128",
		"2001:db8:0:0:0:0:0:1": "2001:db8::1/128",
	}
	for input, want := range valid {
		if got, err := NormalizeWhitelistAddress(input); err != nil || got != want {
			t.Errorf("NormalizeWhitelistAddress(%q) = %q, %v; want %q", input, got, err, want)
		}
	}

	invalid := []string{
		"", "office", "203.0.113.0/24", "2001:db8::/64", "2001:db8::5/32", "203.0.113.10/128",
		"203.0.113.10/", "203.0.113.10/abc", "010.0.0.1", "fe80::1%eth0", "203.0.113.300",
	}
	for _, input := range invalid {
		if got, err := NormalizeWhitelistAddress(input); err == nil {
			t.Errorf("NormalizeWhitelistAddress(%q) = %q; want an error", input, got)
		}
	}
}

func TestWhitelistsAPIRequests(t *testing.T) {
	type request struct{ method, path, body string }
	var got []request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = append(got, request{r.Method, r.URL.Path, strings.TrimSpace(string(body))})
		switch {
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte("[" + marlyWhitelist + "]"))
		case r.Method == http.MethodDelete && r.URL.Path == "/v4/organization/ip-whitelists/7":
			w.WriteHeader(http.StatusNoContent)
		default:
			_, _ = w.Write([]byte(marlyWhitelist))
		}
	}))
	defer server.Close()

	whitelists := NewWhitelistsAPI(NewClient(server.URL, "token"))
	key := NewSecret(testKey)
	description := "Corp NAT"
	name := "hq"
	if _, err := whitelists.List(); err != nil {
		t.Fatal(err)
	}
	if _, err := whitelists.Create(CreateWhitelistInput{Name: "office", Entries: []string{"203.0.113.10/32"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := whitelists.Create(CreateWhitelistInput{
		Name: "office", Description: &description, Entries: []string{"203.0.113.10/32"}, ApplyToAllAPIKeys: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := whitelists.SetEnabled(7, false); err != nil {
		t.Fatal(err)
	}
	if _, err := whitelists.Update(7, UpdateWhitelistInput{Name: &name, Entries: []string{"2001:db8::5/128"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := whitelists.Assign(7, key); err != nil {
		t.Fatal(err)
	}
	if _, err := whitelists.Unassign(7, key); err != nil {
		t.Fatal(err)
	}
	if err := whitelists.Delete(7); err != nil {
		t.Fatal(err)
	}

	want := []request{
		{"GET", "/v4/organization/ip-whitelists", ""},
		// Marly defaults enabled to true, so a disabled create must send false.
		{"POST", "/v4/organization/ip-whitelists", `{"name":"office","entries":["203.0.113.10/32"],"apply_to_all_api_keys":false,"enabled":false}`},
		{"POST", "/v4/organization/ip-whitelists", `{"name":"office","description":"Corp NAT","entries":["203.0.113.10/32"],"apply_to_all_api_keys":true,"enabled":true}`},
		// Marly rejects null fields on PATCH, so unset fields are left out.
		{"PATCH", "/v4/organization/ip-whitelists/7", `{"enabled":false}`},
		{"PATCH", "/v4/organization/ip-whitelists/7", `{"name":"hq","entries":["2001:db8::5/128"]}`},
		{"POST", "/v4/organization/ip-whitelists/7/api-keys/" + testKey, ""},
		{"DELETE", "/v4/organization/ip-whitelists/7/api-keys/" + testKey, ""},
		{"DELETE", "/v4/organization/ip-whitelists/7", ""},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d requests, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("request %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestWhitelistDecodesMarlyShapeAndMasksKeys(t *testing.T) {
	var whitelist Whitelist
	if err := json.Unmarshal([]byte(marlyWhitelist), &whitelist); err != nil {
		t.Fatal(err)
	}
	if whitelist.ID != 7 || whitelist.Name != "office" || !whitelist.Enabled || whitelist.Description != nil {
		t.Fatalf("unexpected whitelist %#v", whitelist)
	}
	if len(whitelist.Entries) != 1 || whitelist.Entries[0].CIDR != "203.0.113.10/32" {
		t.Fatalf("unexpected entries %#v", whitelist.Entries)
	}
	assignment := whitelist.AssignedAPIKeys[0]
	if assignment.APIKey.Reveal() != testKey || assignment.AssignmentKind != AssignmentManual {
		t.Fatalf("unexpected assignment %#v", assignment)
	}
	encoded, err := json.Marshal(whitelist)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), testKey) || !strings.Contains(string(encoded), `"api_key":"9f1c****6789"`) {
		t.Fatalf("expected a masked key, got %s", encoded)
	}
}

func TestFindWhitelist(t *testing.T) {
	whitelists := []Whitelist{{ID: 7, Name: "Office"}, {ID: 8, Name: "ci"}, {ID: 9, Name: "ci"}, {ID: 10, Name: "7"}}
	cases := map[string]struct {
		selector  string
		wantID    int64
		ambiguous bool
		missing   bool
	}{
		"by id":                    {selector: "8", wantID: 8},
		"by name ignoring case":    {selector: " office ", wantID: 7},
		"duplicate name":           {selector: "ci", ambiguous: true},
		"name equal to another id": {selector: "7", ambiguous: true},
		"no match":                 {selector: "home", missing: true},
		"empty":                    {selector: "", missing: true},
	}
	for name, tc := range cases {
		got, err := FindWhitelist(whitelists, tc.selector)
		var lookupErr *WhitelistLookupError
		switch {
		case tc.ambiguous || tc.missing:
			if !errors.As(err, &lookupErr) || lookupErr.Ambiguous != tc.ambiguous {
				t.Errorf("%s: got %#v, %v", name, got, err)
			}
		case err != nil || got.ID != tc.wantID:
			t.Errorf("%s: got %#v, %v; want ID %d", name, got, err, tc.wantID)
		}
	}
}
