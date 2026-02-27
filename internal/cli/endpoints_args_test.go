package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestEndpointOptionalKeySelectorFromArgs(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("key", "", "test")
	if keyFlag := cmd.Flags().Lookup("key"); keyFlag != nil {
		keyFlag.NoOptDefVal = endpointAutoKeySentinel
	}

	epKeyName = endpointAutoKeySentinel

	if err := cmd.Flags().Set("key", endpointAutoKeySentinel); err != nil {
		t.Fatalf("failed to set key flag: %v", err)
	}

	selector, err := endpointOptionalKeySelectorFromArgs(cmd, []string{"my-key"})
	if err != nil {
		t.Fatalf("expected selector, got error: %v", err)
	}
	if selector != "my-key" {
		t.Fatalf("expected my-key, got %q", selector)
	}
}
