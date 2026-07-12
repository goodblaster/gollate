#!/usr/bin/env bash
# Full document -> sorted text pipeline: rasterize (if PDF), OCR, sort.
# The sorted result is the ONLY thing on stdout (redirect it to a file);
# all progress and logs go to stderr. macOS (Apple Vision, sips, pdf-to-png).
#
# Usage:
#   scripts/sort.sh -i <image-or-pdf> -t <canonical.txt> [options]
#
#   -i FILE   input image (.png/.jpg/...) or .pdf              (required)
#   -t FILE   canonical text file, one line per line           (required)
#   -e ENGINE apple | tesseract           (default: apple on macOS, else tesseract)
#   -l LANG   sorter language (english, japanese, arabic, ...)  (default english)
#   -L CODES  OCR engine language codes (e.g. ja-JP, jpn+jpn_vert+eng)
#   -f FORMAT json | text                                      (default json)
#
# Tall images are sliced automatically by the OCR utilities; multi-page PDFs
# sort the first page only.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/bin"

engine=""
lang=english
format=json
ocr_lang=""
input=""
text=""

usage() { sed -n '2,18p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2; exit 2; }

while getopts "i:t:e:l:L:f:h" opt; do
  case "$opt" in
    i) input="$OPTARG" ;;
    t) text="$OPTARG" ;;
    e) engine="$OPTARG" ;;
    l) lang="$OPTARG" ;;
    L) ocr_lang="$OPTARG" ;;
    f) format="$OPTARG" ;;
    *) usage ;;
  esac
done

# Expand a leading ~/ ourselves: make passes its variables quoted, so the
# shell never expands the tilde.
input="${input/#\~\//$HOME/}"
text="${text/#\~\//$HOME/}"

[ -n "$input" ] && [ -n "$text" ] || { echo "error: -i and -t are required" >&2; usage; }
[ -f "$input" ] || { echo "error: no such file: $input" >&2; exit 1; }
[ -f "$text" ]  || { echo "error: no such file: $text" >&2; exit 1; }

# Default engine matches the platform's OCR tooling: Apple Vision on macOS,
# Tesseract elsewhere.
if [ -z "$engine" ]; then
  [ "$(uname -s)" = "Darwin" ] && engine=apple || engine=tesseract
fi
case "$engine" in apple|tesseract) ;; *) echo "error: -e must be apple or tesseract" >&2; exit 1 ;; esac

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# --- Prepare a raster image in the temp dir (so *-ocr.json lands there) ---
shopt -s nocasematch
if [[ "$input" == *.pdf ]]; then
  echo ">> rasterizing PDF at 2x..." >&2
  "$ROOT/scripts/pdf-to-png.sh" "$input" "$tmp/page.png" >&2
  if [ -f "$tmp/page.png" ]; then
    img="$tmp/page.png"
  elif [ -f "$tmp/page-1.png" ]; then
    echo ">> multi-page PDF: sorting page 1 only" >&2
    img="$tmp/page-1.png"
  else
    echo "error: rasterization produced no image" >&2; exit 1
  fi
else
  img="$tmp/page.${input##*.}"
  cp "$input" "$img"
fi
shopt -u nocasematch

# --- OCR (utilities auto-slice tall images) ---
lang_flag=()
[ -n "$ocr_lang" ] && lang_flag=(-lang "$ocr_lang")
echo ">> OCR ($engine)..." >&2
ocr_log="$tmp/ocr.log"
case "$engine" in
  apple)     util="$BIN/ocr-util" ;;
  tesseract) util="$BIN/tesseract-util" ;;
esac
if ! "$util" ${lang_flag[@]+"${lang_flag[@]}"} "$img" > "$ocr_log" 2>&1; then
  cat "$ocr_log" >&2; echo "error: OCR failed" >&2; exit 1
fi
cat "$ocr_log" >&2
ocr_json="${img%.*}-ocr.json"
[ -s "$ocr_json" ] || { echo "error: OCR produced no output ($ocr_json)" >&2; exit 1; }

# Slice count for response meta (utilities print "Sliced into N strips";
# absent means the image was short enough to OCR whole).
slices=$(sed -n 's/.*Sliced into \([0-9][0-9]*\) strips.*/\1/p' "$ocr_log" | head -1)
[ -z "$slices" ] && slices=1

# --- Image dimensions (the sorter needs pixel width/height) ---
read -r w h < <(sips -g pixelWidth -g pixelHeight "$img" \
  | awk '/pixelWidth:/{w=$2} /pixelHeight:/{h=$2} END{print w, h}')
[ -n "$w" ] && [ -n "$h" ] || { echo "error: could not read image dimensions" >&2; exit 1; }
echo ">> sorting ${w}x${h}, language=$lang..." >&2

# --- Sort: result to stdout, logs already to stderr ---
"$BIN/gollate" -engine "$engine" -language "$lang" \
  -ocr-file "$ocr_json" -text-file "$text" \
  -width "$w" -height "$h" -format "$format" \
  -source "$input" -slices "$slices"
