package cli

import (
	"errors"
	"testing"
)

func TestClassifyExecutionErrorUnknownGet(t *testing.T) {
	code, message, help := classifyExecutionError(errors.New(`unknown command "get" for "dwellir"`))
	if code != "validation_error" {
		t.Fatalf("expected validation_error, got %q", code)
	}
	if message == "" || help == "" {
		t.Fatalf("expected message/help to be populated")
	}
}

func TestClassifyExecutionErrorMissingArgs(t *testing.T) {
	code, message, help := classifyExecutionError(errors.New("accepts 1 arg(s), received 0"))
	if code != "validation_error" {
		t.Fatalf("expected validation_error, got %q", code)
	}
	if message != "Missing required arguments." {
		t.Fatalf("unexpected message: %q", message)
	}
	if help == "" {
		t.Fatalf("expected non-empty help")
	}
}
