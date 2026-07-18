#!/bin/bash
set -e

# OCR Test Suite Generator (Phase 1 - slow, run when content/layouts change)
#
# For each language/layout combination this generates, via bin/testdoc:
#   document.pdf, document.png (2x raster), document.html,
#   canonical.txt (ground truth incl. title/footer, in reading order),
#   test-info.json (language, layout, direction, image dimensions)
# then runs Apple Vision and Tesseract OCR on document.png.
#
# Fast validation afterwards: go test ./integration  (or make integration)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_DIR="$PROJECT_ROOT/testdata/ocr-tests"
CONTENT_DIR="$TEST_DIR/content"
BIN_DIR="$PROJECT_ROOT/bin"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

for tool in testdoc ocr-util tesseract-util; do
    if [ ! -f "$BIN_DIR/$tool" ]; then
        echo "Error: $tool not found. Run 'make utils' first."
        exit 1
    fi
done

# Test matrix: one entry per test directory.
# Format: "language layout direction [extra testdoc flags]"
# Direction "-" means testdoc's default for the language (rtl for arabic).
# English is primary and gets the extra layout archetypes.
MATRIX=(
    "english single -"
    "english two-column -"
    "english three-column -"
    "english mixed-sizes -"
    "english sidebar -"
    "english grid - -layout grid -font-size 11"
    "english-legal single -"
    "spanish single -"
    "spanish two-column -"
    "spanish three-column -"
    "chinese single -"
    "chinese two-column -"
    "chinese three-column -"
    "japanese single -"
    "japanese two-column - -layout two-column -font-size 10"
    "japanese three-column - -layout three-column -font-size 10"
    "japanese single-vertical - -direction vertical -layout single -font-size 10"
    "japanese two-column-vertical - -direction vertical -layout two-column -font-size 10"
    "japanese book-vertical - -layout book -direction vertical -font-size 10"
    "arabic single -"
    "arabic two-column -"
    "arabic three-column -"
    "hindi single -"
    "hindi two-column -"
    "hindi three-column - -layout three-column -font-size 11"
)

# Apple Vision language hints (only languages Vision supports).
apple_lang() {
    case "$1" in
        japanese*) echo "-lang ja-JP" ;;
        chinese*)  echo "-lang zh-Hans,zh-Hant" ;;
        arabic*)   echo "-lang ar-SA" ;;
        spanish*)  echo "-lang es-ES" ;;
        hindi*)    echo "SKIP" ;;  # Vision does not support Devanagari
        *)         echo "" ;;
    esac
}

tesseract_lang() {
    case "$1" in
        japanese*) echo "-lang jpn+jpn_vert+eng" ;;
        chinese*)  echo "-lang chi_sim+chi_tra+eng" ;;
        arabic*)   echo "-lang ara+eng" ;;
        spanish*)  echo "-lang spa+eng" ;;
        hindi*)    echo "-lang hin+eng" ;;
        *)         echo "" ;;
    esac
}

echo "=================================================="
echo "  OCR Test Suite Generator"
echo "=================================================="
echo "${#MATRIX[@]} test combinations"
echo ""

FAILED=0
current=0
for entry in "${MATRIX[@]}"; do
    current=$((current + 1))
    read -r lang layout direction extra <<< "$entry"
    test_name="${lang}-${layout}"
    test_dir="$TEST_DIR/$test_name"
    content_file="$CONTENT_DIR/${lang}.txt"

    echo -e "${BLUE}[$current/${#MATRIX[@]}]${NC} $test_name"

    if [ ! -f "$content_file" ]; then
        echo -e "  ${RED}✗${NC} Missing content file: $content_file"
        FAILED=$((FAILED + 1))
        continue
    fi

    mkdir -p "$test_dir"

    # Build testdoc argument list. Entries with extra flags set their own
    # -layout/-direction; plain entries use the matrix columns.
    args=(-content "$content_file" -out "$test_dir" -lang "$lang")
    if [ -z "$extra" ]; then
        args+=(-layout "$layout")
        [ "$direction" != "-" ] && args+=(-direction "$direction")
    else
        # shellcheck disable=SC2206
        args+=($extra)
    fi

    if ! "$BIN_DIR/testdoc" "${args[@]}"; then
        echo -e "  ${RED}✗${NC} Document generation failed"
        FAILED=$((FAILED + 1))
        continue
    fi
    echo -e "  ${GREEN}✓${NC} Document generated"

    # Apple Vision OCR
    apple_args=$(apple_lang "$test_name")
    if [ "$apple_args" = "SKIP" ]; then
        echo -e "  ${YELLOW}⊘${NC} Apple OCR skipped (language unsupported by Vision)"
        rm -f "$test_dir/apple-ocr.json"
    else
        # shellcheck disable=SC2086
        if "$BIN_DIR/ocr-util" $apple_args "$test_dir/document.png" > /dev/null 2>&1 \
            && [ -s "$test_dir/document-ocr.json" ]; then
            mv "$test_dir/document-ocr.json" "$test_dir/apple-ocr.json"
            echo -e "  ${GREEN}✓${NC} Apple OCR complete"
        else
            echo -e "  ${YELLOW}⊘${NC} Apple OCR produced no output"
            rm -f "$test_dir/document-ocr.json" "$test_dir/apple-ocr.json"
        fi
    fi

    # Tesseract OCR
    tess_args=$(tesseract_lang "$test_name")
    # shellcheck disable=SC2086
    if "$BIN_DIR/tesseract-util" $tess_args "$test_dir/document.png" > /dev/null 2>&1 \
        && [ -s "$test_dir/document-ocr.json" ]; then
        mv "$test_dir/document-ocr.json" "$test_dir/tesseract-ocr.json"
        echo -e "  ${GREEN}✓${NC} Tesseract OCR complete"
    else
        echo -e "  ${YELLOW}⊘${NC} Tesseract OCR produced no output"
        rm -f "$test_dir/document-ocr.json" "$test_dir/tesseract-ocr.json"
    fi
done

echo ""
echo "=================================================="
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All test data generated.${NC}"
else
    echo -e "${RED}$FAILED combination(s) failed.${NC}"
fi
echo "=================================================="
echo ""
echo "Next: go test ./integration          # validate sorting accuracy"
echo "      UPDATE_BASELINES=1 go test ./integration   # ratchet baselines"

exit $FAILED
