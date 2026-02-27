//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

func TestKeysCreateRequiresName(t *testing.T) {
	result := runCLI(t, "keys", "create", "--json")
	if result.exitCode == 0 {
		t.Fatalf("expected non-zero exit when --name is missing")
	}
	if !strings.Contains(result.stdout, "\"code\":\"validation_error\"") {
		t.Fatalf("expected validation_error JSON output, got: %s", result.stdout)
	}
	if !strings.Contains(result.stdout, "Missing required flag --name.") {
		t.Fatalf("expected missing --name guidance, got: %s", result.stdout)
	}
}
