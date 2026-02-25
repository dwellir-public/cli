# Dwellir CLI — Design Document

**Date:** 2026-02-25
**Status:** Approved
**Author:** Elias + Claude

## Goal

Build a Go CLI (`dwellir`) that gives coding agents and humans full access to the Dwellir dashboard functionality: endpoint discovery, API key management, usage analytics, error logs, and account info. Primary audience is coding agents (Claude Code, Codex, Cursor) operating non-interactively. Secondary audience is humans.

## Architecture

The CLI is a standalone Go binary that talks to the Marly API over HTTPS. It uses a dedicated CLI token system (separate from the dashboard session JWT) with per-directory profile binding for multi-account support. Output defaults to human-readable, configurable to JSON globally via `dwellir config set output json`. Telemetry via PostHog.

```
┌─────────────┐     ┌─────────────┐
│  dwellir    │────▶│    Marly    │
│  (Go CLI)   │     │  (FastAPI)  │
└─────────────┘     └─────────────┘
       │
       ▼
~/.config/dwellir/   (tokens, config, profiles)
```

## Tech Stack

- **Language:** Go 1.24+
- **CLI framework:** Cobra + Viper
- **HTTP client:** net/http (stdlib)
- **Auto-update:** creativeprojects/go-selfupdate
- **Telemetry:** posthog/posthog-go
- **Release:** GoReleaser + GitHub Actions
- **Distribution:** curl install, Homebrew, AUR, npm wrapper, `go install`

## Command Structure

```
dwellir
├── auth
│   ├── login              # Browser-based OAuth flow
│   ├── logout             # Clear local credentials
│   ├── status             # Show current auth state
│   └── token              # Print current access token
│
├── endpoints
│   ├── list               # List all chains/networks/endpoints
│   ├── search <query>     # Search by chain name, network, protocol
│   └── get <chain>        # Get endpoints for a specific chain
│
├── keys
│   ├── list               # List all API keys
│   ├── create             # Create new API key
│   ├── update <key>       # Update key (name, quotas, enabled)
│   ├── delete <key>       # Delete API key
│   ├── enable <key>       # Enable a key
│   └── disable <key>      # Disable a key
│
├── usage
│   ├── summary            # Current billing cycle summary
│   ├── history            # Usage over time
│   └── rps                # Current RPS metrics
│
├── logs
│   ├── errors             # List error logs (with filters)
│   ├── stats              # Error metrics/classifications
│   └── facets             # Error facet aggregations
│
├── account
│   ├── info               # Org info, plan, billing status
│   └── subscription       # Current subscription details
│
├── config
│   ├── set <key> <value>  # Set config
│   ├── get <key>          # Get config value
│   └── list               # Show all config
│
├── update                 # Self-update to latest version
└── version                # Print version info
```

### Global Flags

- `--json` — JSON output (overrides config)
- `--human` — Human-readable output (overrides config)
- `--profile <name>` — Use specific auth profile
- `-q, --quiet` — Suppress non-essential output
- `--anon-telemetry` — Anonymize telemetry (strip user/org identity)

### Endpoint Filters

- `--chain` — Filter by chain name
- `--network` — Filter by network (mainnet, testnet)
- `--protocol` — Filter by protocol (https, wss)
- `--node-type` — Filter by node type (full, archive)
- `--ecosystem` — Filter by ecosystem (evm, substrate, cosmos, solana)

### Log Filters

- `--key` — Filter by API key
- `--endpoint` — Filter by FQDN
- `--status-code` — Filter by HTTP status code
- `--rpc-method` — Filter by RPC method
- `--from, --to` — Time range
- `--limit, --cursor` — Pagination

## Auth System

### CLI Token (new, separate from session JWT)

CLI tokens are a dedicated token type stored in a new marly DB table. Independent lifecycle from dashboard sessions — user logs out of dashboard, CLI keeps working.

```
CLI Token:
  - id: UUID
  - user_id: UUID
  - organization_id: integer
  - name: string
  - token_hash: string (bcrypt)
  - scope: enum (read, read-write, admin)
  - last_used_at: datetime
  - expires_at: datetime (90 days, auto-refreshed on use)
  - created_at: datetime
  - revoked_at: datetime (nullable)
```

### Browser Auth Flow

```
dwellir auth login [--profile <name>]

1. Generate session UUID
2. Start local HTTP server on random port
3. Open browser to dashboard.dwellir.com/cli-auth?session=<UUID>&port=<PORT>
4. Print to stderr: "Opening browser... waiting for auth"
5. Timeout after 5 min → print error with link to /agents for manual token
```

### /cli-auth Page (unauthenticated)

If no session: landing page with info cards + two CTAs:
- "I have an account" → loads Outseta login embed
- "I don't have an account" → loads Outseta register embed

After auth → scope selection → token created → POST to localhost callback → CLI stores token.

### Token Resolution Order

```
1. DWELLIR_TOKEN env var           (highest, for CI)
2. --token flag                     (explicit override)
3. --profile flag                   (named profile)
4. .dwellir.json in current dir     (per-repo binding)
5. Walk up parent dirs              (per-workspace binding)
6. config.json default profile      (global default)
7. profiles/default.json            (fallback)
```

### Per-Directory Config

`.dwellir.json` in repo root:
```json
{
  "profile": "work"
}
```

### Token Storage

```
~/.config/dwellir/
├── config.json
├── profiles/
│   ├── default.json
│   ├── work.json
│   └── personal.json
```

### Token Auto-Refresh

On each request, if token expires within 7 days, marly returns refreshed token in `X-Dwellir-Refreshed-Token` response header. CLI silently writes to disk.

## Output Format

### Default: human-readable. Configurable:

```
dwellir config set output json
```

### JSON Envelope

```json
{
  "ok": true,
  "data": { ... },
  "meta": {
    "command": "keys.list",
    "timestamp": "2026-02-25T12:00:00Z",
    "profile": "default"
  }
}
```

### Error Envelope

```json
{
  "ok": false,
  "error": {
    "code": "not_authenticated",
    "message": "No authentication token found.",
    "help": "Run 'dwellir auth login' or visit https://dashboard.dwellir.com/agents"
  }
}
```

### Exit Codes

- 0: success
- 1: general error
- 2: auth error
- 3: not found
- 4: validation error

## Telemetry (PostHog)

**SDK:** posthog/posthog-go

**Events:**
- `cli_installed` — first run (OS, arch, install method, version)
- `cli_command` — every invocation (command, flags, output format, duration_ms, exit_code)
- `cli_updated` — self-update (from/to version)
- `cli_auth` — auth events (method, success/failure)

**Identity:** org_id + user_id from token. With `--anon-telemetry`: random device UUID only.

**Behavior:** fire-and-forget, async flush on exit, silent fail on network issues, respects DO_NOT_TRACK env var (anonymizes but still tracks).

## Distribution

| Method | Command |
|--------|---------|
| curl | `curl -fsSL https://dwellir.com/install.sh \| sh` |
| Homebrew | `brew install dwellir-public/tap/dwellir` |
| AUR | `yay -S dwellir-cli` |
| npm | `npm install -g @dwellir/cli` |
| Go | `go install github.com/dwellir-public/cli/cmd/dwellir@latest` |
| Binary | GitHub releases |

### Self-Update

`dwellir update` via creativeprojects/go-selfupdate. Checks GitHub releases. On every command, if version >30 days old, stderr notice: `Notice: dwellir vX.Y.Z available. Run 'dwellir update'`

## /agents Dashboard Page

Future AX hub at dashboard.dwellir.com/agents. For CLI launch:
- List active CLI tokens (name, scope, last used, created)
- Create new token manually (for headless/CI)
- Revoke tokens
- Edit token name/scope
- Link from API Keys page

## Marly Work Required

| What | Details |
|------|---------|
| CLI token table + migrations | New `cli_tokens` table in auth schema |
| CRUD endpoints | POST/GET/DELETE/PATCH `/v4/auth/cli-token(s)` |
| Token auth middleware | Validate Bearer token, check scope, track last_used |
| Token auto-refresh | Return refreshed token in X-Dwellir-Refreshed-Token header |
| Ecosystem labels on chains | Surface ecosystem field from brian chain data |

## Brian Work Required

| What | Details |
|------|---------|
| Ecosystem labels | Ensure all chains in CSV have ecosystem labels (evm, substrate, cosmos, solana, etc.) |

## Frontend Work Required

| What | Details |
|------|---------|
| `/cli-auth` page | Landing page with Outseta embeds, scope selector, localhost callback |
| `/agents` page | Token management UI |
| Link from API Keys | "Looking for CLI tokens? → /agents" |

## Repo Structure

```
cli/
├── cmd/dwellir/main.go
├── internal/
│   ├── cli/          # Cobra commands
│   ├── api/          # Marly API client
│   ├── config/       # Config + profiles
│   ├── output/       # JSON + human formatters
│   ├── auth/         # Browser flow + token resolution
│   └── telemetry/    # PostHog
├── test/e2e/         # E2E tests
├── .goreleaser.yaml
├── .golangci.yml
├── Makefile
├── AGENTS.md         # Agent instructions (source of truth)
├── CLAUDE.md         # Symlink → AGENTS.md
└── README.md
```
