
# gollate

[![Go Reference](https://pkg.go.dev/badge/github.com/goodblaster/gollate.svg)](https://pkg.go.dev/github.com/goodblaster/gollate)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**gollate** sorts ocr-extracted data using supplied, canonical text. 
It analyzes the text and ocr blocks, tries to determine the most logical order for those blocks, 
and then returns them to a user for further use.

## Features
- Go library for sorting OCR blocks using canonical text
- Command-line tool for testing and standalone usage

## Testing

See [TESTING.md](TESTING.md) — current accuracy by language/engine, how the
regression suite works, and the known algorithm issues queued up next.
Suite mechanics: [testdata/ocr-tests/README.md](testdata/ocr-tests/README.md).


### Input

- **engine**
  - The OCR engine that was used to extract the data being sent in the input_json field.
  - Built-in engines:
      - tesseract
      - easyocr
      - apple (macOS only)
      - blocks (engine-neutral: pre-normalized blocks, works with any OCR source)
  - Any other engine — Textract, Google Document AI, a local model — can be
    used either through the `blocks` format or by registering a custom
    engine (see "Adding an OCR engine" below).
- **lines**
  - An array of lines that are canonical; we will attempt to sort OCR data to match these lines.
  - This is not text that was extract via OCR. 
  - If we are working with a web page, the text was probably pulled from innerText.
  - If we are working with a PDF, the text was probably pulled from the text layer.
  - If we are working with a DOC file, the text was probably pulled from the text layer.
- **input_json**
  - This is the data as extracted by the OCR engine.
  - It will be in different formats, depending on the engine used to extract the data.
  - The engine parameter will tell us how to interpret this data. 
- **page_width**
  - Page width in pixels. We need this to make certain determinations because OCR data generally works in percentages.
- **page_height**
  - Page height in pixels. We need this to make certain determinations because OCR data generally works in percentages.



### Output

- **document** — the primary output format (modeled on Google Document AI,
  flipped from the original Textract-style block list):
  - **text**: all sorted text as one readable blob, paragraphs separated by
    newlines. Read this first.
  - **paragraphs**: layout referencing the blob — each paragraph has a
    `span` (byte range into `text`), `bounds` (box around the whole
    paragraph), and `tokens`.
  - **tokens**: one per word — a `span` into `text`, that word's page
    `bounds` (0-1 fractions), and OCR `confidence`. Tokens carry no text of
    their own; slice `text` with the span. Spans are byte offsets (safe for
    direct slicing, matters for CJK/accents).
  - In Go, `Document`, `Paragraph`, and `Token` have `String()` methods
    that return their slice of the text — debugging convenience, not part
    of the JSON.
- **unhandled** — canonical lines that couldn't be matched to any OCR blocks
  (hidden elements, dynamic content, or OCR failures), one readable line
  each, whitespace trimmed.
- **meta** — statistics and provenance about the sort:
  - **stats**: a success/failure snapshot — `lines_found`,
    `lines_unhandled`, `lines_split`, `lines_reconciled`, `line_repairs`,
    `holes_bridged`/`holes_filled`, `leftover_blocks`, `canonical_lines`,
    `input_blocks`, `passes`, `elapsed_ms`.
  - **source**: what was sorted — `engine`, `language`, `width`/`height`,
    `slices` (how many strips the image was cut into for OCR), and
    `image_file`/`ocr_file`/`text_file` (path + byte size). The CLI fills
    these; the library never sees the original image.
  - **extra**: any caller-supplied pass-through (`Meta` on the request).
- **sorted_blocks** *(deprecated — prefer `document`; not emitted by
  default)*: the legacy flat block list, with empty blocks marking line
  breaks. Library consumers may still populate it explicitly.

The default output is `document` (JSON). Build the standard response in Go
with `api.NewSortResponse(sorter)`.



## Algorithm

The **gollate** algorithm works in the following steps:

1. Normalize inputs. This mostly means dealing with punctuation. Julie's -> Julies, etc.
2. Sort lines by length. We improve accuracy by looking for the longest lines first.
3. Map ocr text contents to blocks. We will use this to determine all possible paths to a solution.
4. For each line:
   1. Find all possible paths to the line (paths may cross visual line
      wraps and bridge words missing from OCR — see error tolerance below).
   2. If necessary, break the line into smaller pieces.
   3. Determine the shortest path; short duplicated lines tie-break by
      proximity to their matched canonical neighbors.
   4. Track what we did and did not find.
5. Reconciliation pass: rescue unfound short fragments using neighbors as
   spatial anchors.
6. Add leftover words.
7. Sort all lines by their original index.

Real OCR misreads and drops words; the error-tolerance mechanisms
(wrap bridging, wildcard holes, short-line anchoring, reconciliation) keep
matching working anyway, and `ConfigForLanguage` enables the combination
each script measurably benefits from. Details: [pkg/sorters/README.md](pkg/sorters/README.md)
and [ALGORITHM.md](ALGORITHM.md); measurements: [TESTING.md](TESTING.md).

## Installation

### As a Library

```bash
go get github.com/goodblaster/gollate
```

### Command-Line Tool

```bash
git clone https://github.com/goodblaster/gollate.git
cd gollate
make build
# Binary will be at bin/gollate
```

Or install to your $GOPATH/bin:

```bash
make install
```

## Usage

### One-shot pipeline: `make sort`

The whole document-to-sorted-text pipeline (rasterize if PDF, OCR with
auto-slicing, sort) in one command. Sorted output goes to stdout; all
progress goes to stderr, so redirecting gives a clean file (macOS only):

```bash
make sort IMG=document.pdf TEXT=canonical.txt LANG=english > sorted.txt

# JSON output, Tesseract engine, vertical Japanese:
make sort IMG=page.png TEXT=canon.txt ENGINE=tesseract LANG=japanese \
         OCR_LANG=jpn+jpn_vert+eng FORMAT=json > sorted.json
```

Variables: `IMG` and `TEXT` required; `ENGINE` (apple|tesseract, default
apple), `LANG` (sorter language, default english), `OCR_LANG` (engine
language codes passed to the OCR utility), `FORMAT` (text|json, default
text). The pipeline logic lives in `scripts/sort.sh`.

### Command-Line Tool (individual steps)

```bash
# Starting from a PDF? OCR engines need raster images:
scripts/pdf-to-png.sh document.pdf          # -> document.png at 2x
bin/ocr-util document.png                   # Apple Vision OCR (macOS), auto-slices tall pages
# or: bin/tesseract-util document.png       # Tesseract OCR

# Born-digital PDF? Skip OCR and extract the embedded text layer with its
# positions instead. Emits the blocks format; sort with --engine blocks.
# Backends are pluggable with graceful fallback: pdfkit (macOS built-in)
# and poppler (any OS, `brew install poppler`); -backend auto (default)
# picks the best installed one for the script — pass -lang so Hindi gets
# poppler (PDFKit mangles Devanagari). Matches or beats OCR for Latin,
# CJK (including vertical Japanese), Arabic, and Hindi — see the
# per-case baselines in testdata/ocr-tests/baselines.json. This is an input source only — canonical
# text must still come from a structured source (innerText, DOC text),
# never the PDF.
bin/pdftext-util -lang english document.pdf # -> document-pdftext.json

# Basic usage - output sorted text to stdout
gollate --engine apple \
         --language english \
         --ocr-file data.json \
         --text-file text.txt \
         --width 1920 \
         --height 1080

# Output as JSON to a file
gollate --engine tesseract \
         --ocr-file ocr.json \
         --text-file canonical.txt \
         --width 2000 \
         --height 2588 \
         --format json \
         --output result.json

# Generate debug output with visualization images
gollate --engine apple \
         --ocr-file data.json \
         --text-file text.txt \
         --width 1920 \
         --height 1080 \
         --image screenshot.png \
         --debug-dir ./debug-output
```

#### Command-Line Flags

- `--engine` - OCR engine: apple, tesseract, easyocr, blocks (default: apple on macOS, tesseract elsewhere)
- `--ocr-file` - Path to OCR JSON file [required]
- `--text-file` - Path to canonical text file (one line per line) [required]
- `--width` - Page width in pixels [required]
- `--height` - Page height in pixels [required]
- `--language` - Document language hint (english, spanish, chinese, japanese, arabic, hindi)
- `--image` - Path to image file (for debug visualization)
- `--source` - Original source file (PDF/image), recorded in response `meta.source`
- `--slices` - Strips the image was sliced into for OCR, recorded in `meta.source`
- `--output` - Output file path (default: stdout)
- `--debug-dir` - Directory for debug output files (enables debug mode)
- `--format` - Output format: json, text (default: json)

### OCR Engine JSON Formats

Each OCR engine has a specific JSON format that gollate expects:

#### Tesseract

```json
{
  "words": [
    {
      "text": "Hello",
      "line_num": 0,
      "left": 100,
      "top": 200,
      "width": 50,
      "height": 20,
      "conf": 95.5
    }
  ]
}
```

- Coordinates are in pixels
- `conf` is confidence (0-100 scale)
- `line_num` tracks which line the word belongs to

#### EasyOCR

```json
{
  "results": [
    {
      "bbox": [[x1,y1], [x2,y2], [x3,y3], [x4,y4]],
      "text": "Hello",
      "confidence": 0.95
    }
  ]
}
```

- `bbox` contains four corner points of the text region
- Coordinates are in pixels
- `confidence` is 0-1 scale

#### Apple Vision (macOS only)

See [Apple Vision Integration](CLAUDE.md#apple-vision-ocr-macos-only) for details on the JSON format from VNRecognizeTextRequest.

#### Blocks (any engine)

The engine-neutral format: a JSON array of blocks already normalized to the
common shape, in OCR emit order. Convert any engine's response to this with
a few lines in any language:

```json
[
  {
    "text": "Hello",
    "bounds": {"top": 0.1, "left": 0.2, "width": 0.05, "height": 0.02},
    "normalized_conf": 0.98
  }
]
```

- Coordinates are page fractions (0-1), not pixels
- One block per word/token (not lines or paragraphs)
- `engine` and index fields are optional — filled in automatically

### Adding an OCR engine

For first-class support of another engine (Textract, Google Document AI,
anything local or via API), map a name to code: implement `engines.Engine`
and register it. The full contract adapters must satisfy is documented on
`ocr.Block`.

```go
import (
    "io"

    "github.com/goodblaster/gollate/pkg/engines"
    "github.com/goodblaster/gollate/pkg/ocr"
)

type textract struct{}

func (textract) Name() string { return "textract" }

func (textract) Read(r io.Reader, pageWidth, pageHeight int) ([]ocr.Block, error) {
    // Parse the raw Textract response JSON (WORD blocks; geometry is
    // already 0-1 relative) into ocr.Block values.
}

func init() { engines.Register(textract{}) }
```

After registration, `api.SortRequest{Engine: "textract", ...}` works like
any built-in. Parse raw API response bodies rather than depending on vendor
SDKs, and keep blocks at word granularity.

### As a Go Library

```go
package main

import (
    "fmt"
    "os"

    "github.com/goodblaster/gollate/pkg/api"
    "github.com/goodblaster/gollate/pkg/sorters"
)

func main() {
    ocrJSON, _ := os.ReadFile("data.json")        // raw OCR engine output
    canonical, _ := os.ReadFile("canonical.txt")  // known correct text

    // Parse and normalize the engine-specific OCR JSON.
    request := api.SortRequest{
        Engine:     "apple", // or "tesseract", "easyocr"
        Lines:      splitLines(string(canonical)),
        InputJson:  string(ocrJSON),
        PageWidth:  1920,
        PageHeight: 1080,
    }
    if err := request.Parse(); err != nil {
        panic(err)
    }

    // ConfigForLanguage is the recommended entry point: it selects the
    // per-language configuration, including the error-tolerance mechanisms
    // each script measurably benefits from (see TESTING.md).
    config := sorters.ConfigForLanguage("english")
    sorter := sorters.NewOcrSorterWithConfig(request.Blocks(), request.Lines, nil, config)

    if _, err := sorter.Sort(); err != nil {
        panic(err)
    }

    // NewSortResponse assembles document + unhandled + stats.
    response := api.NewSortResponse(sorter)
    fmt.Println(response.Document.Text)
    fmt.Printf("matched %d/%d lines in %dms\n",
        response.Meta.Stats.LinesFound, response.Meta.Stats.CanonicalLines,
        response.Meta.Stats.ElapsedMs)
}

func splitLines(s string) []string { /* strings.Split(s, "\n") */ return nil }
```

The `nil` logger argument silences the sorter. To capture debug output,
implement `pkg/logger.Logger` (Debug/Debugf/Error/Errorf/Fatal/WithError)
or use the built-in `logger.NewLogos()` / `logger.Noop{}`.

# ILLUSTRATION

### Canonical lines:

-------------------------------------------------------------------------
```
What is Lorem Ipsum?
Lorem Ipsum is simply dummy text of the printing and typesetting industry.
```

### Simplified view of ocr input:

-------------------------------------------------------------------------
```
What is
Lorem Ipsum?
Lorem Ipsum is simply dummy text
of the printing and
typesetting industry.
```

### Lines will be normalized and sorted to:

-------------------------------------------------------------------------
```
lorem ipsum is simply dummy text of the printing and typesetting industry
what is lorem ipsum
```

### OCR blocks will be grouped something like this:

-------------------------------------------------------------------------
```
"what":        [0]
"is":          [1,6]
"lorem":       [2,4]
"ipsum":       [3,5]
"simply":      [7]
"dummy":       [8]
"text":        [9]
"of":          [10]
"the":         [11]
"printing":    [12]
"and":         [13]
"typesetting": [14]
"industry":    [15]
```

Each block will also contain position and size data.
We use this data to determine which words are most likely to read together.

### Start with the longest line:

-------------------------------------------------------------------------
```
Lorem Ipsum is simply dummy text of the printing and typesetting industry.
```

### Determine all possible paths to build this line:

-------------------------------------------------------------------------
2,3,1,7,8,9,11,12,13,14,15
2,3,6,7,8,9,11,12,13,14,15
2,5,1,7,8,9,11,12,13,14,15
2,5,6,7,8,9,11,12,13,14,15
4,3,1,7,8,9,11,12,13,14,15
4,3,6,7,8,9,11,12,13,14,15
4,5,1,7,8,9,11,12,13,14,15
4,5,6,7,8,9,11,12,13,14,15

### Measure the distances between each word.

-------------------------------------------------------------------------
```
d(2,3) + d(3,1) + d(1,7) ...
d(2,3) + d(3,6) + d(7,7) ...
```

As we calculate these values, if a distance is so great as to be statistically unlikely, abandon that path.
Finally, choose the shortest path.

Words cannot be reused, so they must be removed from the index.
Assume this is our shortest path: 
```
4,5,6,7,8,9,11,12,13,14,15
```

That means, the index will become:
```
"what":        [0]
"is":          [1]
"lorem":       [2]
"ipsum":       [3]
```

**You can see how this speeds analysis of future lines.**

### The next line to handle is:
```
what is lorem ipsum
```

-------------------------------------------------------------------------
There is only one possible path: 
```
1,2,3,4
```
And that path perfectly matches our remaining ocr blocks.

We now have 2 lines/sentences:
```
4,5,6,7,8,9,11,12,13,14,15
0,1,2,3
```

If we sort them according to the original indexing, we are likely to put them in an order most likely to be ready by a human. Sort using just the first word. Result:
```
0,1,2,3
4,5,6,7,8,9,11,12,13,14,15
```

Text output is:
```
what is lorem ipsum
lorem ipsum is simply dummy text of the printing and typesetting industry
```

This exactly matches out normalized canonical input text.

### Our final sorted output:

-------------------------------------------------------------------------
```
0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15
```
This is the same order we received the data in. 
But, this is just a simple illustration of the algorithm; 
in most real world scenarios, you should see many differences.

There is a benefit that was not conveyed in this illustration. The output will actually be something more like:
```
0,1,2,3,X,4,5,6,7,8,9,10,11,12,13,14,15
```
X is an empty block that symbolizes a separator then can be interpreted as a line break. 
If you look at the original ocr input, 
you can see that the output has removed the unwanted line breaks, 
while retaining the one line break that we do want.

If all words were run together without breaks, 
"ipsum lorem" is a term we could find that really does not exist in the document.

Also, as we received the original ocr text, 
"printing and typesetting industry" is a term we could not have found 
because the words appeared on separate lines.

## License

gollate is released under the [MIT License](LICENSE).
