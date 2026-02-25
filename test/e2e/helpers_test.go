//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	dir, _ := os.Getwd()
	root := filepath.Join(dir, "..", "..")
	binaryPath = filepath.Join(root, "bin", "dwellir-test")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/dwellir")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("failed to build: " + string(out))
	}

	code := m.Run()
	_ = os.Remove(binaryPath)
	os.Exit(code)
}

type cliResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()
	return runCLIWithConfigDir(t, t.TempDir(), args...)
}

func runCLIWithConfigDir(t *testing.T, configDir string, args ...string) cliResult {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(),
		"DWELLIR_CONFIG_DIR="+configDir,
		"HOME="+t.TempDir(),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		exitCode = 1
	}

	return cliResult{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: exitCode,
	}
}

func parseJSON(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, raw)
	}
	return result
}
