#!/usr/bin/env bash
# Experiment matrix runner.
#
# Runs the integration suite once with default config, then once per given
# flag combination, and prints a per-case accuracy delta table. Experiment
# runs are not gated by baselines; the default run is, so a broken flag-off
# path fails loudly here.
#
# Usage:
#   ./scripts/run-experiments.sh \
#     "EnableApproxChainFallback=true" \
#     "EnableChainHoles=true,HolePathPenalty=1.5" \
#     "EnableApproxChainFallback=true,EnableChainHoles=true"
#
# Results land in experiments/<timestamp>/ (gitignored); paste the table
# wherever you track experiment results.
set -euo pipefail
cd "$(dirname "$0")/.."

if [ $# -eq 0 ]; then
  echo "usage: $0 \"Flag=value[,Flag=value...]\" [...]" >&2
  exit 2
fi

outdir="experiments/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$outdir"

echo "== baseline (default config, ratchet-gated) =="
RESULTS_JSON="$PWD/$outdir/baseline.json" \
  go test ./integration -run TestOCRSorting -count=1 | tail -1

i=0
for flags in "$@"; do
  i=$((i + 1))
  echo "== V$i: $flags =="
  EXPERIMENT_FLAGS="$flags" RESULTS_JSON="$PWD/$outdir/v$i.json" \
    go test ./integration -run TestOCRSorting -count=1 | tail -1
done

echo
variants=()
for j in $(seq 1 $i); do variants+=("$outdir/v$j.json"); done
go run ./cmd/exp-report "$outdir/baseline.json" "${variants[@]}" | tee "$outdir/report.txt"
echo
echo "results: $outdir/"
