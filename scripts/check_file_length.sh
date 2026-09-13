#!/usr/bin/env bash
# Fail if a production source file has 500 or more lines.
set -euo pipefail

MAX="${MAX_FILE_LINES:-500}"
ROOT="${1:-.}"
# Space-separated extensions without dots.
EXTS="${FILE_LENGTH_EXTS:-py}"
# Colon-separated path fragments to skip.
SKIP="${FILE_LENGTH_SKIP:-/tests/:/alembic/:/__pycache__/}"

name_args=()
for ext in $EXTS; do
  if [[ ${#name_args[@]} -gt 0 ]]; then
    name_args+=(-o)
  fi
  name_args+=(-name "*.${ext}")
done

fail=0
while IFS= read -r -d '' path; do
  skip=0
  IFS=':' read -ra parts <<<"$SKIP"
  for part in "${parts[@]}"; do
    if [[ -n "$part" && "$path" == *"$part"* ]]; then
      skip=1
      break
    fi
  done
  if [[ "$skip" -eq 1 ]]; then
    continue
  fi
  lines=$(wc -l <"$path")
  if [[ "$lines" -ge "$MAX" ]]; then
    printf '%6d  %s\n' "$lines" "$path"
    fail=1
  fi
done < <(
  find "$ROOT" \
    \( -name .git -o -name node_modules -o -name dist -o -name coverage -o -name __pycache__ -o -name alembic \) -prune -o \
    -type f \( "${name_args[@]}" \) -print0
)

if [[ "$fail" -eq 1 ]]; then
  echo "files at or over ${MAX} lines"
  exit 1
fi
echo "ok: no production files at or over ${MAX} lines"
