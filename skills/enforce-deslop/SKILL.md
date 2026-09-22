---
name: enforce-deslop
description: Enforce anti-slop quality gates (complexity, file size, dead code, no any) using the repo's own linter and CI. Use when the user asks to deslop, enforce deslop, clean AI slop, reduce complexity, split long files, ban any, or work on chore/enforce-deslop / chore/complexity-gate PRs. Also use when adding code that would fail those gates.
---

# Enforce deslop

Read this repo's `AGENTS.md` deslop section first. Project config and scripts win.

## Gates we enforce

- Cyclomatic complexity: max 15 (McCabe). Tool: project linter, not eyeballing.
- Cognitive complexity: max 15 where the linter has it (Biome, revive). Do not mix CC and cognitive in one before/after pair.
- Lines per production file: max 500. Tests excluded unless AGENTS.md says otherwise.
- Dead code: unused imports/vars/symbols via the project linter or knip.
- TypeScript `any`: 0. Prefer real types. `unknown` is allowed.

## Gates we do not enforce

- `unknown`: 0. `unknown` is the safe top type. Do not ban it.
- Test coverage 100%. That produces weak tests. Prefer tests on behavior.
- CRAP and mutation testing. Need coverage/mutation infra that these repos do not run in CI yet.
- Halstead difficulty. No first-class rule in Biome/ESLint/Ruff/revive. Noisy on modern code.

## Workflow

1. Run the repo deslop commands from AGENTS.md. Report numbers before edits.
2. Fix worst first. One function or one file at a time.
3. Preserve behavior. Run the repo tests.
4. Do not game metrics (dense one-liners, `any` as `Record<string, never>`, splitting files without names).
5. End with a before/after table.

## Output

```
## Deslop report
| Check | Before | After |
| complexity (fn) | 28 | 6 |
| file lines | 1200 | 410 |
```
