#!/bin/bash
set -e

# OCR Test Runner (Fast)
# Uses pre-generated OCR data to quickly validate sorting algorithm
# Run this frequently during development to catch regressions

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_DIR="${TEST_DIR:-$PROJECT_ROOT/testdata/ocr-tests}"
BIN_DIR="$PROJECT_ROOT/bin"

# Color codes
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Ensure binaries exist
if [ ! -f "$BIN_DIR/gollate" ]; then
    echo "Error: gollate not found. Run 'make build' first."
    exit 1
fi

# Test configuration file (in same directory as test directories)
CONFIG_FILE="$TEST_DIR/test-config.conf"
# Handle case where TEST_DIR is overridden - look in PROJECT_ROOT/testdata/ocr-tests
if [ ! -f "$CONFIG_FILE" ] && [ "$TEST_DIR" != "$PROJECT_ROOT/testdata/ocr-tests" ]; then
    CONFIG_FILE="$PROJECT_ROOT/testdata/ocr-tests/test-config.conf"
fi

# Find all test directories
TEST_DIRS=$(find "$TEST_DIR" -mindepth 1 -maxdepth 1 -type d ! -name 'content' | sort)

if [ -z "$TEST_DIRS" ]; then
    echo -e "${RED}No test directories found!${NC}"
    echo "Run ./scripts/generate-ocr-tests.sh first to create test data."
    exit 1
fi

# Count tests
TOTAL_TESTS=$(echo "$TEST_DIRS" | wc -l | xargs)

echo "=================================================="
echo "  OCR Test Runner (Fast Mode)"
echo "=================================================="
echo "Found $TOTAL_TESTS test directories"
echo ""

START_TIME=$(date +%s)
PASSED=0
FAILED=0
SKIPPED=0

# Initialize problems.todo file
PROBLEMS_FILE="$PROJECT_ROOT/problems.todo"
cat > "$PROBLEMS_FILE" << 'EOF'
# OCR Sorting Problems

This file lists differences between canonical (ground truth) text and sorted OCR output.
Generated automatically by run-ocr-tests.sh.

**Legend:**
- [ ] = Action item / potential fix to try
- **Issue** = Description of the problem
- **Possible causes** = Why this might be happening
- **Potential fixes** = Things to try to improve results

---

EOF

# Function to check if test should be skipped
should_skip_test() {
    local test_name=$1
    local engine=$2

    if [ ! -f "$CONFIG_FILE" ]; then
        return 1  # No config, don't skip
    fi

    # Search for matching rule in config file
    # Format: test-name:engine:reason
    local match=$(grep -v "^#" "$CONFIG_FILE" | grep -v "^$" | grep "^${test_name}:${engine}:")

    if [ -n "$match" ]; then
        # Extract reason (everything after second colon)
        echo "$match" | cut -d: -f3-
        return 0  # Should skip
    fi
    return 1  # Should not skip
}

# Function to run sorting test
run_sorting_test() {
    local test_dir=$1
    local test_name=$(basename "$test_dir")
    local engine=$2
    local ocr_file="$test_dir/${engine}-ocr.json"
    local sorted_file="$test_dir/${engine}-sorted.json"
    local canonical_file="$test_dir/canonical.txt"

    if [ ! -f "$ocr_file" ]; then
        return 1  # Skip
    fi

    if [ ! -f "$canonical_file" ]; then
        echo -e "    ${RED}✗${NC} Missing canonical.txt"
        return 2  # Error
    fi

    # Page dimensions and language come from test-info.json (written by
    # testdoc). Language is the only hint the sorter is allowed.
    local info_file="$test_dir/test-info.json"
    local width=1632 height=2112 language=""
    if [ -f "$info_file" ] && command -v jq &> /dev/null; then
        width=$(jq -r '.width' "$info_file")
        height=$(jq -r '.height' "$info_file")
        language=$(jq -r '.language' "$info_file")
    fi

    # Run sorting with correction suggestions enabled
    "$BIN_DIR/gollate" \
        --engine "$engine" \
        --ocr-file "$ocr_file" \
        --text-file "$canonical_file" \
        --width "$width" \
        --height "$height" \
        --language "$language" \
        --format json \
        --output "$sorted_file" \
        --enable-corrections \
        --correction-edit-distance 2 > /dev/null 2>&1

    if [ $? -eq 0 ] && [ -s "$sorted_file" ]; then
        return 0  # Success
    else
        return 2  # Error
    fi
}

# Function to generate overlay
generate_overlay() {
    local test_dir=$1
    local engine=$2
    local ocr_file="$test_dir/${engine}-ocr.json"
    local overlay_file="$test_dir/${engine}-overlay.jpg"
    local image_file="$test_dir/document.png"
    if [ ! -f "$image_file" ]; then
        image_file="$test_dir/document.jpg"
    fi

    if [ ! -f "$BIN_DIR/ocr-highlight" ]; then
        return 1  # Skip
    fi

    if [ ! -f "$image_file" ] || [ ! -f "$ocr_file" ]; then
        return 1  # Skip
    fi

    "$BIN_DIR/ocr-highlight" \
        -image "$image_file" \
        -ocr "$ocr_file" \
        -engine "$engine" \
        -output "$overlay_file" > /dev/null 2>&1

    if [ $? -eq 0 ] && [ -s "$overlay_file" ]; then
        return 0  # Success
    else
        return 1  # Skip/Error
    fi
}

# Function to extract sorted text from JSON
extract_sorted_text() {
    local sorted_json=$1
    local test_name=$(basename $(dirname "$sorted_json"))

    # Detect if this is a CJK language (no spaces between characters)
    local is_cjk=false
    if [[ "$test_name" == japanese-* ]] || [[ "$test_name" == chinese-* ]]; then
        is_cjk=true
    fi

    # Use jq if available for clean extraction
    if command -v jq &> /dev/null; then
        # Extract blocks and handle empty blocks as paragraph breaks
        if [ "$is_cjk" = true ]; then
            # CJK: concatenate non-empty blocks without spaces, empty blocks create paragraph breaks
            jq -r '.sorted_blocks[] | .text' "$sorted_json" 2>/dev/null | \
                awk 'BEGIN { line="" }
                     { if ($0 == "") {
                         if (line != "") print line;
                         print "";
                         line=""
                       } else {
                         line = line $0
                       }
                     }
                     END { if (line != "") print line }'
        else
            # Non-CJK: join blocks with spaces, empty blocks create paragraph breaks
            jq -r '.sorted_blocks[] | .text' "$sorted_json" 2>/dev/null | \
                awk 'BEGIN { line="" }
                     { if ($0 == "") {
                         if (line != "") { sub(/ $/, "", line); print line };
                         print "";
                         line=""
                       } else {
                         line = line $0 " "
                       }
                     }
                     END { if (line != "") { sub(/ $/, "", line); print line } }'
        fi
    else
        # Fallback: basic extraction
        grep '"text":' "$sorted_json" | \
            sed 's/.*"text": *"\([^"]*\)".*/\1/' | \
            tr '\n' ' ' | \
            sed 's/  */ /g'
    fi
}

extract_unsorted_text() {
    local ocr_json=$1
    local engine=$2
    local test_name=$(basename $(dirname "$ocr_json"))

    # Detect if this is a CJK language (no spaces between characters)
    local is_cjk=false
    if [[ "$test_name" == japanese-* ]] || [[ "$test_name" == chinese-* ]]; then
        is_cjk=true
    fi

    # Use jq if available for clean extraction
    if command -v jq &> /dev/null; then
        if [ "$engine" = "apple" ]; then
            # Apple format: array of lines with words
            if [ "$is_cjk" = true ]; then
                jq -r '.[] | .words[] | .text' "$ocr_json" 2>/dev/null | tr -d '\n'
            else
                jq -r '.[] | .words[] | .text' "$ocr_json" 2>/dev/null | tr '\n' ' '
            fi
        else
            # Tesseract format: {words: [...]}
            if [ "$is_cjk" = true ]; then
                jq -r '.words[] | .text' "$ocr_json" 2>/dev/null | tr -d '\n'
            else
                jq -r '.words[] | .text' "$ocr_json" 2>/dev/null | tr '\n' ' '
            fi
        fi
    else
        # Fallback: basic extraction
        grep '"text":' "$ocr_json" | \
            sed 's/.*"text": *"\([^"]*\)".*/\1/' | \
            tr '\n' ' '
    fi
}

# Function to compare canonical and sorted text
compare_texts() {
    local test_name=$1
    local engine=$2
    local test_dir=$3
    local canonical_file="$test_dir/canonical.txt"
    local sorted_file="$test_dir/${engine}-sorted.json"
    local problems_file="$PROJECT_ROOT/problems.todo"

    if [ ! -f "$canonical_file" ] || [ ! -f "$sorted_file" ]; then
        return 1
    fi

    # Extract sorted text
    local sorted_text=$(extract_sorted_text "$sorted_file")

    # Read canonical text
    local canonical_text=$(cat "$canonical_file")

    # Use Python to calculate diff-based metrics
    local temp_canonical=$(mktemp)
    local temp_sorted=$(mktemp)
    echo "$canonical_text" > "$temp_canonical"
    echo "$sorted_text" > "$temp_sorted"

    local metrics=$(python3 - "$temp_canonical" "$temp_sorted" << 'PYTHON'
import sys
import re
from difflib import SequenceMatcher

def normalize_text(text):
    text = text.lower()
    text = ' '.join(text.split())
    return text

def is_cjk_char(char):
    """Check if character is Chinese, Japanese, or Korean"""
    code = ord(char)
    return (0x4E00 <= code <= 0x9FFF or    # CJK Unified Ideographs
            0x3040 <= code <= 0x309F or    # Hiragana
            0x30A0 <= code <= 0x30FF or    # Katakana
            0xAC00 <= code <= 0xD7AF)      # Hangul

def split_into_units(text):
    """
    Split text into comparison units.
    For CJK text: split into individual characters
    For other text: split into words
    """
    # Check if text is primarily CJK
    if not text:
        return []

    cjk_count = sum(1 for c in text if is_cjk_char(c))
    is_cjk_text = cjk_count > len(text) * 0.3  # >30% CJK characters

    if is_cjk_text:
        # For CJK: extract only alphanumeric characters (including CJK)
        units = [c for c in text if c.isalnum()]
    else:
        # For non-CJK: extract words
        units = re.findall(r'\w+', text)

    return units

with open(sys.argv[1], 'r') as f:
    canonical = f.read()
with open(sys.argv[2], 'r') as f:
    sorted_text = f.read()

canonical = normalize_text(canonical)
sorted_text = normalize_text(sorted_text)

canonical_units = split_into_units(canonical)
sorted_units = split_into_units(sorted_text)

if len(canonical_units) == 0 and len(sorted_units) == 0:
    print("0:0:0:100.0")
elif len(canonical_units) == 0:
    print(f"{len(sorted_units)}:{len(sorted_units)}:0:0.0")
elif len(sorted_units) == 0:
    print(f"{len(canonical_units)}:0:{len(canonical_units)}:0.0")
else:
    matcher = SequenceMatcher(None, canonical_units, sorted_units)
    matching_blocks = matcher.get_matching_blocks()
    matches = sum(block.size for block in matching_blocks[:-1])
    deletions = len(canonical_units) - matches
    insertions = len(sorted_units) - matches
    accuracy = 100.0 * (matches / len(canonical_units))
    print(f"{len(canonical_units)}:{len(sorted_units)}:{matches}:{accuracy:.2f}")
PYTHON
)

    rm -f "$temp_canonical" "$temp_sorted"

    # Parse metrics: canonical_count:sorted_count:matches:accuracy
    local canonical_units=$(echo "$metrics" | cut -d: -f1)
    local sorted_units=$(echo "$metrics" | cut -d: -f2)
    local matches=$(echo "$metrics" | cut -d: -f3)
    local accuracy=$(echo "$metrics" | cut -d: -f4)

    # Calculate differences
    local deletions=$((canonical_units - matches))
    local insertions=$((sorted_units - matches))
    local total_diff=$((deletions + insertions))

    # Only report if accuracy is below 100%
    if [ "$(echo "$accuracy < 100.0" | bc -l)" -eq 1 ]; then
        # Append to problems file
        echo "" >> "$problems_file"
        echo "## Problem: $test_name / $engine" >> "$problems_file"
        echo "" >> "$problems_file"
        echo "**Accuracy:** ${accuracy}%" >> "$problems_file"
        echo "**Canonical units:** $canonical_units" >> "$problems_file"
        echo "**Sorted units:** $sorted_units" >> "$problems_file"
        echo "**Matches:** $matches" >> "$problems_file"
        echo "**Missing from output:** $deletions units" >> "$problems_file"
        echo "**Extra in output:** $insertions units" >> "$problems_file"
        echo "" >> "$problems_file"

        # Analyze the type of problem
        if [ $deletions -gt $insertions ]; then
            local missing_pct=$(awk "BEGIN {printf \"%.1f\", ($deletions / $canonical_units) * 100}")
            echo "**Issue:** Sorted output is missing approximately $missing_pct% of content" >> "$problems_file"
            echo "" >> "$problems_file"
            echo "**Possible causes:**" >> "$problems_file"
            echo "- OCR failed to detect some text blocks" >> "$problems_file"
            echo "- Blocks were filtered out due to low confidence" >> "$problems_file"
            echo "" >> "$problems_file"
            echo "**Potential fixes:**" >> "$problems_file"
            echo "- [ ] Review OCR quality - check ${engine}-ocr.json for missing blocks" >> "$problems_file"
            echo "- [ ] Review correction thresholds (--correction-edit-distance)" >> "$problems_file"
        elif [ $insertions -gt $deletions ]; then
            local extra_pct=$(awk "BEGIN {printf \"%.1f\", ($insertions / $canonical_units) * 100}")
            echo "**Issue:** Sorted output has approximately $extra_pct% extra content" >> "$problems_file"
            echo "" >> "$problems_file"
            echo "**Possible causes:**" >> "$problems_file"
            echo "- OCR detected spurious text (noise, artifacts, headers/footers)" >> "$problems_file"
            echo "- Document has text not in canonical file (e.g., page numbers, headers)" >> "$problems_file"
            echo "" >> "$problems_file"
            echo "**Potential fixes:**" >> "$problems_file"
            echo "- [ ] Review ${engine}-overlay.jpg to identify extra blocks" >> "$problems_file"
            echo "- [ ] Add confidence filtering for noisy OCR" >> "$problems_file"
            echo "- [ ] Implement header/footer detection and filtering" >> "$problems_file"
        else
            echo "**Issue:** Similar amounts of missing and extra content (OCR errors or word order issues)" >> "$problems_file"
            echo "" >> "$problems_file"
            echo "**Possible causes:**" >> "$problems_file"
            echo "- OCR misread characters (substitution errors)" >> "$problems_file"
            echo "- Sorting algorithm chose wrong path through blocks" >> "$problems_file"
            echo "- Multi-column layout confused the sorter" >> "$problems_file"
            echo "" >> "$problems_file"
            echo "**Potential fixes:**" >> "$problems_file"
            echo "- [ ] Review ${engine}-overlay.jpg to check block order" >> "$problems_file"
            echo "- [ ] Tune distance calculation for this layout type" >> "$problems_file"
            echo "- [ ] Check if OCR character confusion patterns can be corrected" >> "$problems_file"
        fi

        echo "" >> "$problems_file"
        echo "**Files to review:**" >> "$problems_file"
        echo "- testdata/ocr-tests/$test_name/canonical.txt" >> "$problems_file"
        echo "- testdata/ocr-tests/$test_name/${engine}-ocr.json" >> "$problems_file"
        echo "- testdata/ocr-tests/$test_name/${engine}-sorted.json" >> "$problems_file"
        echo "- testdata/ocr-tests/$test_name/${engine}-overlay.jpg" >> "$problems_file"
        echo "- testdata/ocr-tests/$test_name/summary.txt" >> "$problems_file"
        echo "" >> "$problems_file"
        echo "---" >> "$problems_file"

        return 0  # Found differences
    fi

    return 1  # No differences
}

# Function to update summary
update_summary() {
    local test_dir=$1
    local test_name=$(basename "$test_dir")
    local summary_file="$test_dir/summary.txt"

    cat > "$summary_file" << EOF
OCR Test Summary
================
Test: $test_name
Last Run: $(date)

Files:
------
- document.pdf     : Test document
- canonical.txt    : Ground truth text
EOF

    # Add info for each engine
    for engine in apple tesseract; do
        local engine_upper=$(echo "$engine" | tr '[:lower:]' '[:upper:]')
        local skip_var="SKIP_${engine_upper}"
        local skip_reason="${!skip_var}"

        if [ -n "$skip_reason" ]; then
            echo "" >> "$summary_file"
            echo "${engine} OCR:" >> "$summary_file"
            echo "  ⊘ SKIPPED: $skip_reason" >> "$summary_file"
        elif [ -f "$test_dir/${engine}-ocr.json" ]; then
            echo "" >> "$summary_file"
            echo "${engine} OCR:" >> "$summary_file"
            echo "  - ${engine}-ocr.json       : Raw OCR output" >> "$summary_file"

            if [ -f "$test_dir/${engine}-sorted.json" ]; then
                echo "  - ${engine}-sorted.json   : Sorted OCR output" >> "$summary_file"

                # Count blocks in sorted output
                local sorted_blocks=$(grep -o '"text"' "$test_dir/${engine}-sorted.json" 2>/dev/null | wc -l | xargs)
                echo "  - Sorted blocks: $sorted_blocks" >> "$summary_file"
            fi

            if [ -f "$test_dir/${engine}-overlay.jpg" ]; then
                echo "  - ${engine}-overlay.jpg   : Visualization with block IDs" >> "$summary_file"
            fi

            # Count blocks in raw OCR
            local block_count=$(grep -o '"text"' "$test_dir/${engine}-ocr.json" 2>/dev/null | wc -l | xargs)
            echo "  - Raw block count: $block_count" >> "$summary_file"
        fi
    done

    # Append unsorted text for each engine
    echo "" >> "$summary_file"
    echo "========================================" >> "$summary_file"
    echo "Unsorted OCR Output (Reading Order)" >> "$summary_file"
    echo "========================================" >> "$summary_file"

    for engine in apple tesseract; do
        local engine_upper=$(echo "$engine" | tr '[:lower:]' '[:upper:]')
        local skip_var="SKIP_${engine_upper}"
        local skip_reason="${!skip_var}"

        if [ -n "$skip_reason" ]; then
            echo "" >> "$summary_file"
            echo "${engine} (unsorted):" >> "$summary_file"
            echo "----------------------------------------" >> "$summary_file"
            echo "⊘ SKIPPED: $skip_reason" >> "$summary_file"
        elif [ -f "$test_dir/${engine}-ocr.json" ]; then
            echo "" >> "$summary_file"
            echo "${engine} (unsorted):" >> "$summary_file"
            echo "----------------------------------------" >> "$summary_file"

            # Extract and format unsorted text
            local unsorted_text=$(extract_unsorted_text "$test_dir/${engine}-ocr.json" "$engine")
            if [ -n "$unsorted_text" ]; then
                echo "$unsorted_text" >> "$summary_file"
            else
                echo "(No text extracted)" >> "$summary_file"
            fi
        fi
    done

    # Append sorted text for each engine
    echo "" >> "$summary_file"
    echo "" >> "$summary_file"
    echo "========================================" >> "$summary_file"
    echo "Sorted Text Output" >> "$summary_file"
    echo "========================================" >> "$summary_file"

    for engine in apple tesseract; do
        local engine_upper=$(echo "$engine" | tr '[:lower:]' '[:upper:]')
        local skip_var="SKIP_${engine_upper}"
        local skip_reason="${!skip_var}"

        if [ -n "$skip_reason" ]; then
            echo "" >> "$summary_file"
            echo "${engine} (sorted):" >> "$summary_file"
            echo "----------------------------------------" >> "$summary_file"
            echo "⊘ SKIPPED: $skip_reason" >> "$summary_file"
        elif [ -f "$test_dir/${engine}-sorted.json" ]; then
            echo "" >> "$summary_file"
            echo "${engine} (sorted):" >> "$summary_file"
            echo "----------------------------------------" >> "$summary_file"

            # Extract and format sorted text
            local sorted_text=$(extract_sorted_text "$test_dir/${engine}-sorted.json")
            if [ -n "$sorted_text" ]; then
                # Output each paragraph as a single line to preserve structure
                echo "$sorted_text" >> "$summary_file"
            else
                echo "(No text extracted)" >> "$summary_file"
            fi
        fi
    done
}

# Process each test
current=0
for test_dir in $TEST_DIRS; do
    current=$((current + 1))
    test_name=$(basename "$test_dir")

    echo -e "${BLUE}[$current/$TOTAL_TESTS]${NC} Testing: $test_name"

    test_passed=true
    SKIP_APPLE=""
    SKIP_TESSERACT=""

    # Test Apple Vision if available
    if [ -f "$test_dir/apple-ocr.json" ]; then
        # Temporarily disable exit-on-error for skip check (returns 1 when not skipping)
        set +e
        skip_reason=$(should_skip_test "$test_name" "apple")
        skip_result=$?
        set -e
        if [ $skip_result -eq 0 ]; then
            echo -e "  ${YELLOW}⊘${NC} Apple test skipped: $skip_reason"
            SKIP_APPLE="$skip_reason"
            SKIPPED=$((SKIPPED + 1))
        else
            if run_sorting_test "$test_dir" "apple"; then
                echo -e "  ${GREEN}✓${NC} Apple sorting passed"
                generate_overlay "$test_dir" "apple" > /dev/null 2>&1
            else
                echo -e "  ${RED}✗${NC} Apple sorting failed"
                test_passed=false
            fi
        fi
    fi

    # Test Tesseract if available
    if [ -f "$test_dir/tesseract-ocr.json" ]; then
        # Temporarily disable exit-on-error for skip check (returns 1 when not skipping)
        set +e
        skip_reason=$(should_skip_test "$test_name" "tesseract")
        skip_result=$?
        set -e
        if [ $skip_result -eq 0 ]; then
            echo -e "  ${YELLOW}⊘${NC} Tesseract test skipped: $skip_reason"
            SKIP_TESSERACT="$skip_reason"
            SKIPPED=$((SKIPPED + 1))
        else
            if run_sorting_test "$test_dir" "tesseract"; then
                echo -e "  ${GREEN}✓${NC} Tesseract sorting passed"
                generate_overlay "$test_dir" "tesseract" > /dev/null 2>&1
            else
                echo -e "  ${RED}✗${NC} Tesseract sorting failed"
                test_passed=false
            fi
        fi
    fi

    # Compare canonical vs sorted for non-skipped tests (before unsetting skip vars)
    # compare_texts returns 1 when a test is at 100% accuracy (nothing to
    # report); don't let set -e treat that as a failure.
    if [ -z "$SKIP_APPLE" ] && [ -f "$test_dir/apple-sorted.json" ]; then
        compare_texts "$test_name" "apple" "$test_dir" || true
    fi
    if [ -z "$SKIP_TESSERACT" ] && [ -f "$test_dir/tesseract-sorted.json" ]; then
        compare_texts "$test_name" "tesseract" "$test_dir" || true
    fi

    # Update summary (pass skip info as environment variables)
    export SKIP_APPLE
    export SKIP_TESSERACT
    update_summary "$test_dir"
    unset SKIP_APPLE SKIP_TESSERACT

    # Track results
    if $test_passed; then
        PASSED=$((PASSED + 1))
    else
        FAILED=$((FAILED + 1))
    fi
done

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

# Count problems in problems.todo
PROBLEM_COUNT=$(grep -c "^## Problem:" "$PROBLEMS_FILE" 2>/dev/null || echo "0")

# Add summary to problems.todo
cat >> "$PROBLEMS_FILE" << EOF

# Summary

**Total problems found:** $PROBLEM_COUNT
**Generated:** $(date)
**Duration:** ${DURATION}s

## Next Steps

1. Review each problem and its suggested fixes
2. Prioritize fixes based on:
   - Test importance (e.g., single-column should work perfectly)
   - Severity of the issue (% of words missing/wrong)
   - Ease of fix
3. Test changes by running: \`./scripts/run-ocr-tests.sh\`
4. Monitor this file after changes to track improvements

## General Improvement Areas

- **OCR Quality**: Consider using higher resolution images or better OCR engines
- **Corrections**: Implement spelling correction for common OCR errors
- **Layout Detection**: Improve multi-column and complex layout handling
- **Language Support**: Add better support for RTL, vertical text, and non-Latin scripts
EOF

echo ""
echo "=================================================="
echo "  Test Results"
echo "=================================================="
echo -e "${GREEN}Passed:${NC}  $PASSED"
echo -e "${RED}Failed:${NC}  $FAILED"
echo -e "${YELLOW}Skipped:${NC} $SKIPPED"
echo -e "Duration: ${DURATION}s"
echo ""
echo -e "${BLUE}Problems found:${NC} $PROBLEM_COUNT"
echo -e "${BLUE}Problems file:${NC} problems.todo"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    exit 1
fi
