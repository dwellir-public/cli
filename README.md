# Dwellir CLI

Dwellir CLI (`dwellir`) gives you terminal access to core Dwellir dashboard workflows:

- authenticate and manage profiles
- discover blockchain endpoints
- create and manage API keys
- inspect usage analytics and error logs
- view account and subscription details

It is designed for both humans and agents, with consistent `--json` output on every command.

## Install

### Option 1: Install latest release (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/dwellir-public/cli/main/scripts/install.sh | sh
```

Note: this requires published GitHub releases.

### Option 2: Build from source

```bash
git clone git@github.com:dwellir-public/cli.git
cd cli
make build
./bin/dwellir --help
```

### Option 3: Go install

```bash
go install github.com/dwellir-public/cli/cmd/dwellir@latest
```

## Quickstart

### 1) Authenticate

Browser flow:

```bash
dwellir auth login
```

Headless/CI flow:

```bash
dwellir auth login --token <CLI_TOKEN>
```

You can also set a token directly:

```bash
export DWELLIR_TOKEN=<CLI_TOKEN>
```

### 2) Explore endpoints

```bash
dwellir endpoints list
dwellir endpoints search ethereum --protocol https
dwellir endpoints get base
```

### 3) Search docs from the terminal

```bash
dwellir docs search authentication
dwellir docs get getting-started
dwellir docs get https://www.dwellir.com/docs/hyperliquid/historical-data
```

### 4) Manage keys

```bash
dwellir keys list
dwellir keys create --name ci-key --daily-quota 100000
dwellir keys disable <key-id>
```

### 5) Check usage and logs

```bash
dwellir usage summary
dwellir usage history --interval day
dwellir logs errors --status-code 429 --limit 100
```

## Command Overview

Top-level commands:

- `dwellir auth` — login/logout/status/token
- `dwellir docs` — list/search/get public docs pages as markdown
- `dwellir endpoints` — list/search/get chains and networks
- `dwellir keys` — list/create/update/delete/enable/disable API keys
- `dwellir usage` — summary/history/rps analytics
- `dwellir logs` — errors/stats/facets with filters
- `dwellir account` — info/subscription
- `dwellir config` — set/get/list CLI config
- `dwellir version` — build/version metadata
- `dwellir update` — self-update from GitHub releases

For full command help:

```bash
dwellir --help
dwellir <command> --help
```

## Output Modes

Every command supports:

- `--human` (default): readable output
- `--json`: structured machine-readable output

Example:

```bash
dwellir keys list --json
```

JSON responses use a common envelope:

```json
{
  "ok": true,
  "data": {},
  "meta": {
    "command": "keys.list",
    "timestamp": "..."
  }
}
```

Errors return `ok: false` and a non-zero exit code.

## Profiles and Config

Config is stored in:

- `~/.config/dwellir/config.json`
- `~/.config/dwellir/profiles/<name>.json`

Per-project profile binding is supported via `.dwellir.json`:

```json
{ "profile": "work" }
```

### Config commands

```bash
dwellir config list
dwellir config set output json
dwellir config set default_profile work
dwellir config get output
```

## Resolution Rules

### Token resolution order

1. `DWELLIR_TOKEN`
2. `--token` flag
3. Resolved profile token (from profile resolution below)

### Profile resolution order

1. `--profile`
2. `DWELLIR_PROFILE`
3. nearest `.dwellir.json`
4. `default_profile` in config
5. `default`

## Environment Variables

- `DWELLIR_TOKEN` — explicit auth token override
- `DWELLIR_PROFILE` — default profile override
- `DWELLIR_CONFIG_DIR` — custom config directory
- `DWELLIR_API_URL` — override API base URL (default: `https://dashboard.dwellir.com/marly-api`)
- `DWELLIR_DASHBOARD_URL` — override dashboard URL for browser auth
- `DWELLIR_DOCS_BASE_URL` — override docs base URL (default: `https://www.dwellir.com/docs`)
- `DWELLIR_DOCS_INDEX_URL` — override docs index URL (default: `<docs-base>/llms.txt`)

## Development

```bash
make build
make test
make test-e2e
make lint
make check
```

Core paths:

- `cmd/dwellir/` — entrypoint
- `internal/cli/` — Cobra commands
- `internal/api/` — API client and domain APIs
- `internal/auth/` — token resolution and login flow
- `internal/config/` — config/profile management
- `internal/output/` — formatter and envelope
- `internal/telemetry/` — PostHog integration
- `test/e2e/` — end-to-end tests

## Releases

- Version source of truth: `VERSION` (SemVer, e.g. `0.1.0`).
- Every PR merged to `main` must bump `VERSION`.
- A merge to `main` automatically:
  1. creates/pushes `v<VERSION>` tag
  2. runs GoReleaser on that tag
  3. publishes GitHub release artifacts
