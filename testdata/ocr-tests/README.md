# OCR Test Suite

Regression suite validating OCR sorting across languages and layouts.

## Design

- **Documents are generated, not hand-made.** `bin/testdoc` renders each
  language's content through HTML/CSS layouts using headless Chrome, which
  handles bidi (Arabic), Devanagari, vertical Japanese, CSS columns, and font
  fallback correctly. Generation **fails if content overflows the page**, so
  broken/overlapping documents cannot silently enter the suite.
- **Ground truth cannot drift.** `canonical.txt` (title first, body in reading
  order, footer last) is emitted by `testdoc` from the same in-memory document
  that produced the PDF/PNG.
- **Language is the only hint.** The sorter receives the language (mapped to a
  config via `sorters.ConfigForLanguage`) and nothing else. Layout information
  — column counts, orientation, font sizes — must be inferred by the
  algorithm from block geometry, never supplied by the harness.
- **Accuracy ratchets.** The Go suite scores order-sensitive accuracy (longest
  common subsequence of words, or characters for CJK, vs canonical) and fails
  any case that drops below its committed baseline. Improvements raise
  baselines; regressions fail CI.

## Two-phase workflow

### Phase 1 — generate documents + OCR (slow; when content/layouts change)

```bash
make build utils
./scripts/generate-ocr-tests.sh
```

For each language/layout combination this produces:

```
<language>-<layout>/
  document.pdf     # the test document (for humans)
  document.png     # 2x raster (1632x2112) — the OCR input
  document.html    # intermediate HTML, for debugging layout
  canonical.txt    # ground truth, full page, reading order
  test-info.json   # language, layout, direction, image dimensions
  apple-ocr.json   # Apple Vision output (when supported)
  tesseract-ocr.json
```

### Phase 2 — validate sorting (fast; run constantly during development)

```bash
go test ./integration        # or: make integration
```

Sorts the committed OCR JSON, scores against canonical, compares to
`baselines.json`. ~6 seconds for the whole suite.

When accuracy legitimately improves:

```bash
UPDATE_BASELINES=1 go test ./integration   # or: make baselines
```

This raises baselines (never lowers them) and adds entries for new cases.
Commit the updated `baselines.json`. If an intentional change trades accuracy
away somewhere, lower that entry by hand in the same commit that explains why.

### Optional — human-readable debugging artifacts

```bash
./scripts/run-ocr-tests.sh
```

Regenerates `*-sorted.json`, `*-overlay.jpg` (numbered block visualization),
per-test `summary.txt`, and `problems.todo`.

## Test matrix

Defined in `scripts/generate-ocr-tests.sh` (`MATRIX` array). Languages:
english (primary), spanish, chinese, japanese, arabic (RTL), hindi. Layouts:

| Layout | What it exercises |
|---|---|
| single | title + body paragraphs, one column |
| two-column / three-column | column wrap, title spanning columns |
| mixed-sizes (english) | h2 section headings + small-print paragraphs |
| sidebar (english) | asymmetric columns, smaller sidebar font |
| grid (english) | product tiles: distinct headlines over heavily repeated short lines ("Learn more"/"Buy" ×12) — exercises duplicate-line anchoring and single-word reconciliation |
| english-legal (single) | quote-heavy legal prose (sentences ending in `'quotes.'`) from `content/english-legal.txt` — a content archetype, not a layout. Note: on a *clean* render this sorts ~99% via the leftover assembler regardless, so it does not by itself isolate the sentence-splitter's quote handling — the unit tests in `pkg/sorters/line_manipulation_test.go` guard that directly, and the private corpus (below) exercises the real-world behavior. |
| single-vertical / two-column-vertical (japanese) | vertical writing (tategaki), currently near-0% pending reading-order auto-detection |

## Noise fixtures (`*-noise02/05/10`)

Derived fixtures measuring OCR-error tolerance: `scripts/generate-noise-fixtures.sh`
runs `cmd/noise-inject` on a clean, high-accuracy apple case
(english/spanish/chinese-single), injecting seeded, deterministic character
misreads into 2/5/10% of word blocks. Only block *text* changes — bounding
boxes are untouched — so any accuracy loss is attributable purely to text
mismatch. The derived JSON is committed; regenerate only if the base
fixtures change (same seed = same output).

## Experiments (A/B testing config changes)

```bash
./scripts/run-experiments.sh "EnableChainHoles=false" "MaxPasses=5,..."
```

Runs the suite once per flag combination (`EXPERIMENT_FLAGS` overrides any
`SorterConfig` field on top of the language config) and prints a per-case
accuracy delta table vs a default-config run. Experiment runs are never
gated by baselines and refuse `UPDATE_BASELINES`.

Some combinations need a smaller font to fit one page (the generator fails
loudly on overflow); those carry explicit `-font-size` flags in the matrix.

Apple Vision does not support Devanagari, so hindi has no `apple-ocr.json`.
The Go suite only tests engine files that exist.

## Private corpus (real documents that can't be committed)

Synthetic renders are clean, so they under-represent real-world failure
modes (a clean page is carried by the leftover assembler no matter what the
matcher does). Real documents — a legal PDF, a webpage capture, a scanned
form — are the meaningful regression tests, but they usually can't live in
the repo (copyright, PII, size).

The `TestCorpus` harness (`integration/corpus_test.go`) handles this the
standard way: only the harness is committed; the documents and their scores
stay in a private directory you point at with `OCRSORT_CORPUS`. Each entry is
shaped exactly like a suite fixture (`canonical.txt`, `<engine>-ocr.json`,
`test-info.json`), and baselines live in `<corpus>/baselines.json`. The test
skips when `OCRSORT_CORPUS` is unset, so it never blocks CI or contributors.

```bash
export OCRSORT_CORPUS=~/ocr-corpus

# Add a document (rasterizes if PDF, OCRs, writes the entry):
scripts/corpus-add.sh -n contract -i ~/docs/contract.pdf -t ~/docs/contract.txt

# Record baselines, then run the gate:
UPDATE_BASELINES=1 go test ./integration -run TestCorpus
go test ./integration -run TestCorpus
```

## Adding a language or layout

1. Add `content/<language>.txt` (~8 paragraphs, blank-line separated), plus
   entries in `titles`, `headings`, and `fontStacks` in `cmd/testdoc/main.go`
   and a language case in `sorters.ConfigForLanguage`.
2. Add matrix entries in `scripts/generate-ocr-tests.sh` (and OCR language
   codes in `apple_lang`/`tesseract_lang`).
3. New layouts are CSS work in `cmd/testdoc/main.go` (`buildDocument`).
4. Run phase 1, inspect `document.png`, then `make baselines` and commit.
