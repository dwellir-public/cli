package auth

import (
	"fmt"
	"os"

	"github.com/dwellir-public/cli/internal/config"
)

// ResolveToken finds the CLI token using the priority chain:
// 1. DWELLIR_TOKEN env var
// 2. --token flag value
// 3. Profile resolved from --profile flag / .dwellir.json / config default
func ResolveToken(tokenFlag string, profileFlag string, cwd string, configDir string) (string, error) {
	if envToken := os.Getenv("DWELLIR_TOKEN"); envToken != "" {
		return envToken, nil
	}

	if tokenFlag != "" {
		return tokenFlag, nil
	}

	envProfile := os.Getenv("DWELLIR_PROFILE")
	profileName := config.ResolveProfileName(profileFlag, envProfile, cwd, configDir)

	p, err := config.LoadProfile(configDir, profileName)
	if err != nil {
		return "", fmt.Errorf("not authenticated. Run 'dwellir auth login' or set DWELLIR_TOKEN.\n\nFor headless/CI, create a token at https://dashboard.dwellir.com/agents")
	}

	if p.Token == "" {
		return "", fmt.Errorf("profile '%s' has no token. Run 'dwellir auth login --profile %s'", profileName, profileName)
	}

	return p.Token, nil
}
