#!/usr/bin/env bash
# Add a real document to your private test corpus (see integration/corpus_test.go).
# Rasterizes (if PDF), OCRs, and writes a suite-shaped entry into
# $OCRSORT_CORPUS/<name>/ - so copyrighted documents stay entirely out of the
# repo while their accuracy is still tracked. macOS (Apple Vision, sips).
#
# Usage:
#   OCRSORT_CORPUS=~/ocr-corpus scripts/corpus-add.sh -n contract -i ~/docs/contract.pdf -t ~/docs/contract.txt
#
#   -n NAME   corpus entry name (directory under $OCRSORT_CORPUS)  (required)
#   -i FILE   source image (.png/.jpg/...) or .pdf                 (required)
#   -t FILE   canonical text file                                  (required)
#   -e ENGINE apple | tesseract          (default: apple on macOS, else tesseract)
#   -l LANG   sorter language (english, japanese, ...)             (default english)
#   -L CODES  OCR engine language codes (e.g. ja-JP, jpn+jpn_vert+eng)
#
# After adding entries, record baselines and run:
#   OCRSORT_CORPUS=~/ocr-corpus UPDATE_BASELINES=1 go test ./integration -run TestCorpus
#   OCRSORT_CORPUS=~/ocr-corpus go test ./integration -run TestCorpus
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/bin"

: "${OCRSORT_CORPUS:?set OCRSORT_CORPUS to your corpus directory}"

name="" input="" text="" engine="" lang=english ocr_lang=""
usage() { sed -n '2,20p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2; exit 2; }

while getopts "n:i:t:e:l:L:h" opt; do
  case "$opt" in
    n) name="$OPTARG" ;;
    i) input="${OPTARG/#\~\//$HOME/}" ;;
    t) text="${OPTARG/#\~\//$HOME/}" ;;
    e) engine="$OPTARG" ;;
    l) lang="$OPTARG" ;;
    L) ocr_lang="$OPTARG" ;;
    *) usage ;;
  esac
done

[ -n "$name" ] && [ -n "$input" ] && [ -n "$text" ] || { echo "error: -n, -i, -t required" >&2; usage; }
[ -f "$input" ] || { echo "error: no such file: $input" >&2; exit 1; }
[ -f "$text" ]  || { echo "error: no such file: $text" >&2; exit 1; }
[ -f "$BIN/ocr-util" ] || { echo "error: run 'make utils' first" >&2; exit 1; }
if [ -z "$engine" ]; then
  [ "$(uname -s)" = "Darwin" ] && engine=apple || engine=tesseract
fi

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
shopt -s nocasematch
if [[ "$input" == *.pdf ]]; then
  "$ROOT/scripts/pdf-to-png.sh" "$input" "$tmp/page.png" >&2
  img="$tmp/page.png"; [ -f "$img" ] || img="$tmp/page-1.png"
else
  img="$tmp/page.${input##*.}"; cp "$input" "$img"
fi
shopt -u nocasematch

lang_flag=()
[ -n "$ocr_lang" ] && lang_flag=(-lang "$ocr_lang")
case "$engine" in
  apple)     "$BIN/ocr-util" ${lang_flag[@]+"${lang_flag[@]}"} "$img" >&2 ;;
  tesseract) "$BIN/tesseract-util" ${lang_flag[@]+"${lang_flag[@]}"} "$img" >&2 ;;
  *) echo "error: -e must be apple or tesseract" >&2; exit 1 ;;
esac
ocr_json="${img%.*}-ocr.json"
[ -s "$ocr_json" ] || { echo "error: OCR produced no output" >&2; exit 1; }

read -r w h < <(sips -g pixelWidth -g pixelHeight "$img" \
  | awk '/pixelWidth:/{w=$2} /pixelHeight:/{h=$2} END{print w, h}')

dest="$OCRSORT_CORPUS/$name"
mkdir -p "$dest"
cp "$text" "$dest/canonical.txt"
cp "$ocr_json" "$dest/$engine-ocr.json"
printf '{\n  "language": "%s",\n  "layout": "corpus",\n  "direction": "ltr",\n  "width": %s,\n  "height": %s\n}\n' \
  "$lang" "$w" "$h" > "$dest/test-info.json"

echo ">> added $dest ($engine, ${w}x${h})" >&2
echo "   run: OCRSORT_CORPUS='$OCRSORT_CORPUS' UPDATE_BASELINES=1 go test ./integration -run TestCorpus" >&2
