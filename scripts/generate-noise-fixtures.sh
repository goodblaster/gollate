#!/usr/bin/env bash
# Regenerates the committed noise fixtures.
#
# Each fixture derives from a clean, high-accuracy apple base case: only the
# OCR JSON text is corrupted (seeded, deterministic); canonical.txt and
# test-info.json are copied so geometry and scoring inputs stay ground truth.
# After regenerating, record default-config baselines with: make baselines
set -euo pipefail
cd "$(dirname "$0")/.."

SEED=20260706
BASES=(english-single spanish-single chinese-single)
RATES=(02 05 10)

go build -o bin/noise-inject ./cmd/noise-inject

for base in "${BASES[@]}"; do
  for rate in "${RATES[@]}"; do
    src="testdata/ocr-tests/${base}"
    dir="testdata/ocr-tests/${base}-noise${rate}"
    mkdir -p "$dir"
    cp "$src/canonical.txt" "$src/test-info.json" "$dir/"
    bin/noise-inject -in "$src/apple-ocr.json" -out "$dir/apple-ocr.json" \
      -rate "0.${rate}" -seed "$SEED"
  done
done
