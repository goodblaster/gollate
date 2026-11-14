#!/usr/bin/env bash
# Generate the multi-page fixtures under testdata/multipage (slow; run when
# content or the "pages" layout changes). macOS: Apple Vision, CoreGraphics
# rasterization; also needs tesseract and poppler (pdftotext).
#
# For each entry this writes, in testdata/multipage/<name>/:
#   document.pdf, document.html, canonical.txt   (cmd/testdoc -layout pages)
#   page-N.png                                   2x raster of each page
#   apple-N.json, tesseract-N.json               OCR of each page
#   pdftext-N.json                               text layer of each page
#   info.json                                    language + page pixel sizes
#
# The suite (go test ./integration -run TestMultipage) reads only the
# committed JSON and canonical.txt.
set -euo pipefail
cd "$(dirname "$0")/.."

make -s build utils >/dev/null
CONTENT=testdata/ocr-tests/content
OUT=testdata/multipage

# name | language | content file | title | extra testdoc flags
ENTRIES=(
  "english-book|english|$CONTENT/english-book.txt|The Keeper's Ledgers|-font-size 11 -line-spacing 1.4"
)

for entry in "${ENTRIES[@]}"; do
  IFS='|' read -r name lang content title extra <<< "$entry"
  dir="$OUT/$name"
  rm -rf "$dir"
  mkdir -p "$dir"
  echo "== $name"

  # shellcheck disable=SC2086
  bin/testdoc -content "$content" -out "$dir" -lang "$lang" -layout pages -title "$title" $extra

  scripts/pdf-to-png.sh "$dir/document.pdf" "$dir/page.png" 2.0 >/dev/null
  [ -f "$dir/page.png" ] && mv "$dir/page.png" "$dir/page-1.png"   # single-page PDFs get no suffix

  bin/pdftext-util -lang "$lang" "$dir/document.pdf" >/dev/null
  [ -f "$dir/document-pdftext.json" ] && mv "$dir/document-pdftext.json" "$dir/document-1-pdftext.json"

  pages=$(ls "$dir"/page-*.png | wc -l | tr -d ' ')
  sizes=()
  for n in $(seq 1 "$pages"); do
    png="$dir/page-$n.png"
    mv "$dir/document-$n-pdftext.json" "$dir/pdftext-$n.json"

    bin/ocr-util "$png" >/dev/null
    mv "$dir/page-$n-ocr.json" "$dir/apple-$n.json"
    bin/tesseract-util "$png" >/dev/null
    mv "$dir/page-$n-ocr.json" "$dir/tesseract-$n.json"

    w=$(sips -g pixelWidth "$png" | awk '/pixelWidth/{print $2}')
    h=$(sips -g pixelHeight "$png" | awk '/pixelHeight/{print $2}')
    sizes+=("{\"width\": $w, \"height\": $h}")
  done

  (IFS=,; printf '{\n  "language": "%s",\n  "pages": [%s]\n}\n' "$lang" "${sizes[*]}") > "$dir/info.json"
  echo "   $pages pages"
done
