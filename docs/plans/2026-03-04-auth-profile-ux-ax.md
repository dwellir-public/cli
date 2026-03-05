# Auth, Profiles, and Doctor UX/AX Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make authentication/profile behavior deterministic across directories and improve observability for both humans and coding agents.

**Architecture:** Unify profile resolution behind a single shared helper used by `auth login`, `auth logout`, token resolution, and diagnostics. Add explicit profile-inspection commands (`profiles`) plus a high-signal `doctor` command that reports profile source, auth state, output mode source, and agent-detection inputs. Keep output deterministic with explicit precedence and agent-friendly diagnostics.

**Tech Stack:** Go 1.24+, Cobra, existing formatter system (`human` / `json` / `toon`), unit tests + e2e tests.

---

## Design Decisions

1. **Login writes to resolved profile by default**
- `dwellir auth login` without `--profile` should write to the same effective profile that runtime commands use (`--profile`, `DWELLIR_PROFILE`, nearest `.dwellir.json`, `default_profile`, fallback `default`).
- Rationale: eliminates current split-brain behavior where login writes `default` while runtime resolves `bench`/`work`.

2. **Per-directory profile remains first-class**
- Keep `.dwellir.json` as project-local override.
- Add explicit commands to view and modify local profile bindings.

3. **Diagnostics first for agent workflows**
- Add `dwellir doctor` for one-command diagnosis.
- Add `dwellir profiles` commands so agents can verify profile context before mutating actions.

4. **Output behavior stays explicit-over-implicit**
- Flags > config > auto-detection.
- Auto-detection should be inspectable via doctor output (exact markers and tty state), so users and agents can understand why a mode was selected.

---

### Task 1: Unify Effective Profile Resolution

**Files:**
- Create: `internal/cli/profile_context.go`
- Modify: `internal/cli/auth.go`
- Modify: `internal/cli/endpoints.go`
- Modify: `internal/auth/resolve.go`
- Test: `internal/cli/profile_context_test.go`
- Test: `internal/auth/resolve_test.go`

**Step 1: Add shared resolver in CLI layer**
- Implement helper returning:
  - `profileName string`
  - `source string` (`flag`, `env`, `dwellir_json`, `config_default`, `fallback_default`)
  - `cwd string`
- Source must be deterministic and re-usable by login/logout/doctor/profiles.

**Step 2: Make `auth login` and `auth logout` use shared resolver**
- In `internal/cli/auth.go`, replace `profile == "" ? "default"` branches.
- When no explicit `--profile`, login/logout should target resolved effective profile.

**Step 3: Improve auth resolution errors**
- In `internal/auth/resolve.go`, when profile load fails, include resolved profile name in error/help.
- Example shape: `profile 'bench' not found; run 'dwellir auth login --profile bench'`.

**Step 4: Add unit tests for precedence + source**
- Cases:
  - `--profile` wins over everything.
  - `DWELLIR_PROFILE` wins over `.dwellir.json`.
  - `.dwellir.json` wins over `default_profile`.
  - fallback to `default`.

**Step 5: Add regression test for reported bug**
- Reproduce: config default profile `bench`, run login without flag, ensure runtime token resolution succeeds afterward.

**Step 6: Commit**
```bash
git add internal/cli/profile_context.go internal/cli/auth.go internal/auth/resolve.go internal/cli/profile_context_test.go internal/auth/resolve_test.go
git commit -m "fix: unify effective profile resolution across auth login and runtime"
```

---

### Task 2: Harden Browser Login Callback Validation

**Files:**
- Modify: `internal/auth/login.go`
- Test: `internal/auth/login_test.go`

**Step 1: Validate callback payload before saving profile**
- Reject empty token payload.
- Return clear `auth_failed` message if callback lacks token.

**Step 2: Add tests for callback handler**
- `POST /callback` with missing token -> non-200, no success path.
- Valid payload -> success.

**Step 3: Commit**
```bash
git add internal/auth/login.go internal/auth/login_test.go
git commit -m "fix: reject browser auth callbacks without token"
```

---

### Task 3: Add `dwellir profiles` Command Group

**Files:**
- Create: `internal/cli/profiles.go`
- Modify: `internal/cli/root.go` (register command)
- Modify: `internal/config/profiles.go` (if helper methods are needed)
- Modify: `internal/output/human.go`
- Test: `internal/cli/profiles_test.go`
- Test: `test/e2e/profiles_test.go`
- Docs: `README.md`

**Step 1: Implement `profiles list`**
- Show all profiles with:
  - name
  - token present (`yes/no`)
  - user/org (if present)
  - active (`yes/no`)
  - active source (for active profile)

**Step 2: Implement `profiles current`**
- Show resolved active profile and why it was selected (source chain).

**Step 3: Implement `profiles bind` / `profiles unbind`**
- `profiles bind <name>` writes `.dwellir.json` in current directory.
- `profiles unbind` removes local `.dwellir.json` binding.
- Keep non-interactive semantics.

**Step 4: Add formatter wiring**
- Human output: clean key/value for `current`; table for `list`.
- JSON/TOON: regular envelope, stable keys.

**Step 5: Add tests**
- Unit + e2e for list/current/bind/unbind.
- Ensure commands work in non-TTY environments (agent execution).

**Step 6: Commit**
```bash
git add internal/cli/profiles.go internal/cli/root.go internal/output/human.go internal/cli/profiles_test.go test/e2e/profiles_test.go README.md
# add any config helper edits if present
git commit -m "feat: add profile introspection and directory binding commands"
```

---

### Task 4: Add `dwellir doctor` for Auth/Profile/Output Diagnostics

**Files:**
- Create: `internal/cli/doctor.go`
- Modify: `internal/cli/root.go` (register command)
- Modify: `internal/output/human.go`
- Test: `internal/cli/doctor_test.go`
- Test: `test/e2e/doctor_test.go`
- Docs: `README.md`

**Step 1: Implement `doctor` checks (offline by default)**
- Report:
  - config dir path
  - output mode + source (flag/config/auto/default)
  - tty state
  - agent markers present
  - resolved profile + source
  - profile file path exists?
  - token present?
- Emit `status: ok|warn|error` per check.

**Step 2: Add `--verify-api` option**
- Optional network check calling lightweight authenticated endpoint (e.g., account info).
- Default remains offline for reliability and speed.

**Step 3: Add formatter wiring + tests**
- Human: readable grouped sections/table.
- JSON/TOON: structured `checks[]` array for agents.

**Step 4: Commit**
```bash
git add internal/cli/doctor.go internal/cli/root.go internal/output/human.go internal/cli/doctor_test.go test/e2e/doctor_test.go README.md
git commit -m "feat: add doctor command for auth profile and output diagnostics"
```

---

### Task 5: Refine Agent Output Detection and Explainability

**Files:**
- Modify: `internal/cli/root.go`
- Test: `internal/cli/root_test.go`
- Test: `test/e2e/config_test.go`
- Docs: `README.md`

**Step 1: Keep precedence rules explicit and testable**
- Confirm priority remains:
  - explicit flags
  - config output
  - auto-selection (if no explicit config)
  - hard default human

**Step 2: Reduce false-positive confusion**
- Add helper that returns `auto_reason` metadata (`agent+non_tty`, `none`, etc.) for doctor.
- Optionally add explicit env override for wrappers (e.g., `DWELLIR_AGENT=1`) and test it.

**Step 3: Add regression tests**
- Non-agent non-tty without config -> human.
- Agent markers + non-tty without config -> toon.
- Config output human overrides auto.

**Step 4: Commit**
```bash
git add internal/cli/root.go internal/cli/root_test.go test/e2e/config_test.go README.md
git commit -m "fix: make output auto-selection explainable and regression-tested"
```

---

### Task 6: End-to-End Verification and Docs Cleanup

**Files:**
- Modify: `README.md`
- Modify: `AGENTS.md` (if command list/examples are documented there)
- Add: `docs/plans/2026-03-04-auth-profile-ux-ax.md` (this plan; keep updated as implementation reality changes)

**Step 1: Run full verification**
```bash
goimports -w .
golangci-lint run
go test ./...
go test ./test/e2e/ -tags=e2e -v
```

**Step 2: Manual scenario checks**
- Login from `~` with `default_profile=default` and verify `account info`.
- Project with `.dwellir.json` bound to `work` and verify resolution.
- Run `dwellir doctor` in both contexts and validate source attribution.
- Run commands in non-TTY/agent-like environment and verify output mode behavior + doctor explanation.

**Step 3: Final commit**
```bash
git add README.md AGENTS.md docs/plans/2026-03-04-auth-profile-ux-ax.md
git commit -m "docs: document profile workflow doctor and agent diagnostics"
```

---

## Rollout Recommendation

1. Land **Task 1 + Task 2** first as a patch release (bugfix severity).
2. Land **Task 3 + Task 4** next as minor release (new UX/AX surfaces).
3. Land **Task 5** with cautious telemetry/bench checks to avoid regressing agent ergonomics.

## Acceptance Criteria

- `auth login` and `account info` use the same effective profile without surprises.
- Missing/empty token callbacks cannot report successful auth.
- Users/agents can answer “which profile am I using and why?” with `profiles current` and `doctor`.
- Users/agents can list available profiles and local bindings quickly.
- Output mode decisions are diagnosable and reproducible.
