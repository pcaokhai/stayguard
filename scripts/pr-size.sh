#!/usr/bin/env bash
# PR size per docs/10 §143: changed lines excluding generated files (linguist-generated).
# Usage: scripts/pr-size.sh [base-ref]   (default origin/main). Exit 1 above LIMIT.
set -u
cd "$(dirname "$0")/.."
base=${1:-origin/main}
LIMIT=400
total=0; skipped=0
while IFS=$'\t' read -r add del file; do
  [ "$add" = "-" ] && continue # binary
  if [ "$(git check-attr linguist-generated -- "$file" | awk '{print $NF}')" = "true" ]; then
    skipped=$((skipped + add + del))
  else
    total=$((total + add + del))
  fi
done < <(git diff --numstat "$base...HEAD")
echo "changed lines: $total (generated, excluded: $skipped, limit $LIMIT)"
[ "$total" -le "$LIMIT" ]
