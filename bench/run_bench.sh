#!/usr/bin/env bash
# run_bench.sh — benchmark strainer against grep, ripgrep, and awk
#
# Requirements:
#   - hyperfine  (https://github.com/sharkdp/hyperfine)
#   - rg         (ripgrep)
#   - grep       (with -P / PCRE support; GNU grep or grep-pcre)
#   - awk
#   - strainer   built and in PATH (or pass STRAINER=./strainer)
#
# Usage:
#   cd bench
#   go run gen_bench.go [-size 500]   # generates bench.txt (~500 MB)
#   bash run_bench.sh

set -euo pipefail

STRAINER="${STRAINER:-$(which strainer 2>/dev/null || echo ../strainer)}"
BENCH_FILE="${BENCH_FILE:-bench.txt}"
OUTPUT="${OUTPUT:-/dev/null}"

if [[ ! -f "$BENCH_FILE" ]]; then
  echo "bench.txt not found — run: go run gen_bench.go" >&2
  exit 1
fi

SIZE_MB=$(du -m "$BENCH_FILE" | cut -f1)
echo "Benchmark file: $BENCH_FILE (${SIZE_MB} MB)"
echo "Filter: length 8–12 AND printable ASCII"
echo ""

# The equivalent regex-based filter for grep/rg/awk
REGEX='^[\x20-\x7E]{8,12}$'

if command -v hyperfine &>/dev/null; then
  hyperfine \
    --warmup 3 \
    --runs 10 \
    --export-markdown results.md \
    "$STRAINER --input $BENCH_FILE --min 8 --max 12 --ascii --output $OUTPUT --quiet" \
    "rg '$REGEX' $BENCH_FILE > $OUTPUT" \
    "grep -P '$REGEX' $BENCH_FILE > $OUTPUT" \
    "awk 'length>=8 && length<=12 && /^[[:print:]]+\$/' $BENCH_FILE > $OUTPUT"
  echo ""
  cat results.md
else
  echo "hyperfine not found — falling back to 'time' (single run each)"
  echo ""

  for label_cmd in \
      "strainer|$STRAINER --input $BENCH_FILE --min 8 --max 12 --ascii --output $OUTPUT --quiet" \
      "ripgrep|rg '$REGEX' $BENCH_FILE > $OUTPUT" \
      "grep|grep -P '$REGEX' $BENCH_FILE > $OUTPUT" \
      "awk|awk 'length>=8 && length<=12 && /^[[:print:]]+\$/' $BENCH_FILE > $OUTPUT"
  do
    label="${label_cmd%%|*}"
    cmd="${label_cmd#*|}"
    echo -n "$label: "
    { time eval "$cmd" ; } 2>&1 | grep real | awk '{print $2}'
  done
fi
