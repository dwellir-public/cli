# Agent Output Benchmark (2026-03-03)

This benchmark compared `json` vs `toon` as the default output mode for auto-detected agent environments.

## Setup

- Agents:
  - Codex CLI (`gpt-5.3-codex`)
  - Claude Code CLI (`opus`)
- Runs: 2 full runs, 4 variants each (`codex/claude x json/toon`) = 8 total agent runs.
- Task: run `dwellir` commands, extract values, and generate a `report.json` from command output only.
- Commands executed by agents:
  - `dwellir version`
  - `dwellir config get output`
  - `dwellir keys create`
  - `dwellir endpoints list --network invalid`
  - `dwellir bogus`

## Aggregate Results

| Metric | JSON | TOON |
|---|---:|---:|
| Variant runs | 4 | 4 |
| Task pass rate | 4/4 | 4/4 |
| `jq` attempts on `dwellir` output | 2 | 0 |
| Parse-error incidents | 0 | 0 |
| Mean tool calls | 6.25 | 4.75 |
| Mean output tokens | 2045.75 | 1298.50 |

Observed in this run set:

- Equal correctness across both modes.
- Lower interaction overhead in TOON mode:
  - ~24% fewer tool calls.
  - ~36.5% lower output tokens.
- Natural `jq` usage appeared only in JSON mode (Codex), and not in TOON mode.

## Interpretation

These results support TOON as a better default for LLM-agent ergonomics in this CLI.

JSON remains important for strict machine parsing pipelines, especially where scripts are `jq`-centric.

## Caveats

- Sample size is small (8 total runs).
- Commands were chosen to be offline-safe and deterministic.
- This benchmark focuses on agent ergonomics, not on every production pipeline shape.

## Related TOON Guidance

- When not to use TOON:
  <https://github.com/toon-format/toon/tree/main/packages/toon#when-not-to-use-toon>
- Upstream TOON benchmark section:
  <https://github.com/toon-format/toon/tree/main/packages/toon#benchmarks>
