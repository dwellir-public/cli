package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const testKey = "9f1c2d3e-4b5a-6789-abcd-ef0123456789"

func TestSecretMasksEveryOutputPath(t *testing.T) {
	key := APIKey{APIKey: NewSecret(testKey), Name: "ci"}
	encoded, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outputs := map[string]string{
		"%v":   fmt.Sprintf("%v", key),
		"%+v":  fmt.Sprintf("%+v", key),
		"%#v":  fmt.Sprintf("%#v", key),
		"%s":   fmt.Sprintf("%s", key.APIKey),
		"%q":   fmt.Sprintf("%q", key.APIKey),
		"%x":   fmt.Sprintf("%x", key.APIKey),
		"json": string(encoded),
	}
	for name, got := range outputs {
		if strings.Contains(got, testKey) || !strings.Contains(got, "9f1c****6789") {
			t.Errorf("%s: expected only the masked key, got %s", name, got)
		}
	}
}

func TestSecretMasking(t *testing.T) {
	cases := map[string]string{
		"":             "",
		"short":        "****",
		"0123456789ab": "****",
		testKey:        "9f1c****6789",
	}
	for in, want := range cases {
		if got := NewSecret(in).Masked(); got != want {
			t.Errorf("Masked(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSecretDecodesTheFullValue(t *testing.T) {
	var row UsageHistory
	if err := json.Unmarshal([]byte(`{"api_key":"`+testKey+`"}`), &row); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.APIKey.Reveal() != testKey {
		t.Fatalf("expected the full key after decoding, got %q", row.APIKey.Reveal())
	}

	if err := json.Unmarshal([]byte(`{"api_key":null}`), &row); err != nil || !row.APIKey.IsZero() {
		t.Fatalf("expected null to decode to an empty key, got %q, %v", row.APIKey.Reveal(), err)
	}
	encoded, _ := json.Marshal(row)
	if strings.Contains(string(encoded), "api_key") {
		t.Fatalf("expected an empty usage key to be omitted, got %s", encoded)
	}
}
