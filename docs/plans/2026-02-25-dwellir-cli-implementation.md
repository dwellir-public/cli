# Dwellir CLI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go CLI (`dwellir`) that provides full programmatic access to the Dwellir dashboard — endpoint discovery, API key management, usage analytics, error logs, and account info — optimized for coding agents.

**Architecture:** Standalone Go binary using Cobra/Viper. Talks to the Marly API over HTTPS. Uses dedicated CLI tokens (not session JWTs) stored in `~/.config/dwellir/`. Output defaults to human-readable, configurable to JSON. PostHog telemetry built in.

**Tech Stack:** Go 1.24+, Cobra, Viper, net/http, creativeprojects/go-selfupdate, posthog/posthog-go, GoReleaser

**Design Doc:** `docs/plans/2026-02-25-dwellir-cli-design.md`

**Repo:** `github.com/dwellir-public/cli`

**Marly API Base:** `https://dashboard.dwellir.com/marly-api` (production), configurable via `DWELLIR_API_URL` env var

---

## Task 1: Scaffold Go Module and Repo Structure — [API-355](https://linear.app/dwellir/issue/API-355)

**Files:**
- Create: `go.mod`
- Create: `cmd/dwellir/main.go`
- Create: `internal/cli/root.go`
- Create: `Makefile`
- Create: `.golangci.yml`
- Create: `.gitignore`
- Create: `AGENTS.md`
- Create: `CLAUDE.md` (symlink to AGENTS.md)
- Create: `README.md`

**Step 1: Initialize Go module**

```bash
cd ~/src/dwellir/api-platform/cli
go mod init github.com/dwellir-public/cli
```

**Step 2: Install core dependencies**

```bash
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
```

**Step 3: Write main.go**

```go
// cmd/dwellir/main.go
package main

import (
	"os"

	"github.com/dwellir-public/cli/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
```

**Step 4: Write root command**

```go
// internal/cli/root.go
package cli

import (
	"github.com/spf13/cobra"
)

var (
	jsonOutput  bool
	humanOutput bool
	profile     string
	quiet       bool
	anonTelemetry bool
)

var rootCmd = &cobra.Command{
	Use:   "dwellir",
	Short: "Dwellir CLI — Blockchain RPC infrastructure from your terminal",
	Long: `Dwellir CLI provides full access to the Dwellir platform.

Manage API keys, browse blockchain endpoints, view usage analytics,
and debug error logs — all from the command line.

Get started:
  dwellir auth login       Authenticate with your Dwellir account
  dwellir endpoints list   Browse available blockchain endpoints
  dwellir keys list        List your API keys

Documentation: https://dwellir.com/docs
Dashboard:     https://dashboard.dwellir.com`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&humanOutput, "human", false, "Output as human-readable (default)")
	rootCmd.PersistentFlags().StringVar(&profile, "profile", "", "Use a specific auth profile")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&anonTelemetry, "anon-telemetry", false, "Anonymize telemetry data")
}

func Execute() error {
	return rootCmd.Execute()
}
```

**Step 5: Write Makefile**

```makefile
.PHONY: build test test-e2e lint fmt check run clean

build:
	go build -o bin/dwellir ./cmd/dwellir

test:
	go test ./...

test-e2e:
	go test ./test/e2e/ -tags=e2e -v

lint:
	golangci-lint run

fmt:
	goimports -w .

check: fmt lint test

run:
	go run ./cmd/dwellir

clean:
	rm -rf bin/
```

**Step 6: Write .golangci.yml**

```yaml
run:
  timeout: 5m

linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports

linters-settings:
  goimports:
    local-prefixes: github.com/dwellir-public/cli
```

**Step 7: Write .gitignore**

```
bin/
dist/
*.exe
.DS_Store
```

**Step 8: Write AGENTS.md**

```markdown
# Dwellir CLI — Agent Instructions

## Quick Reference

- Build: `go build -o bin/dwellir ./cmd/dwellir`
- Test: `go test ./...`
- Lint: `golangci-lint run`
- Format: `goimports -w .`
- E2E: `go test ./test/e2e/ -tags=e2e -v`
- Run: `./bin/dwellir <command>`
- Full check: `make check` (format + lint + test)

## Development Loop

1. Write/edit code
2. Run: `goimports -w . && golangci-lint run && go test ./...`
3. If lint/test fails, fix and repeat step 2
4. For new commands: `go build -o bin/dwellir ./cmd/dwellir && ./bin/dwellir <command>`
5. E2E tests: `go test ./test/e2e/ -tags=e2e -run TestName -v`

## Architecture

- `cmd/dwellir/main.go` — Entry point
- `internal/cli/` — Cobra command definitions (one file per command group)
- `internal/api/` — Marly API client (one file per domain)
- `internal/config/` — Config + profile management
- `internal/output/` — JSON + human output formatters
- `internal/auth/` — Browser auth flow + token resolution
- `internal/telemetry/` — PostHog integration
- `test/e2e/` — End-to-end tests (build + run binary)

## Rules

- Every command MUST support `--json` output
- Never use interactive prompts in default mode (only in `--human` mode, and even then sparingly)
- All errors return structured JSON with `ok: false`, error code, message, and help text
- Exit codes: 0=success, 1=error, 2=auth error, 3=not found, 4=validation error
- API client methods return `(T, error)` — never panic
- Config lives in `~/.config/dwellir/`
- Tests use `testing` package with table-driven tests
- E2E tests build the binary and assert on stdout + exit codes

## Coding Standards

- Go 1.24+, gofmt with tabs
- golangci-lint for linting
- goimports for import organization
- Conventional commits: `feat:`, `fix:`, `chore:`, `refactor:`, `test:`
- Line length: keep reasonable (~120 chars), not enforced
```

**Step 9: Create CLAUDE.md symlink**

```bash
cd ~/src/dwellir/api-platform/cli
ln -s AGENTS.md CLAUDE.md
```

**Step 10: Verify build**

```bash
make build
./bin/dwellir --help
```

Expected: Help text with "Dwellir CLI" header and available commands.

**Step 11: Commit**

```bash
git init
git add -A
git commit -m "chore: scaffold dwellir CLI repo with Go module, Cobra root command, and tooling"
```

---

## Task 2: Output Formatter — [API-355](https://linear.app/dwellir/issue/API-355)

**Files:**
- Create: `internal/output/format.go`
- Create: `internal/output/json.go`
- Create: `internal/output/human.go`
- Create: `internal/output/format_test.go`

**Step 1: Write failing test**

```go
// internal/output/format_test.go
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
	// Should contain "ok":true
	if !bytes.Contains(buf.Bytes(), []byte(`"ok":true`)) {
		t.Errorf("expected ok:true in output, got: %s", got)
	}
}

func TestJSONError(t *testing.T) {
	var buf bytes.Buffer
	f := NewJSONFormatter(&buf)
	err := f.Error("not_authenticated", "No token found.", "Run 'dwellir auth login'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/output/ -v
```

Expected: FAIL — types not defined.

**Step 3: Write format.go (interface + types)**

```go
// internal/output/format.go
package output

import "io"

// Response is the JSON envelope for all CLI output.
type Response struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error *ErrorBody  `json:"error,omitempty"`
	Meta  *Meta       `json:"meta,omitempty"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Help    string `json:"help,omitempty"`
}

type Meta struct {
	Command   string `json:"command"`
	Timestamp string `json:"timestamp"`
	Profile   string `json:"profile,omitempty"`
}

// Formatter defines how CLI output is rendered.
type Formatter interface {
	Success(command string, data interface{}) error
	Error(code string, message string, help string) error
	Write(data interface{}) error
}

// New returns a Formatter based on the format string ("json" or "human").
func New(format string, w io.Writer) Formatter {
	if format == "json" {
		return NewJSONFormatter(w)
	}
	return NewHumanFormatter(w)
}
```

**Step 4: Write json.go**

```go
// internal/output/json.go
package output

import (
	"encoding/json"
	"io"
	"time"
)

type JSONFormatter struct {
	w io.Writer
}

func NewJSONFormatter(w io.Writer) *JSONFormatter {
	return &JSONFormatter{w: w}
}

func (f *JSONFormatter) Success(command string, data interface{}) error {
	resp := Response{
		OK:   true,
		Data: data,
		Meta: &Meta{
			Command:   command,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
	return f.encode(resp)
}

func (f *JSONFormatter) Error(code string, message string, help string) error {
	resp := Response{
		OK: false,
		Error: &ErrorBody{
			Code:    code,
			Message: message,
			Help:    help,
		},
	}
	return f.encode(resp)
}

func (f *JSONFormatter) Write(data interface{}) error {
	return f.encode(data)
}

func (f *JSONFormatter) encode(v interface{}) error {
	enc := json.NewEncoder(f.w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
```

**Step 5: Write human.go**

```go
// internal/output/human.go
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

type HumanFormatter struct {
	w io.Writer
}

func NewHumanFormatter(w io.Writer) *HumanFormatter {
	return &HumanFormatter{w: w}
}

func (f *HumanFormatter) Success(command string, data interface{}) error {
	return f.Write(data)
}

func (f *HumanFormatter) Error(code string, message string, help string) error {
	fmt.Fprintf(f.w, "Error: %s\n", message)
	if help != "" {
		fmt.Fprintf(f.w, "\n%s\n", help)
	}
	return nil
}

func (f *HumanFormatter) Write(data interface{}) error {
	switch v := data.(type) {
	case []map[string]interface{}:
		return f.writeTable(v)
	case map[string]interface{}:
		return f.writeKeyValue(v)
	default:
		// Fall back to indented JSON for complex types
		enc := json.NewEncoder(f.w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
}

func (f *HumanFormatter) writeTable(rows []map[string]interface{}) error {
	if len(rows) == 0 {
		fmt.Fprintln(f.w, "No results.")
		return nil
	}
	tw := tabwriter.NewWriter(f.w, 0, 0, 2, ' ', 0)
	// Use keys from first row as headers
	var headers []string
	for k := range rows[0] {
		headers = append(headers, k)
	}
	for _, h := range headers {
		fmt.Fprintf(tw, "%s\t", h)
	}
	fmt.Fprintln(tw)
	for _, row := range rows {
		for _, h := range headers {
			fmt.Fprintf(tw, "%v\t", row[h])
		}
		fmt.Fprintln(tw)
	}
	return tw.Flush()
}

func (f *HumanFormatter) writeKeyValue(data map[string]interface{}) error {
	tw := tabwriter.NewWriter(f.w, 0, 0, 2, ' ', 0)
	for k, v := range data {
		fmt.Fprintf(tw, "%s:\t%v\n", k, v)
	}
	return tw.Flush()
}
```

**Step 6: Run tests**

```bash
go test ./internal/output/ -v
```

Expected: PASS

**Step 7: Commit**

```bash
git add internal/output/
git commit -m "feat: add output formatter with JSON envelope and human-readable modes"
```

---

## Task 3: Config and Profile System — [API-355](https://linear.app/dwellir/issue/API-355)

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/profiles.go`
- Create: `internal/config/config_test.go`
- Create: `internal/cli/config.go`

**Step 1: Write failing test**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefault(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Output != "human" {
		t.Errorf("expected default output 'human', got '%s'", cfg.Output)
	}
	if cfg.DefaultProfile != "default" {
		t.Errorf("expected default profile 'default', got '%s'", cfg.DefaultProfile)
	}
}

func TestSetAndGet(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := Load(dir)
	err := cfg.Set("output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val := cfg.Get("output")
	if val != "json" {
		t.Errorf("expected 'json', got '%s'", val)
	}
}

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := Load(dir)
	cfg.Set("output", "json")
	err := cfg.Save()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg2, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg2.Output != "json" {
		t.Errorf("expected 'json' after reload, got '%s'", cfg2.Output)
	}
}

func TestProfileSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	p := &Profile{
		Name:  "work",
		Token: "test-token-123",
		Org:   "my-org",
		User:  "me@example.com",
	}
	err := SaveProfile(dir, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	loaded, err := LoadProfile(dir, "work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.Token != "test-token-123" {
		t.Errorf("expected token 'test-token-123', got '%s'", loaded.Token)
	}
}

func TestResolveProfileFromDwellirJSON(t *testing.T) {
	configDir := t.TempDir()
	projectDir := t.TempDir()

	// Write .dwellir.json
	content := []byte(`{"profile": "work"}`)
	os.WriteFile(filepath.Join(projectDir, ".dwellir.json"), content, 0644)

	name := ResolveProfileName("", "", projectDir, configDir)
	if name != "work" {
		t.Errorf("expected 'work' from .dwellir.json, got '%s'", name)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/config/ -v
```

Expected: FAIL — types not defined.

**Step 3: Write config.go**

```go
// internal/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Output         string `json:"output"`
	DefaultProfile string `json:"default_profile"`
	configDir      string
}

var validKeys = map[string]bool{
	"output":          true,
	"default_profile": true,
}

func Load(configDir string) (*Config, error) {
	cfg := &Config{
		Output:         "human",
		DefaultProfile: "default",
		configDir:      configDir,
	}

	path := filepath.Join(configDir, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	cfg.configDir = configDir
	return cfg, nil
}

func (c *Config) Set(key, value string) error {
	if !validKeys[key] {
		return fmt.Errorf("unknown config key: %s (valid keys: output, default_profile)", key)
	}
	switch key {
	case "output":
		if value != "json" && value != "human" {
			return fmt.Errorf("output must be 'json' or 'human'")
		}
		c.Output = value
	case "default_profile":
		c.DefaultProfile = value
	}
	return c.Save()
}

func (c *Config) Get(key string) string {
	switch key {
	case "output":
		return c.Output
	case "default_profile":
		return c.DefaultProfile
	default:
		return ""
	}
}

func (c *Config) All() map[string]string {
	return map[string]string{
		"output":          c.Output,
		"default_profile": c.DefaultProfile,
	}
}

func (c *Config) Save() error {
	if err := os.MkdirAll(c.configDir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	path := filepath.Join(c.configDir, "config.json")
	return os.WriteFile(path, data, 0600)
}

// DefaultConfigDir returns the XDG-compliant config directory.
func DefaultConfigDir() string {
	if dir := os.Getenv("DWELLIR_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dwellir")
}
```

**Step 4: Write profiles.go**

```go
// internal/config/profiles.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Profile struct {
	Name  string `json:"name"`
	Token string `json:"token"`
	Org   string `json:"org,omitempty"`
	User  string `json:"user,omitempty"`
}

type dwellirJSON struct {
	Profile string `json:"profile"`
}

func SaveProfile(configDir string, p *Profile) error {
	dir := filepath.Join(configDir, "profiles")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating profiles dir: %w", err)
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling profile: %w", err)
	}
	path := filepath.Join(dir, p.Name+".json")
	return os.WriteFile(path, data, 0600)
}

func LoadProfile(configDir string, name string) (*Profile, error) {
	path := filepath.Join(configDir, "profiles", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading profile '%s': %w", name, err)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing profile '%s': %w", name, err)
	}
	return &p, nil
}

func ListProfiles(configDir string) ([]string, error) {
	dir := filepath.Join(configDir, "profiles")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name()[:len(e.Name())-5])
		}
	}
	return names, nil
}

// ResolveProfileName determines which profile to use based on priority:
// 1. Explicit flag (--profile)
// 2. DWELLIR_PROFILE env var
// 3. .dwellir.json in cwd or parent dirs
// 4. Config default_profile
// 5. "default"
func ResolveProfileName(flagValue string, envValue string, cwd string, configDir string) string {
	if flagValue != "" {
		return flagValue
	}
	if envValue != "" {
		return envValue
	}
	if name := findDwellirJSON(cwd); name != "" {
		return name
	}
	cfg, err := Load(configDir)
	if err == nil && cfg.DefaultProfile != "" {
		return cfg.DefaultProfile
	}
	return "default"
}

func findDwellirJSON(dir string) string {
	for {
		path := filepath.Join(dir, ".dwellir.json")
		data, err := os.ReadFile(path)
		if err == nil {
			var d dwellirJSON
			if json.Unmarshal(data, &d) == nil && d.Profile != "" {
				return d.Profile
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
```

**Step 5: Run tests**

```bash
go test ./internal/config/ -v
```

Expected: PASS

**Step 6: Write config CLI commands**

```go
// internal/cli/config.go
package cli

import (
	"fmt"

	"github.com/dwellir-public/cli/internal/config"
	"github.com/dwellir-public/cli/internal/output"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure CLI settings",
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Long:  "Set a CLI configuration value.\n\nValid keys: output (json|human), default_profile (<name>)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultConfigDir())
		if err != nil {
			return err
		}
		if err := cfg.Set(args[0], args[1]); err != nil {
			return err
		}
		f := getFormatter()
		return f.Success("config.set", map[string]string{args[0]: args[1]})
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultConfigDir())
		if err != nil {
			return err
		}
		val := cfg.Get(args[0])
		if val == "" {
			return fmt.Errorf("unknown config key: %s", args[0])
		}
		f := getFormatter()
		return f.Success("config.get", map[string]string{args[0]: val})
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all config values",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultConfigDir())
		if err != nil {
			return err
		}
		f := getFormatter()
		return f.Success("config.list", cfg.All())
	},
}

func init() {
	configCmd.AddCommand(configSetCmd, configGetCmd, configListCmd)
	rootCmd.AddCommand(configCmd)
}

func getFormatter() output.Formatter {
	cfg, _ := config.Load(config.DefaultConfigDir())
	format := cfg.Output
	if jsonOutput {
		format = "json"
	}
	if humanOutput {
		format = "human"
	}
	return output.New(format, rootCmd.OutOrStdout())
}
```

**Step 7: Build and verify**

```bash
make build
./bin/dwellir config set output json
./bin/dwellir config get output
./bin/dwellir config list
```

**Step 8: Commit**

```bash
git add internal/config/ internal/cli/config.go
git commit -m "feat: add config and profile system with per-directory .dwellir.json support"
```

---

## Task 4: Marly API Client — [API-356](https://linear.app/dwellir/issue/API-356)

**Files:**
- Create: `internal/api/client.go`
- Create: `internal/api/client_test.go`

**Step 1: Write failing test**

```go
// internal/api/client_test.go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer token, got: %s", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/v3/chains" {
			t.Errorf("expected /v3/chains, got: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]map[string]string{{"name": "Ethereum"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	var result []map[string]string
	err := client.Get("/v3/chains", nil, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0]["name"] != "Ethereum" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestClientPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got: %s", r.Method)
		}
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	var result map[string]bool
	err := client.Post("/v4/organization/analytics", map[string]string{"interval": "day"}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result["ok"] {
		t.Error("expected ok: true")
	}
}

func TestClientUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"detail": "Not authenticated"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-token")
	var result map[string]string
	err := client.Get("/v4/user", nil, &result)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got: %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("expected 401, got: %d", apiErr.StatusCode)
	}
}

func TestClientTokenRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Dwellir-Refreshed-Token", "new-token-456")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	var refreshedToken string
	client := NewClient(server.URL, "old-token")
	client.OnTokenRefresh = func(newToken string) {
		refreshedToken = newToken
	}

	var result map[string]string
	client.Get("/v4/user", nil, &result)
	if refreshedToken != "new-token-456" {
		t.Errorf("expected refreshed token, got: '%s'", refreshedToken)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/api/ -v
```

**Step 3: Write client.go**

```go
// internal/api/client.go
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultTimeout = 30 * time.Second

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (HTTP %d): %s", e.StatusCode, e.Body)
}

type Client struct {
	baseURL        string
	token          string
	httpClient     *http.Client
	OnTokenRefresh func(newToken string)
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

func (c *Client) Get(path string, params map[string]string, result interface{}) error {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("parsing URL: %w", err)
	}
	if params != nil {
		q := u.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	return c.do(req, result)
}

func (c *Client) Post(path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req, result)
}

func (c *Client) Delete(path string, result interface{}) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	return c.do(req, result)
}

func (c *Client) Patch(path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(http.MethodPatch, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req, result)
}

func (c *Client) do(req *http.Request, result interface{}) error {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "dwellir-cli")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for token refresh
	if refreshed := resp.Header.Get("X-Dwellir-Refreshed-Token"); refreshed != "" && c.OnTokenRefresh != nil {
		c.OnTokenRefresh(refreshed)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	if result != nil && len(body) > 0 {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}
	}
	return nil
}
```

**Step 4: Run tests**

```bash
go test ./internal/api/ -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/
git commit -m "feat: add Marly API client with token auth, auto-refresh, and error handling"
```

---

## Task 5: Auth — Token Resolution and Login Flow — [API-356](https://linear.app/dwellir/issue/API-356)

**Files:**
- Create: `internal/auth/resolve.go`
- Create: `internal/auth/login.go`
- Create: `internal/auth/resolve_test.go`
- Create: `internal/cli/auth.go`

**Step 1: Write failing test for token resolution**

```go
// internal/auth/resolve_test.go
package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dwellir-public/cli/internal/config"
)

func TestResolveTokenFromEnv(t *testing.T) {
	t.Setenv("DWELLIR_TOKEN", "env-token-123")
	token, err := ResolveToken("", "", t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "env-token-123" {
		t.Errorf("expected env token, got: '%s'", token)
	}
}

func TestResolveTokenFromFlag(t *testing.T) {
	token, err := ResolveToken("flag-token", "", t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "flag-token" {
		t.Errorf("expected flag token, got: '%s'", token)
	}
}

func TestResolveTokenFromProfile(t *testing.T) {
	configDir := t.TempDir()
	p := &config.Profile{
		Name:  "default",
		Token: "profile-token",
	}
	config.SaveProfile(configDir, p)

	token, err := ResolveToken("", "", t.TempDir(), configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "profile-token" {
		t.Errorf("expected profile token, got: '%s'", token)
	}
}

func TestResolveTokenFromDwellirJSON(t *testing.T) {
	configDir := t.TempDir()
	projectDir := t.TempDir()

	// Save a "work" profile
	p := &config.Profile{Name: "work", Token: "work-token"}
	config.SaveProfile(configDir, p)

	// Write .dwellir.json pointing to "work"
	os.WriteFile(filepath.Join(projectDir, ".dwellir.json"), []byte(`{"profile":"work"}`), 0644)

	token, err := ResolveToken("", "", projectDir, configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "work-token" {
		t.Errorf("expected work token, got: '%s'", token)
	}
}

func TestResolveTokenMissing(t *testing.T) {
	_, err := ResolveToken("", "", t.TempDir(), t.TempDir())
	if err == nil {
		t.Fatal("expected error when no token is available")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/auth/ -v
```

**Step 3: Write resolve.go**

```go
// internal/auth/resolve.go
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
	// 1. Env var (highest priority)
	if envToken := os.Getenv("DWELLIR_TOKEN"); envToken != "" {
		return envToken, nil
	}

	// 2. Explicit --token flag
	if tokenFlag != "" {
		return tokenFlag, nil
	}

	// 3. Resolve profile and load token
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
```

**Step 4: Write login.go (browser auth flow)**

```go
// internal/auth/login.go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/dwellir-public/cli/internal/config"
)

const loginTimeout = 5 * time.Minute

type CallbackPayload struct {
	Token string `json:"token"`
	Org   string `json:"org"`
	User  string `json:"user"`
}

// Login starts the browser-based auth flow:
// 1. Starts a local HTTP server
// 2. Opens browser to dashboard.dwellir.com/cli-auth
// 3. Waits for callback with token
// 4. Saves token to profile
func Login(configDir string, profileName string, dashboardURL string) (*config.Profile, error) {
	if profileName == "" {
		profileName = "default"
	}

	// Find a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting local server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	resultCh := make(chan *CallbackPayload, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload CallbackPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			errCh <- fmt.Errorf("invalid callback payload: %w", err)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", dashboardURL)
		w.Header().Set("Access-Control-Allow-Methods", "POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
		resultCh <- &payload
	})

	// Handle CORS preflight
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", dashboardURL)
			w.Header().Set("Access-Control-Allow-Methods", "POST")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
		}
	})

	server := &http.Server{Handler: mux}
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	defer server.Shutdown(context.Background())

	// Open browser
	authURL := fmt.Sprintf("%s/cli-auth?port=%d", dashboardURL, port)
	fmt.Fprintf(config.Stderr(), "Opening browser for authentication...\n")
	fmt.Fprintf(config.Stderr(), "If the browser doesn't open, visit:\n  %s\n\n", authURL)
	openBrowser(authURL)

	// Wait for callback or timeout
	select {
	case payload := <-resultCh:
		p := &config.Profile{
			Name:  profileName,
			Token: payload.Token,
			Org:   payload.Org,
			User:  payload.User,
		}
		if err := config.SaveProfile(configDir, p); err != nil {
			return nil, fmt.Errorf("saving profile: %w", err)
		}
		return p, nil

	case err := <-errCh:
		return nil, err

	case <-time.After(loginTimeout):
		return nil, fmt.Errorf("authentication timed out after %s.\n\nFor headless/CI environments, create a token manually at:\n  %s/agents", loginTimeout, dashboardURL)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}
```

Note: `config.Stderr()` is a small helper that returns `os.Stderr` — add this to config.go:

```go
// Add to internal/config/config.go
func Stderr() io.Writer {
	return os.Stderr
}
```

**Step 5: Run tests**

```bash
go test ./internal/auth/ -v
```

Expected: PASS

**Step 6: Write auth CLI commands**

```go
// internal/cli/auth.go
package cli

import (
	"fmt"
	"os"

	"github.com/dwellir-public/cli/internal/auth"
	"github.com/dwellir-public/cli/internal/config"
	"github.com/spf13/cobra"
)

const defaultDashboardURL = "https://dashboard.dwellir.com"

var tokenFlag string

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Dwellir",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate via browser or token",
	Long:  "Opens a browser window for authentication.\n\nFor headless/CI: dwellir auth login --token <TOKEN>",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		f := getFormatter()

		// Direct token mode (headless/CI)
		if tokenFlag != "" {
			profileName := profile
			if profileName == "" {
				profileName = "default"
			}
			p := &config.Profile{
				Name:  profileName,
				Token: tokenFlag,
			}
			if err := config.SaveProfile(configDir, p); err != nil {
				return err
			}
			return f.Success("auth.login", map[string]string{
				"status":  "authenticated",
				"profile": profileName,
				"method":  "token",
			})
		}

		// Browser flow
		dashboardURL := os.Getenv("DWELLIR_DASHBOARD_URL")
		if dashboardURL == "" {
			dashboardURL = defaultDashboardURL
		}

		p, err := auth.Login(configDir, profile, dashboardURL)
		if err != nil {
			return f.Error("auth_failed", err.Error(), "")
		}

		return f.Success("auth.login", map[string]string{
			"status":  "authenticated",
			"profile": p.Name,
			"user":    p.User,
			"org":     p.Org,
			"method":  "browser",
		})
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear local credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		f := getFormatter()
		profileName := profile
		if profileName == "" {
			profileName = "default"
		}
		profilePath := fmt.Sprintf("%s/profiles/%s.json", configDir, profileName)
		os.Remove(profilePath)
		return f.Success("auth.logout", map[string]string{
			"status":  "logged_out",
			"profile": profileName,
		})
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current auth state",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		f := getFormatter()

		cwd, _ := os.Getwd()
		envProfile := os.Getenv("DWELLIR_PROFILE")
		profileName := config.ResolveProfileName(profile, envProfile, cwd, configDir)

		p, err := config.LoadProfile(configDir, profileName)
		if err != nil {
			return f.Error("not_authenticated", "No active session.", "Run 'dwellir auth login' to authenticate.")
		}

		return f.Success("auth.status", map[string]string{
			"profile": profileName,
			"user":    p.User,
			"org":     p.Org,
		})
	},
}

var authTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Print current access token",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		cwd, _ := os.Getwd()
		token, err := auth.ResolveToken(tokenFlag, profile, cwd, configDir)
		if err != nil {
			f := getFormatter()
			return f.Error("not_authenticated", err.Error(), "")
		}
		fmt.Fprintln(cmd.OutOrStdout(), token)
		return nil
	},
}

func init() {
	authLoginCmd.Flags().StringVar(&tokenFlag, "token", "", "Authenticate with a CLI token directly (for headless/CI)")
	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authStatusCmd, authTokenCmd)
	rootCmd.AddCommand(authCmd)
}
```

**Step 7: Build and verify**

```bash
make build
./bin/dwellir auth status --json
./bin/dwellir auth login --token test-123 --json
./bin/dwellir auth status --json
./bin/dwellir auth logout --json
```

**Step 8: Commit**

```bash
git add internal/auth/ internal/cli/auth.go
git commit -m "feat: add auth system with browser login, token resolution, and profile management"
```

---

## Task 6: Endpoints Commands — [API-357](https://linear.app/dwellir/issue/API-357)

**Files:**
- Create: `internal/api/endpoints.go`
- Create: `internal/api/endpoints_test.go`
- Create: `internal/cli/endpoints.go`

**Step 1: Write failing test for API client**

```go
// internal/api/endpoints_test.go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testChain struct {
	ID       int           `json:"id"`
	Name     string        `json:"name"`
	ImageURL string        `json:"image_url"`
	Networks []testNetwork `json:"networks"`
}

type testNetwork struct {
	ID    int        `json:"id"`
	Name  string     `json:"name"`
	Nodes []testNode `json:"nodes"`
}

type testNode struct {
	ID       int          `json:"id"`
	HTTPS    string       `json:"https"`
	WSS      string       `json:"wss"`
	NodeType testNodeType `json:"node_type"`
}

type testNodeType struct {
	Name string `json:"name"`
}

func TestListEndpoints(t *testing.T) {
	chains := []testChain{
		{
			ID: 1, Name: "Ethereum", ImageURL: "eth.png",
			Networks: []testNetwork{
				{
					ID: 1, Name: "Mainnet",
					Nodes: []testNode{
						{ID: 1, HTTPS: "https://eth.dwellir.com", WSS: "wss://eth.dwellir.com", NodeType: testNodeType{Name: "full"}},
					},
				},
			},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(chains)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ep := NewEndpointsAPI(client)
	result, err := ep.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 chain, got %d", len(result))
	}
	if result[0].Name != "Ethereum" {
		t.Errorf("expected Ethereum, got %s", result[0].Name)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/api/ -v -run TestListEndpoints
```

**Step 3: Write endpoints.go API client**

```go
// internal/api/endpoints.go
package api

import "strings"

type Chain struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	ImageURL  string    `json:"image_url"`
	Ecosystem string    `json:"ecosystem,omitempty"`
	Networks  []Network `json:"networks"`
}

type Network struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Nodes []Node `json:"nodes"`
}

type Node struct {
	ID       int      `json:"id"`
	HTTPS    string   `json:"https"`
	WSS      string   `json:"wss"`
	NodeType NodeType `json:"node_type"`
}

type NodeType struct {
	Name string `json:"name"`
}

type EndpointsAPI struct {
	client *Client
}

func NewEndpointsAPI(client *Client) *EndpointsAPI {
	return &EndpointsAPI{client: client}
}

func (e *EndpointsAPI) List() ([]Chain, error) {
	var chains []Chain
	err := e.client.Get("/v3/chains", nil, &chains)
	return chains, err
}

// Search filters chains by query string (matches chain name, network name).
func (e *EndpointsAPI) Search(query string, ecosystem string, nodeType string, protocol string) ([]Chain, error) {
	chains, err := e.List()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var filtered []Chain

	for _, chain := range chains {
		if ecosystem != "" && !strings.EqualFold(chain.Ecosystem, ecosystem) {
			continue
		}
		chainMatch := query == "" || strings.Contains(strings.ToLower(chain.Name), query)

		var matchedNetworks []Network
		for _, net := range chain.Networks {
			netMatch := chainMatch || strings.Contains(strings.ToLower(net.Name), query)
			if !netMatch {
				continue
			}

			var matchedNodes []Node
			for _, node := range net.Nodes {
				if nodeType != "" && !strings.EqualFold(node.NodeType.Name, nodeType) {
					continue
				}
				if protocol == "https" && node.HTTPS == "" {
					continue
				}
				if protocol == "wss" && node.WSS == "" {
					continue
				}
				matchedNodes = append(matchedNodes, node)
			}

			if len(matchedNodes) > 0 {
				net.Nodes = matchedNodes
				matchedNetworks = append(matchedNetworks, net)
			}
		}

		if len(matchedNetworks) > 0 {
			chain.Networks = matchedNetworks
			filtered = append(filtered, chain)
		}
	}

	return filtered, nil
}
```

**Step 4: Run tests**

```bash
go test ./internal/api/ -v
```

Expected: PASS

**Step 5: Write endpoints CLI command**

```go
// internal/cli/endpoints.go
package cli

import (
	"os"

	"github.com/dwellir-public/cli/internal/api"
	"github.com/dwellir-public/cli/internal/auth"
	"github.com/dwellir-public/cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	epChain     string
	epNetwork   string
	epProtocol  string
	epNodeType  string
	epEcosystem string
)

var endpointsCmd = &cobra.Command{
	Use:   "endpoints",
	Short: "Browse and search blockchain endpoints",
}

var endpointsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available endpoints",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			f := getFormatter()
			return f.Error("not_authenticated", err.Error(), "")
		}
		ep := api.NewEndpointsAPI(client)
		chains, err := ep.Search("", epEcosystem, epNodeType, epProtocol)
		if err != nil {
			return err
		}
		f := getFormatter()
		return f.Success("endpoints.list", chains)
	},
}

var endpointsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search endpoints by chain or network name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			f := getFormatter()
			return f.Error("not_authenticated", err.Error(), "")
		}
		ep := api.NewEndpointsAPI(client)
		chains, err := ep.Search(args[0], epEcosystem, epNodeType, epProtocol)
		if err != nil {
			return err
		}
		f := getFormatter()
		return f.Success("endpoints.search", chains)
	},
}

var endpointsGetCmd = &cobra.Command{
	Use:   "get <chain>",
	Short: "Get endpoints for a specific chain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			f := getFormatter()
			return f.Error("not_authenticated", err.Error(), "")
		}
		ep := api.NewEndpointsAPI(client)
		chains, err := ep.Search(args[0], "", "", "")
		if err != nil {
			return err
		}
		f := getFormatter()
		if len(chains) == 0 {
			return f.Error("not_found", "No endpoints found for '"+args[0]+"'.", "Run 'dwellir endpoints list' to see all available chains.")
		}
		return f.Success("endpoints.get", chains)
	},
}

func init() {
	for _, cmd := range []*cobra.Command{endpointsListCmd, endpointsSearchCmd, endpointsGetCmd} {
		cmd.Flags().StringVar(&epEcosystem, "ecosystem", "", "Filter by ecosystem (evm, substrate, cosmos, solana)")
		cmd.Flags().StringVar(&epNodeType, "node-type", "", "Filter by node type (full, archive)")
		cmd.Flags().StringVar(&epProtocol, "protocol", "", "Filter by protocol (https, wss)")
	}
	endpointsCmd.AddCommand(endpointsListCmd, endpointsSearchCmd, endpointsGetCmd)
	rootCmd.AddCommand(endpointsCmd)
}

// newAPIClient creates a Marly API client from the resolved auth token.
func newAPIClient() (*api.Client, error) {
	configDir := config.DefaultConfigDir()
	cwd, _ := os.Getwd()
	token, err := auth.ResolveToken(tokenFlag, profile, cwd, configDir)
	if err != nil {
		return nil, err
	}

	baseURL := os.Getenv("DWELLIR_API_URL")
	if baseURL == "" {
		baseURL = "https://dashboard.dwellir.com/marly-api"
	}

	client := api.NewClient(baseURL, token)

	// Wire up token refresh to save new tokens automatically
	client.OnTokenRefresh = func(newToken string) {
		envProfile := os.Getenv("DWELLIR_PROFILE")
		profileName := config.ResolveProfileName(profile, envProfile, cwd, configDir)
		p, _ := config.LoadProfile(configDir, profileName)
		if p != nil {
			p.Token = newToken
			config.SaveProfile(configDir, p)
		}
	}

	return client, nil
}
```

**Step 6: Build and verify**

```bash
make build
./bin/dwellir endpoints --help
./bin/dwellir endpoints list --json  # Will fail with auth error — expected
./bin/dwellir endpoints search ethereum --json  # Same — expected
```

**Step 7: Commit**

```bash
git add internal/api/endpoints.go internal/api/endpoints_test.go internal/cli/endpoints.go
git commit -m "feat: add endpoints commands with search, filtering by ecosystem/node-type/protocol"
```

---

## Task 7: Keys Commands — [API-357](https://linear.app/dwellir/issue/API-357)

**Files:**
- Create: `internal/api/keys.go`
- Create: `internal/api/keys_test.go`
- Create: `internal/cli/keys.go`

**Step 1: Write failing test**

```go
// internal/api/keys_test.go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		json.NewEncoder(w).Encode(keys)
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
		json.NewDecoder(r.Body).Decode(&input)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(APIKey{APIKey: "new-key", Name: input.Name, Enabled: true})
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
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/api/ -v -run TestListKeys
```

**Step 3: Write keys.go API client**

```go
// internal/api/keys.go
package api

import "fmt"

type APIKey struct {
	APIKey       string `json:"api_key"`
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	DailyQuota   *int   `json:"daily_quota,omitempty"`
	MonthlyQuota *int   `json:"monthly_quota,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type CreateKeyInput struct {
	Name         string `json:"name,omitempty"`
	DailyQuota   *int   `json:"daily_quota,omitempty"`
	MonthlyQuota *int   `json:"monthly_quota,omitempty"`
}

type UpdateKeyInput struct {
	Name         *string `json:"name,omitempty"`
	Enabled      *bool   `json:"enabled,omitempty"`
	DailyQuota   *int    `json:"daily_quota,omitempty"`
	MonthlyQuota *int    `json:"monthly_quota,omitempty"`
}

type KeysAPI struct {
	client *Client
}

func NewKeysAPI(client *Client) *KeysAPI {
	return &KeysAPI{client: client}
}

func (k *KeysAPI) List() ([]APIKey, error) {
	var keys []APIKey
	err := k.client.Get("/v3/user/apikeys", nil, &keys)
	return keys, err
}

func (k *KeysAPI) Create(input CreateKeyInput) (*APIKey, error) {
	var key APIKey
	err := k.client.Post("/v3/user/apikey", input, &key)
	return &key, err
}

func (k *KeysAPI) Update(apiKey string, input UpdateKeyInput) (*APIKey, error) {
	var key APIKey
	err := k.client.Post(fmt.Sprintf("/user/apikey/%s", apiKey), input, &key)
	return &key, err
}

func (k *KeysAPI) Delete(apiKey string) error {
	return k.client.Delete(fmt.Sprintf("/user/apikey/%s", apiKey), nil)
}

func (k *KeysAPI) Enable(apiKey string) (*APIKey, error) {
	enabled := true
	return k.Update(apiKey, UpdateKeyInput{Enabled: &enabled})
}

func (k *KeysAPI) Disable(apiKey string) (*APIKey, error) {
	enabled := false
	return k.Update(apiKey, UpdateKeyInput{Enabled: &enabled})
}
```

**Step 4: Run tests**

```bash
go test ./internal/api/ -v
```

Expected: PASS

**Step 5: Write keys CLI command**

```go
// internal/cli/keys.go
package cli

import (
	"github.com/dwellir-public/cli/internal/api"
	"github.com/spf13/cobra"
)

var (
	keyName         string
	keyDailyQuota   int
	keyMonthlyQuota int
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage API keys",
}

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		keys, err := api.NewKeysAPI(client).List()
		if err != nil {
			return err
		}
		return getFormatter().Success("keys.list", keys)
	},
}

var keysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		input := api.CreateKeyInput{Name: keyName}
		if keyDailyQuota > 0 {
			input.DailyQuota = &keyDailyQuota
		}
		if keyMonthlyQuota > 0 {
			input.MonthlyQuota = &keyMonthlyQuota
		}
		key, err := api.NewKeysAPI(client).Create(input)
		if err != nil {
			return err
		}
		return getFormatter().Success("keys.create", key)
	},
}

var keysUpdateCmd = &cobra.Command{
	Use:   "update <key-id>",
	Short: "Update an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		input := api.UpdateKeyInput{}
		if cmd.Flags().Changed("name") {
			input.Name = &keyName
		}
		if cmd.Flags().Changed("daily-quota") {
			input.DailyQuota = &keyDailyQuota
		}
		if cmd.Flags().Changed("monthly-quota") {
			input.MonthlyQuota = &keyMonthlyQuota
		}
		key, err := api.NewKeysAPI(client).Update(args[0], input)
		if err != nil {
			return err
		}
		return getFormatter().Success("keys.update", key)
	},
}

var keysDeleteCmd = &cobra.Command{
	Use:   "delete <key-id>",
	Short: "Delete an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		err = api.NewKeysAPI(client).Delete(args[0])
		if err != nil {
			return err
		}
		return getFormatter().Success("keys.delete", map[string]string{"key": args[0], "status": "deleted"})
	},
}

var keysEnableCmd = &cobra.Command{
	Use:   "enable <key-id>",
	Short: "Enable an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		key, err := api.NewKeysAPI(client).Enable(args[0])
		if err != nil {
			return err
		}
		return getFormatter().Success("keys.enable", key)
	},
}

var keysDisableCmd = &cobra.Command{
	Use:   "disable <key-id>",
	Short: "Disable an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		key, err := api.NewKeysAPI(client).Disable(args[0])
		if err != nil {
			return err
		}
		return getFormatter().Success("keys.disable", key)
	},
}

func init() {
	keysCreateCmd.Flags().StringVar(&keyName, "name", "", "Key name")
	keysCreateCmd.Flags().IntVar(&keyDailyQuota, "daily-quota", 0, "Daily request quota")
	keysCreateCmd.Flags().IntVar(&keyMonthlyQuota, "monthly-quota", 0, "Monthly request quota")

	keysUpdateCmd.Flags().StringVar(&keyName, "name", "", "New key name")
	keysUpdateCmd.Flags().IntVar(&keyDailyQuota, "daily-quota", 0, "Daily request quota")
	keysUpdateCmd.Flags().IntVar(&keyMonthlyQuota, "monthly-quota", 0, "Monthly request quota")

	keysCmd.AddCommand(keysListCmd, keysCreateCmd, keysUpdateCmd, keysDeleteCmd, keysEnableCmd, keysDisableCmd)
	rootCmd.AddCommand(keysCmd)
}
```

**Step 6: Build and verify**

```bash
make build
./bin/dwellir keys --help
./bin/dwellir keys list --json
./bin/dwellir keys create --help
```

**Step 7: Commit**

```bash
git add internal/api/keys.go internal/api/keys_test.go internal/cli/keys.go
git commit -m "feat: add keys commands (list, create, update, delete, enable, disable)"
```

---

## Task 8: Usage Commands — [API-358](https://linear.app/dwellir/issue/API-358)

**Files:**
- Create: `internal/api/usage.go`
- Create: `internal/cli/usage.go`

**Step 1: Write usage API client**

```go
// internal/api/usage.go
package api

type UsageSummary struct {
	TotalRequests  int    `json:"total_requests"`
	TotalResponses int    `json:"total_responses"`
	RateLimited    int    `json:"rate_limited"`
	BillingStart   string `json:"billing_start,omitempty"`
	BillingEnd     string `json:"billing_end,omitempty"`
}

type UsageHistory struct {
	Timestamp string `json:"timestamp"`
	Requests  int    `json:"requests"`
	Responses int    `json:"responses"`
}

type RPSData struct {
	Timestamp string  `json:"timestamp"`
	RPS       float64 `json:"rps"`
}

type UsageAPI struct {
	client *Client
}

func NewUsageAPI(client *Client) *UsageAPI {
	return &UsageAPI{client: client}
}

func (u *UsageAPI) Summary() (*UsageSummary, error) {
	var summary UsageSummary
	err := u.client.Get("/v4/organization/analytics/monthly_summary", nil, &summary)
	return &summary, err
}

func (u *UsageAPI) History(interval string, from string, to string) ([]UsageHistory, error) {
	body := map[string]string{"interval": interval}
	if from != "" {
		body["from"] = from
	}
	if to != "" {
		body["to"] = to
	}
	var history []UsageHistory
	err := u.client.Post("/v4/organization/analytics", body, &history)
	return history, err
}

func (u *UsageAPI) RPS() ([]RPSData, error) {
	var rps []RPSData
	err := u.client.Post("/v4/organization/analytics/rps", nil, &rps)
	return rps, err
}
```

**Step 2: Write usage CLI command**

```go
// internal/cli/usage.go
package cli

import (
	"github.com/dwellir-public/cli/internal/api"
	"github.com/spf13/cobra"
)

var (
	usageInterval string
	usageFrom     string
	usageTo       string
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "View usage analytics",
}

var usageSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Current billing cycle summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		summary, err := api.NewUsageAPI(client).Summary()
		if err != nil {
			return err
		}
		return getFormatter().Success("usage.summary", summary)
	},
}

var usageHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Usage over time",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		history, err := api.NewUsageAPI(client).History(usageInterval, usageFrom, usageTo)
		if err != nil {
			return err
		}
		return getFormatter().Success("usage.history", history)
	},
}

var usageRPSCmd = &cobra.Command{
	Use:   "rps",
	Short: "Current requests-per-second metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		rps, err := api.NewUsageAPI(client).RPS()
		if err != nil {
			return err
		}
		return getFormatter().Success("usage.rps", rps)
	},
}

func init() {
	usageHistoryCmd.Flags().StringVar(&usageInterval, "interval", "day", "Aggregation interval (minute, hour, day)")
	usageHistoryCmd.Flags().StringVar(&usageFrom, "from", "", "Start time (RFC3339)")
	usageHistoryCmd.Flags().StringVar(&usageTo, "to", "", "End time (RFC3339)")

	usageCmd.AddCommand(usageSummaryCmd, usageHistoryCmd, usageRPSCmd)
	rootCmd.AddCommand(usageCmd)
}
```

**Step 3: Build, verify, commit**

```bash
make build && ./bin/dwellir usage --help
git add internal/api/usage.go internal/cli/usage.go
git commit -m "feat: add usage commands (summary, history, rps)"
```

---

## Task 9: Logs Commands — [API-358](https://linear.app/dwellir/issue/API-358)

**Files:**
- Create: `internal/api/logs.go`
- Create: `internal/cli/logs.go`

**Step 1: Write logs API client**

```go
// internal/api/logs.go
package api

type ErrorLog struct {
	Timestamp         string `json:"timestamp"`
	RequestID         string `json:"request_id"`
	APIKey            string `json:"api_key"`
	FQDN              string `json:"fqdn"`
	StatusCode        int    `json:"response_status_code"`
	StatusLabel       string `json:"response_status_label"`
	RPCMethods        string `json:"request_rpc_methods"`
	HTTPMethod        string `json:"request_http_method"`
	ErrorMessage      string `json:"error_message"`
	BackendLatencyMs  int    `json:"backend_latency_ms"`
	TotalLatencyMs    int    `json:"total_latency_ms"`
}

type ErrorStats struct {
	StatusCode int `json:"status_code"`
	Count      int `json:"count"`
}

type ErrorFacets struct {
	FQDNs      []FacetEntry `json:"fqdns,omitempty"`
	RPCMethods []FacetEntry `json:"rpc_methods,omitempty"`
	Origins    []FacetEntry `json:"origins,omitempty"`
	APIKeys    []FacetEntry `json:"api_keys,omitempty"`
}

type FacetEntry struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type LogsAPI struct {
	client *Client
}

func NewLogsAPI(client *Client) *LogsAPI {
	return &LogsAPI{client: client}
}

func (l *LogsAPI) Errors(filters map[string]interface{}) ([]ErrorLog, error) {
	var logs []ErrorLog
	err := l.client.Post("/v4/organization/logs/errors", filters, &logs)
	return logs, err
}

func (l *LogsAPI) Stats(filters map[string]interface{}) ([]ErrorStats, error) {
	var stats []ErrorStats
	err := l.client.Post("/v4/organization/logs/errors/status_summary", filters, &stats)
	return stats, err
}

func (l *LogsAPI) Facets(filters map[string]interface{}) (*ErrorFacets, error) {
	var facets ErrorFacets
	err := l.client.Post("/v4/organization/logs/errors/facets", filters, &facets)
	return &facets, err
}
```

**Step 2: Write logs CLI command**

```go
// internal/cli/logs.go
package cli

import (
	"github.com/dwellir-public/cli/internal/api"
	"github.com/spf13/cobra"
)

var (
	logKey        string
	logEndpoint   string
	logStatusCode int
	logRPCMethod  string
	logFrom       string
	logTo         string
	logLimit      int
	logCursor     string
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View error logs",
}

var logsErrorsCmd = &cobra.Command{
	Use:   "errors",
	Short: "List error logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		filters := buildLogFilters()
		logs, err := api.NewLogsAPI(client).Errors(filters)
		if err != nil {
			return err
		}
		return getFormatter().Success("logs.errors", logs)
	},
}

var logsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Error metrics and classifications",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		filters := buildLogFilters()
		stats, err := api.NewLogsAPI(client).Stats(filters)
		if err != nil {
			return err
		}
		return getFormatter().Success("logs.stats", stats)
	},
}

var logsFacetsCmd = &cobra.Command{
	Use:   "facets",
	Short: "Error facet aggregations",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		filters := buildLogFilters()
		facets, err := api.NewLogsAPI(client).Facets(filters)
		if err != nil {
			return err
		}
		return getFormatter().Success("logs.facets", facets)
	},
}

func buildLogFilters() map[string]interface{} {
	filters := map[string]interface{}{}
	if logKey != "" {
		filters["api_key"] = logKey
	}
	if logEndpoint != "" {
		filters["fqdn"] = logEndpoint
	}
	if logStatusCode != 0 {
		filters["status_code"] = logStatusCode
	}
	if logRPCMethod != "" {
		filters["rpc_method"] = logRPCMethod
	}
	if logFrom != "" {
		filters["from"] = logFrom
	}
	if logTo != "" {
		filters["to"] = logTo
	}
	if logLimit > 0 {
		filters["limit"] = logLimit
	}
	if logCursor != "" {
		filters["cursor"] = logCursor
	}
	return filters
}

func init() {
	for _, cmd := range []*cobra.Command{logsErrorsCmd, logsStatsCmd, logsFacetsCmd} {
		cmd.Flags().StringVar(&logKey, "key", "", "Filter by API key")
		cmd.Flags().StringVar(&logEndpoint, "endpoint", "", "Filter by FQDN")
		cmd.Flags().IntVar(&logStatusCode, "status-code", 0, "Filter by HTTP status code")
		cmd.Flags().StringVar(&logRPCMethod, "rpc-method", "", "Filter by RPC method")
		cmd.Flags().StringVar(&logFrom, "from", "", "Start time (RFC3339)")
		cmd.Flags().StringVar(&logTo, "to", "", "End time (RFC3339)")
	}
	logsErrorsCmd.Flags().IntVar(&logLimit, "limit", 50, "Max results")
	logsErrorsCmd.Flags().StringVar(&logCursor, "cursor", "", "Pagination cursor")

	logsCmd.AddCommand(logsErrorsCmd, logsStatsCmd, logsFacetsCmd)
	rootCmd.AddCommand(logsCmd)
}
```

**Step 3: Build, verify, commit**

```bash
make build && ./bin/dwellir logs --help
git add internal/api/logs.go internal/cli/logs.go
git commit -m "feat: add logs commands (errors, stats, facets) with filtering"
```

---

## Task 10: Account Commands — [API-358](https://linear.app/dwellir/issue/API-358)

**Files:**
- Create: `internal/api/account.go`
- Create: `internal/cli/account.go`

**Step 1: Write account API + CLI**

```go
// internal/api/account.go
package api

type AccountInfo struct {
	Name               string `json:"name"`
	ServerLocation     string `json:"ideal_server_location,omitempty"`
	TaxID              string `json:"tax_id,omitempty"`
	Subscription       *SubscriptionInfo `json:"current_subscription,omitempty"`
}

type SubscriptionInfo struct {
	PlanName     string `json:"plan_name"`
	RateLimit    int    `json:"rate_limit"`
	BurstLimit   int    `json:"burst_limit"`
	MonthlyQuota *int   `json:"monthly_quota,omitempty"`
	DailyQuota   *int   `json:"daily_quota,omitempty"`
	APIKeysLimit int    `json:"api_keys_limit"`
}

type AccountAPI struct {
	client *Client
}

func NewAccountAPI(client *Client) *AccountAPI {
	return &AccountAPI{client: client}
}

func (a *AccountAPI) Info() (*AccountInfo, error) {
	var info AccountInfo
	err := a.client.Get("/v4/organization/information/outseta", nil, &info)
	return &info, err
}

func (a *AccountAPI) Subscription() (*SubscriptionInfo, error) {
	var sub SubscriptionInfo
	err := a.client.Get("/v3/user/subscription", nil, &sub)
	return &sub, err
}
```

```go
// internal/cli/account.go
package cli

import (
	"github.com/dwellir-public/cli/internal/api"
	"github.com/spf13/cobra"
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "View account and subscription info",
}

var accountInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Organization info, plan, billing status",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		info, err := api.NewAccountAPI(client).Info()
		if err != nil {
			return err
		}
		return getFormatter().Success("account.info", info)
	},
}

var accountSubscriptionCmd = &cobra.Command{
	Use:   "subscription",
	Short: "Current subscription details",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return getFormatter().Error("not_authenticated", err.Error(), "")
		}
		sub, err := api.NewAccountAPI(client).Subscription()
		if err != nil {
			return err
		}
		return getFormatter().Success("account.subscription", sub)
	},
}

func init() {
	accountCmd.AddCommand(accountInfoCmd, accountSubscriptionCmd)
	rootCmd.AddCommand(accountCmd)
}
```

**Step 2: Build, verify, commit**

```bash
make build && ./bin/dwellir account --help
git add internal/api/account.go internal/cli/account.go
git commit -m "feat: add account commands (info, subscription)"
```

---

## Task 11: Telemetry (PostHog) — [API-359](https://linear.app/dwellir/issue/API-359)

**Files:**
- Create: `internal/telemetry/telemetry.go`
- Create: `internal/telemetry/telemetry_test.go`

**Step 1: Install dependency**

```bash
go get github.com/posthog/posthog-go@latest
```

**Step 2: Write failing test**

```go
// internal/telemetry/telemetry_test.go
package telemetry

import (
	"testing"
)

func TestTrackCommand(t *testing.T) {
	// Initialize with a no-op client for testing
	Init("test-key", "test-user", "test-org", false)
	defer Close()

	// Should not panic
	TrackCommand("keys.list", map[string]interface{}{
		"format":    "json",
		"exit_code": 0,
	})
}

func TestAnonymousMode(t *testing.T) {
	Init("test-key", "test-user", "test-org", true)
	defer Close()

	// In anonymous mode, distinct_id should not be user-based
	TrackCommand("endpoints.search", nil)
}
```

**Step 3: Write telemetry.go**

```go
// internal/telemetry/telemetry.go
package telemetry

import (
	"os"
	"runtime"
	"time"

	"github.com/posthog/posthog-go"
)

var (
	client    posthog.Client
	userID    string
	orgID     string
	anonymous bool
	version   string
)

const posthogAPIKey = "" // Set at build time via ldflags

func Init(ver string, user string, org string, anon bool) {
	version = ver
	userID = user
	orgID = org
	anonymous = anon

	apiKey := posthogAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("DWELLIR_POSTHOG_KEY")
	}
	if apiKey == "" {
		return // Telemetry disabled if no key
	}

	var err error
	client, err = posthog.NewWithConfig(apiKey, posthog.Config{
		BatchSize: 10,
		Interval:  30 * time.Second,
	})
	if err != nil {
		return // Silent fail
	}
}

func distinctID() string {
	if anonymous || userID == "" {
		// Use a stable device ID from config dir
		if id := os.Getenv("DWELLIR_DEVICE_ID"); id != "" {
			return "anon:" + id
		}
		return "anon:unknown"
	}
	return userID
}

func baseProperties() posthog.Properties {
	return posthog.NewProperties().
		Set("os", runtime.GOOS).
		Set("arch", runtime.GOARCH).
		Set("version", version).
		Set("org_id", orgID)
}

func TrackCommand(command string, extra map[string]interface{}) {
	if client == nil {
		return
	}
	props := baseProperties().Set("command", command)
	for k, v := range extra {
		props.Set(k, v)
	}
	client.Enqueue(posthog.Capture{
		DistinctId: distinctID(),
		Event:      "cli_command",
		Properties: props,
	})
}

func TrackInstall(method string) {
	if client == nil {
		return
	}
	client.Enqueue(posthog.Capture{
		DistinctId: distinctID(),
		Event:      "cli_installed",
		Properties: baseProperties().Set("install_method", method),
	})
}

func TrackAuth(method string, success bool) {
	if client == nil {
		return
	}
	client.Enqueue(posthog.Capture{
		DistinctId: distinctID(),
		Event:      "cli_auth",
		Properties: baseProperties().
			Set("method", method).
			Set("success", success),
	})
}

func TrackUpdate(fromVersion string, toVersion string) {
	if client == nil {
		return
	}
	client.Enqueue(posthog.Capture{
		DistinctId: distinctID(),
		Event:      "cli_updated",
		Properties: baseProperties().
			Set("from_version", fromVersion).
			Set("to_version", toVersion),
	})
}

func Close() {
	if client != nil {
		client.Close()
	}
}
```

**Step 4: Run tests**

```bash
go test ./internal/telemetry/ -v
```

**Step 5: Commit**

```bash
git add internal/telemetry/
git commit -m "feat: add PostHog telemetry with anonymous mode support"
```

---

## Task 12: Version and Self-Update Commands — [API-359](https://linear.app/dwellir/issue/API-359)

**Files:**
- Create: `internal/cli/version.go`
- Create: `internal/cli/update.go`

**Step 1: Install dependency**

```bash
go get github.com/creativeprojects/go-selfupdate@latest
```

**Step 2: Write version command**

```go
// internal/cli/version.go
package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Set at build time via ldflags
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version info",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := getFormatter()
		return f.Success("version", map[string]string{
			"version":    Version,
			"commit":     Commit,
			"build_date": BuildDate,
			"go_version": runtime.Version(),
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
		})
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.Version = fmt.Sprintf("%s (%s)", Version, Commit)
}
```

**Step 3: Write update command**

```go
// internal/cli/update.go
package cli

import (
	"fmt"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"
)

const repoSlug = "dwellir-public/cli"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update CLI to latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := getFormatter()

		latest, found, err := selfupdate.DetectLatest(cmd.Context(), selfupdate.ParseSlug(repoSlug))
		if err != nil {
			return f.Error("update_failed", fmt.Sprintf("Failed to check for updates: %v", err), "")
		}
		if !found {
			return f.Error("update_failed", "No release found.", "")
		}

		if latest.LessOrEqual(Version) {
			return f.Success("update", map[string]string{
				"status":  "up_to_date",
				"version": Version,
			})
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "Updating to v%s...\n", latest.Version())
		if err := selfupdate.UpdateTo(cmd.Context(), latest.AssetURL, latest.AssetName, ""); err != nil {
			return f.Error("update_failed", fmt.Sprintf("Update failed: %v", err), "Try downloading manually from GitHub releases.")
		}

		return f.Success("update", map[string]string{
			"status":       "updated",
			"from_version": Version,
			"to_version":   latest.Version(),
		})
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
```

**Step 4: Update Makefile build with ldflags**

Update the build target in Makefile:

```makefile
VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/dwellir-public/cli/internal/cli.Version=$(VERSION) \
           -X github.com/dwellir-public/cli/internal/cli.Commit=$(COMMIT) \
           -X github.com/dwellir-public/cli/internal/cli.BuildDate=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/dwellir ./cmd/dwellir
```

**Step 5: Build and verify**

```bash
make build
./bin/dwellir version --json
./bin/dwellir update --json  # Will fail gracefully (no releases yet)
```

**Step 6: Commit**

```bash
git add internal/cli/version.go internal/cli/update.go Makefile
git commit -m "feat: add version and self-update commands"
```

---

## Task 13: GoReleaser and Distribution — [API-360](https://linear.app/dwellir/issue/API-360)

**Files:**
- Create: `.goreleaser.yaml`
- Create: `scripts/install.sh`
- Create: `.github/workflows/release.yml`
- Create: `.github/workflows/ci.yml`

**Step 1: Write .goreleaser.yaml**

```yaml
version: 2

project_name: dwellir

before:
  hooks:
    - go mod tidy

builds:
  - main: ./cmd/dwellir
    binary: dwellir
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X github.com/dwellir-public/cli/internal/cli.Version={{.Version}}
      - -X github.com/dwellir-public/cli/internal/cli.Commit={{.ShortCommit}}
      - -X github.com/dwellir-public/cli/internal/cli.BuildDate={{.Date}}

archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"

brews:
  - repository:
      owner: dwellir-public
      name: homebrew-tap
    homepage: "https://dwellir.com"
    description: "Dwellir CLI — Blockchain RPC infrastructure from your terminal"
    license: "MIT"
    install: |
      bin.install "dwellir"
    test: |
      system "#{bin}/dwellir", "version"
```

**Step 2: Write install script**

```bash
#!/bin/sh
# scripts/install.sh — Dwellir CLI installer
set -e

REPO="dwellir-public/cli"
BINARY="dwellir"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
esac

LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')
if [ -z "$LATEST" ]; then
  echo "Failed to determine latest version." && exit 1
fi

URL="https://github.com/$REPO/releases/download/v${LATEST}/${BINARY}_${OS}_${ARCH}.tar.gz"
echo "Downloading dwellir v${LATEST} for ${OS}/${ARCH}..."

TMP=$(mktemp -d)
curl -fsSL "$URL" | tar -xz -C "$TMP"

INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
  echo "Installing to $INSTALL_DIR (requires sudo)..."
  sudo mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
else
  mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
fi
chmod +x "$INSTALL_DIR/$BINARY"
rm -rf "$TMP"

echo "dwellir v${LATEST} installed to $INSTALL_DIR/$BINARY"
dwellir version
```

**Step 3: Write CI workflow**

```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.24"
      - run: go test ./...
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.24"
      - run: make build
      - run: ./bin/dwellir version
```

**Step 4: Write release workflow**

```yaml
# .github/workflows/release.yml
name: Release
on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: "1.24"
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Step 5: Commit**

```bash
chmod +x scripts/install.sh
git add .goreleaser.yaml scripts/install.sh .github/
git commit -m "chore: add GoReleaser config, install script, and CI/CD workflows"
```

---

## Task 14: E2E Test Framework — [API-360](https://linear.app/dwellir/issue/API-360)

**Files:**
- Create: `test/e2e/helpers_test.go`
- Create: `test/e2e/version_test.go`
- Create: `test/e2e/config_test.go`

**Step 1: Write test helpers**

```go
// test/e2e/helpers_test.go
//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once before all tests
	dir, _ := os.Getwd()
	root := filepath.Join(dir, "..", "..")
	binaryPath = filepath.Join(root, "bin", "dwellir-test")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/dwellir")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("failed to build: " + string(out))
	}

	code := m.Run()
	os.Remove(binaryPath)
	os.Exit(code)
}

type cliResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)

	// Use isolated config dir per test
	configDir := t.TempDir()
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
```

Note: add `import "bytes"` to the imports.

**Step 2: Write version E2E test**

```go
// test/e2e/version_test.go
//go:build e2e

package e2e

import "testing"

func TestVersionJSON(t *testing.T) {
	result := runCLI(t, "version", "--json")
	if result.exitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", result.exitCode, result.stderr)
	}
	parsed := parseJSON(t, result.stdout)
	if parsed["ok"] != true {
		t.Errorf("expected ok: true, got: %v", parsed["ok"])
	}
	data := parsed["data"].(map[string]interface{})
	if _, ok := data["version"]; !ok {
		t.Error("expected 'version' in data")
	}
}

func TestVersionHuman(t *testing.T) {
	result := runCLI(t, "version", "--human")
	if result.exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.exitCode)
	}
	if result.stdout == "" {
		t.Error("expected non-empty output")
	}
}

func TestHelpOutput(t *testing.T) {
	result := runCLI(t, "--help")
	if result.exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.exitCode)
	}
	if result.stdout == "" {
		t.Error("expected help output")
	}
}
```

**Step 3: Write config E2E test**

```go
// test/e2e/config_test.go
//go:build e2e

package e2e

import "testing"

func TestConfigSetAndGet(t *testing.T) {
	// Set output to json
	result := runCLI(t, "config", "set", "output", "json")
	if result.exitCode != 0 {
		t.Fatalf("config set failed: %s", result.stderr)
	}

	// Get it back
	result = runCLI(t, "config", "get", "output", "--json")
	if result.exitCode != 0 {
		t.Fatalf("config get failed: %s", result.stderr)
	}
	parsed := parseJSON(t, result.stdout)
	if parsed["ok"] != true {
		t.Errorf("expected ok: true")
	}
}

func TestKeysListNoAuth(t *testing.T) {
	result := runCLI(t, "keys", "list", "--json")
	// Should fail with auth error
	if result.exitCode == 0 {
		t.Fatal("expected non-zero exit for unauthenticated request")
	}
}

func TestMissingArgs(t *testing.T) {
	result := runCLI(t, "keys", "enable")
	// Cobra should show usage
	if result.exitCode == 0 {
		t.Fatal("expected non-zero exit for missing required arg")
	}
}
```

**Step 4: Run E2E tests**

```bash
make test-e2e
```

**Step 5: Commit**

```bash
git add test/e2e/
git commit -m "test: add E2E test framework with version, config, and auth error tests"
```

---

## Dependency Order

```
Task 1 (scaffold) → Task 2 (output) → Task 3 (config) → Task 4 (API client)
    → Task 5 (auth) → Tasks 6-10 (commands, parallel) → Task 11 (telemetry)
    → Task 12 (version/update) → Task 13 (distribution) → Task 14 (E2E)
```

Tasks 6-10 (endpoints, keys, usage, logs, account) can be implemented in parallel once auth and API client are done.
