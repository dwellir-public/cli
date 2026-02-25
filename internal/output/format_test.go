package output

import (
	"bytes"
	"testing"
)

func TestJSONSuccess(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(&buf)
	err := f.Success("keys.list", map[string]string{"count": "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if got == "" {
		t.Fatal("expected non-empty output")
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"ok":true`)) {
		t.Errorf("expected ok:true in output, got: %s", got)
	}
}

func TestJSONError(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(&buf)
	err := f.Error("not_authenticated", "No token found.", "Run 'dwellir auth login'")
	if err == nil {
		t.Fatal("expected formatter to return non-nil error for error responses")
	}
	got := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte(`"ok":false`)) {
		t.Errorf("expected ok:false in output, got: %s", got)
	}
}

func TestHumanSuccess(t *testing.T) {
	var buf bytes.Buffer
	f := NewHumanFormatter(&buf)
	err := f.Success("keys.list", map[string]string{"count": "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if got == "" {
		t.Fatal("expected non-empty output")
	}
}
