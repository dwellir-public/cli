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
