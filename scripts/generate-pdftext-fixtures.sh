#!/usr/bin/env bash
# Regenerate the committed PDF text-layer fixtures (pdftext-ocr.json) for
# the OCR regression suite.
#
# For every testdata/ocr-tests case with a document.pdf, extracts the text
# layer with pdftext-util (auto backend, language from test-info.json) and
# saves it as pdftext-ocr.json in the engine-neutral blocks format. The
# integration suite scores these via the "blocks" engine under the
# "case/pdftext" baseline keys.
#
# Only needed when fixture documents change (regenerated via
# scripts/generate-ocr-tests.sh) or pdftext extraction behavior changes.
# Requirements: macOS (PDFKit backend) AND poppler (`brew install
# poppler`) so auto selection can pick the right backend per script —
# the suite itself needs neither, it reads the committed JSON.
set -euo pipefail

cd "$(dirname "$0")/.."

if ! command -v pdftotext >/dev/null; then
  echo "error: pdftotext not found; install poppler (brew install poppler)" >&2
  echo "       (Hindi fixtures must be generated with the poppler backend)" >&2
  exit 1
fi

make -s build >/dev/null 2>&1 || true
GOWORK=off go build -o bin/pdftext-util ./cmd/pdftext-util

count=0
for dir in testdata/ocr-tests/*/; do
  pdf="$dir/document.pdf"
  [ -f "$pdf" ] || continue

  lang=$(python3 -c "
import json, sys
try:
    print(json.load(open('$dir/test-info.json'))['language'])
except Exception:
    print('$(basename "$dir")'.split('-')[0])
")

  bin/pdftext-util -lang "$lang" "$pdf" >/dev/null
  mv "${pdf%.pdf}-pdftext.json" "$dir/pdftext-ocr.json"
  count=$((count + 1))
  echo "$(basename "$dir"): pdftext-ocr.json (lang $lang)"
done

echo "regenerated $count pdftext fixtures"
