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
# Derived fixtures (no rendering) follow the rendered ones:
#   *-noiseNN   seeded misreads injected into the Apple pages of a base fixture
#   *-drift     the base fixture's pages against a canonical text from a
#               "different edition" (see make_drift below)
#   *-wrapped   the base fixture's pages against a hard-wrapped canonical text
#   *-lowres    the base PDF rasterized at 72 dpi and OCR'd again
#
# The suite (go test ./integration -run TestMultipage) reads only the
# committed JSON and canonical.txt.
set -euo pipefail
cd "$(dirname "$0")/.."

make -s build utils >/dev/null
go build -o bin/noise-inject ./cmd/noise-inject
CONTENT=testdata/ocr-tests/content
OUT=testdata/multipage

# name | language | content file | title | extra testdoc flags
ENTRIES=(
  "english-book|english|$CONTENT/english-book.txt|The Keeper's Ledgers|-font-size 11 -line-spacing 1.4"
  "english-chapters|english|$CONTENT/english-chapters.txt|The Keeper's Ledgers|-font-size 11 -line-spacing 1.4 -chapters -heads alternate -recto Carrow_Light"
  "english-two-column|english|$CONTENT/english-book.txt|The Keeper's Ledgers|-font-size 10 -line-spacing 1.35 -columns 2 -paper 7x9 -column-gap 18"
  "spanish-book|spanish|$CONTENT/spanish.txt|El Tejido de la Vida Cotidiana|-font-size 11 -line-spacing 1.4"
  "arabic-book|arabic|$CONTENT/arabic.txt|نسيج الحياة اليومية|-direction rtl -font-size 12 -line-spacing 1.6"
  "hindi-book|hindi|$CONTENT/hindi.txt|दैनिक जीवन का ताना-बाना|-font-size 11 -line-spacing 1.5"
  "chinese-book|chinese|$CONTENT/chinese.txt|日常生活的织锦|-font-size 11 -line-spacing 1.6 -paper 5x7"
  "japanese-book|japanese|$CONTENT/japanese.txt|日々の暮らしの織物|-font-size 11 -line-spacing 1.6 -paper 5x7"
  "japanese-vertical|japanese|$CONTENT/japanese-vertical.txt|日々の暮らしの織物|-direction vertical -font-size 11 -line-spacing 1.6 -paper 5x7"
)

# OCR language hints, as in scripts/generate-ocr-tests.sh.
apple_lang() {
  case "$1" in
    japanese) echo "-lang ja-JP" ;;
    chinese)  echo "-lang zh-Hans,zh-Hant" ;;
    arabic)   echo "-lang ar-SA" ;;
    spanish)  echo "-lang es-ES" ;;
    hindi)    echo "SKIP" ;;   # Vision does not support Devanagari
    *)        echo "" ;;
  esac
}
tesseract_lang() {
  case "$1" in
    japanese) echo "-lang jpn+jpn_vert+eng" ;;
    chinese)  echo "-lang chi_sim+chi_tra+eng" ;;
    arabic)   echo "-lang ara+eng" ;;
    spanish)  echo "-lang spa+eng" ;;
    hindi)    echo "-lang hin+eng" ;;
    *)        echo "" ;;
  esac
}

for entry in "${ENTRIES[@]}"; do
  IFS='|' read -r name lang content title extra <<< "$entry"
  dir="$OUT/$name"
  rm -rf "$dir"
  mkdir -p "$dir"
  echo "== $name"

  # Underscores stand in for spaces inside a flag value (-recto Carrow_Light).
  args=()
  for word in $extra; do args+=("${word//_/ }"); done
  bin/testdoc -content "$content" -out "$dir" -lang "$lang" -layout pages -title "$title" "${args[@]}"

  scripts/pdf-to-png.sh "$dir/document.pdf" "$dir/page.png" 2.0 >/dev/null
  [ -f "$dir/page.png" ] && mv "$dir/page.png" "$dir/page-1.png"   # single-page PDFs get no suffix

  bin/pdftext-util -lang "$lang" "$dir/document.pdf" >/dev/null
  [ -f "$dir/document-pdftext.json" ] && mv "$dir/document-pdftext.json" "$dir/document-1-pdftext.json"

  pages=$(ls "$dir"/page-*.png | wc -l | tr -d ' ')
  sizes=()
  alang=$(apple_lang "$lang")
  tlang=$(tesseract_lang "$lang")
  for n in $(seq 1 "$pages"); do
    png="$dir/page-$n.png"
    mv "$dir/document-$n-pdftext.json" "$dir/pdftext-$n.json"

    if [ "$alang" != "SKIP" ]; then
      # shellcheck disable=SC2086
      bin/ocr-util $alang "$png" >/dev/null
      mv "$dir/page-$n-ocr.json" "$dir/apple-$n.json"
    fi
    # shellcheck disable=SC2086
    bin/tesseract-util $tlang "$png" >/dev/null
    mv "$dir/page-$n-ocr.json" "$dir/tesseract-$n.json"

    w=$(sips -g pixelWidth "$png" | awk '/pixelWidth/{print $2}')
    h=$(sips -g pixelHeight "$png" | awk '/pixelHeight/{print $2}')
    sizes+=("{\"width\": $w, \"height\": $h}")
  done

  (IFS=,; printf '{\n  "language": "%s",\n  "pages": [%s]\n}\n' "$lang" "${sizes[*]}") > "$dir/info.json"
  echo "   $pages pages"
done

# --- Derived: noise -------------------------------------------------------
# Seeded character misreads in NN% of the Apple word blocks of every page.
SEED=20261113
for base in english-book; do
  for rate in 05 10; do
    dir="$OUT/$base-noise$rate"
    rm -rf "$dir"; mkdir -p "$dir"
    cp "$OUT/$base/canonical.txt" "$OUT/$base/info.json" "$dir/"
    for src in "$OUT/$base"/apple-*.json; do
      n=$(basename "$src" .json); n=${n#apple-}
      bin/noise-inject -in "$src" -out "$dir/apple-$n.json" -rate "0.$rate" -seed "$((SEED + n))" >/dev/null
    done
    echo "== $base-noise$rate (derived)"
  done
done

# --- Derived: edition drift ----------------------------------------------
# The same scanned pages, matched against a canonical text from another
# edition: it opens with a licence block the scan does not have, carries an
# editor's note mid-text that the scan does not have, and lacks one
# paragraph the scan does have. info.json lists the canonical lines that are
# on no page ("absent_lines"), so scoring can leave them out.
make_drift() {
  local base="$1" dir="$OUT/$1-drift"
  rm -rf "$dir"; mkdir -p "$dir"
  cp "$OUT/$base"/*-[0-9]*.json "$dir/"
  python3 - "$OUT/$base" "$dir" <<'PY'
import json, sys
base, out = sys.argv[1], sys.argv[2]
lines = open(f"{base}/canonical.txt").read().rstrip("\n").split("\n")
front = [
    "This electronic edition is provided for anyone anywhere at no cost and with almost no restrictions whatsoever.",
    "",
    "You may copy it, give it away or re-use it under the terms of the licence included with this edition.",
    "",
    "Produced by volunteers from page images generously made available by the Carrow Island Historical Society.",
    "",
]
note = "Editor's note: the 1961 ledger is now held by the county archive, where it may be consulted by appointment."
drop = next(i for i, l in enumerate(lines) if l.startswith("The crossing took fifty minutes."))
anchor = next(i for i, l in enumerate(lines) if l.startswith("The next page was 4 January"))

new, absent = [], []
for l in front:
    if l: absent.append(len(new))
    new.append(l)
for i, l in enumerate(lines):
    if i == drop or (i == drop + 1 and l == ""):
        continue
    new.append(l)
    if i == anchor:
        new.append("")
        absent.append(len(new)); new.append(note)
open(f"{out}/canonical.txt", "w").write("\n".join(new) + "\n")
info = json.load(open(f"{base}/info.json"))
info["absent_lines"] = absent
json.dump(info, open(f"{out}/info.json", "w"), indent=2)
open(f"{out}/info.json", "a").write("\n")
PY
  echo "== $1-drift (derived)"
}
make_drift english-book

# --- Derived: hard-wrapped canonical --------------------------------------
# The same pages against a canonical text wrapped at 70 columns with blank
# lines between paragraphs - the shape of a plain-text transcription. Every
# canonical "line" is then a visual line of the transcription, not a
# paragraph, and its breaks fall nowhere near the page's own line breaks.
make_wrapped() {
  local base="$1" dir="$OUT/$1-wrapped"
  rm -rf "$dir"; mkdir -p "$dir"
  cp "$OUT/$base"/*-[0-9]*.json "$OUT/$base/info.json" "$dir/"
  python3 - "$OUT/$base/canonical.txt" "$dir/canonical.txt" <<'PY'
import sys, textwrap
out = []
for line in open(sys.argv[1]).read().rstrip("\n").split("\n"):
    out.extend(textwrap.wrap(line, 70) if line.strip() else [""])
open(sys.argv[2], "w").write("\n".join(out) + "\n")
PY
  echo "== $1-wrapped (derived)"
}
make_wrapped english-book

# --- Derived: low-resolution scan ----------------------------------------
# The same PDF rasterized at 1x (72 dpi) instead of 2x and OCR'd again:
# real recognition errors on small type, rather than injected ones.
make_lowres() {
  local base="$1" dir="$OUT/$1-lowres"
  rm -rf "$dir"; mkdir -p "$dir"
  cp "$OUT/$base/canonical.txt" "$dir/"
  scripts/pdf-to-png.sh "$OUT/$base/document.pdf" "$dir/page.png" 1.0 >/dev/null
  local sizes=() n=1
  while [ -f "$dir/page-$n.png" ]; do
    bin/ocr-util "$dir/page-$n.png" >/dev/null && mv "$dir/page-$n-ocr.json" "$dir/apple-$n.json"
    bin/tesseract-util "$dir/page-$n.png" >/dev/null && mv "$dir/page-$n-ocr.json" "$dir/tesseract-$n.json"
    w=$(sips -g pixelWidth "$dir/page-$n.png" | awk '/pixelWidth/{print $2}')
    h=$(sips -g pixelHeight "$dir/page-$n.png" | awk '/pixelHeight/{print $2}')
    sizes+=("{\"width\": $w, \"height\": $h}")
    n=$((n + 1))
  done
  (IFS=,; printf '{\n  "language": "english",\n  "pages": [%s]\n}\n' "${sizes[*]}") > "$dir/info.json"
  echo "== $1-lowres (derived)"
}
make_lowres english-book
