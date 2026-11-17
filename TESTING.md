# Testing: State of the World

Single entry point for how gollate is tested, where accuracy stands, and
what the known algorithm issues are. Suite mechanics live in
[testdata/ocr-tests/README.md](testdata/ocr-tests/README.md); this file is
the context you'd otherwise have to rediscover.

Last updated: 2026-11-16.

## Quick reference

```bash
go test ./...                              # everything (~10s)
make integration                           # accuracy gate only (~4s)
make baselines                             # ratchet baselines up after an improvement
make build utils && ./scripts/generate-ocr-tests.sh   # regenerate documents + OCR (minutes)
./scripts/run-ocr-tests.sh                 # human-debug artifacts (overlays, problems.todo)
./scripts/run-experiments.sh "Flag=v,..."  # A/B flag-gated experiments vs default config
./scripts/generate-noise-fixtures.sh       # regenerate seeded noise fixtures (*-noiseNN)
./scripts/pdf-to-png.sh doc.pdf [out] [2.0] # rasterize a PDF for OCR (engines need images)
OCRSORT_CORPUS=~/c go test ./integration -run TestCorpus  # score private real documents
```

**Synthetic fixtures under-represent real-world failure modes.** A clean
render is carried by the leftover spatial assembler regardless of matcher
quality, so bugs that only bite dense/messy real documents (e.g. the
sentence-split cascade) don't reproduce on generated pages — verified
2026-07-12: a quote-heavy legal fixture scored ~identically with and without
the quote-boundary split fix, while the real Legalese PDF improved. Guard
matcher *logic* with unit tests, and guard real-world behavior with the
private corpus (`integration/corpus_test.go`, `scripts/corpus-add.sh`;
documents stay out of the repo, only the harness is committed — see
testdata/ocr-tests/README.md).

Ground rules (details in CLAUDE.md):

- **Language is the only hint** the sorter may receive (`sorters.ConfigForLanguage`).
  Layout info must be inferred from word-block geometry, never passed in.
- Accuracy = order-sensitive LCS of words (characters for CJK) vs
  `canonical.txt`, gated per-case by `testdata/ocr-tests/baselines.json`
  (ratchets upward only; lower an entry by hand only with a commit message
  explaining the trade).
- Sorting is deterministic; `TestSortingDeterministic` guards this and the
  baseline tolerance is a tight 0.05. If it flakes, someone reintroduced
  map-iteration-order dependence.

## Where accuracy stands (suite, 2026-11-13, regenerated fixtures)

| Language | Apple Vision | Tesseract | PDF text layer | Notes |
|---|---|---|---|---|
| English (8 layouts incl. grid, sidebar, justified, legal) | 99.5–100 | 98.0–100 | 99.3–100 | justified = hyphenated book page |
| English/Spanish + synthetic noise (2–10%) | 89.8–97.8 | n/a | n/a | tracks OCR recall (90–98) |
| Spanish | 99.2–99.5 | 99.2–99.8 | 99.5 | |
| Arabic (RTL) | 99.7–100 | 79.6–87.8 | 93.0–100 | Tesseract recall is only 82–90 |
| Chinese | 99.5–99.8 | 87.2–91.9 | 99.5–100 | Tesseract recall 90–92 |
| Chinese + synthetic noise | 90.3–97.5 | n/a | n/a | monotone in noise now |
| Japanese horizontal | 99.0–99.9 | 98.0–99.5 | 94.5–99.9 | |
| Hindi | n/a (Vision unsupported) | 96.5–97.2 | 97.2–97.5 | |
| Japanese vertical (3 fixtures) | ~0 (Vision can't read tategaki) | 97.7–98.5 | 99.9–100 | |

Exact per-case numbers: `testdata/ocr-tests/baselines.json`. Suite mean
94.2 over 84 fixture/engine pairs (97.7 without the three Apple-vertical
rows), against a mean OCR recall of 94.6 and a raw emit-order mean of 90.6.

**These numbers are not comparable with anything dated before
2026-11-13.** The fixtures were regenerated that day; see the next section.

## 2026-11-13: fixture regeneration, and what it exposed

The previous content was eight generic two-sentence paragraphs per
language with no digits, dates, currency, percentages, abbreviations,
quoted speech, parentheses or mixed script, and ~160 words in English and
Spanish (english-three-column filled two of three columns). It is now a
market feature article written per language with all of those, plus a
literary passage with kanji numerals for vertical Japanese
(`content/japanese-vertical.txt`) and a hyphenated, justified book page
(`english-justified`).

Two tooling bugs surfaced immediately, both of which had been corrupting
or could corrupt fixtures:

- **`tesseract-util` dropped the rest of a page at a word starting with
  `"`.** It parsed Tesseract's TSV with `encoding/csv`. 103 of 359 words
  survived on an English page with dialogue. Also, Tesseract's `line_num`
  restarts per paragraph, so line ids collided across the page; they are
  now a page-unique counter.
- **`testdoc`'s overflow check missed text running into the bottom
  margin**, i.e. through the footer. The committed hindi-single fixture
  had been overflowing by 26px.

Scored on the new fixtures before any sorter change, the suite averaged
88.0 and many cases scored *below raw OCR emit order* — the sorter was
making them worse. The fixes, in the order they landed, with suite-mean
effect:

1. **Pieces of a split paragraph were ordered by page position** (+2.0).
   A paragraph that wraps into the next column continues above where it
   left off, so its second half came out first. Now ordered by text
   offset within the canonical line.
2. **Compound tokens could not match** (`$4.2`, `6:30`, `12,000` are one
   OCR word but two canonical tokens); split hyphenated words were never
   re-joined in output; punctuation-only blocks were discarded.
3. **The language hint did not select the tokenizer.** Detection counted
   CJK characters over all runes; digits and punctuation pushed ordinary
   Chinese/Japanese below the 80% threshold and onto whitespace
   tokenization, silently disabling pathfinding.
4. **Near-miss leftover lines were emitted after everything else** (+3.9).
   Leftovers match canonical lines by exact containment, which one misread
   defeats; the unmatched went last. Lightly noisy pages therefore scored
   far below heavily noisy ones (Chinese 2% noise 52, 10% noise 78), and
   Tesseract CJK and vertical Japanese sat 15–25 points under their emit
   order. They are now placed by token overlap among the lines spanned by
   their emit-order neighbors (`placeByNeighbors`).
5. **CJK sentence endings were never detected** (last byte compared with
   three-byte marks), so CJK paragraphs were never split.
6. **Leftover split boundaries** were measured with a with-spaces length
   in a space-stripped string.
7. **Line-break hyphenation** (`ven-` / `dors`) is re-joined when the
   result is a canonical word.

Diagnostic habit worth keeping: for any low score, compare **OCR recall**
(canonical units present anywhere in the blocks), **emit-order LCS**, and
the **sorted score**. Sorted ≈ recall means the OCR is the limit. Sorted <
emit order means the sorter is doing damage, which is how items 1 and 4
were found.

## Multi-page (added 2026-11-13)

`pkg/multipage` sorts a document page by page and merges across page
breaks. Its gate is `TestMultipage` (in `go test ./integration`), on
fixtures under `testdata/multipage/` generated by
`scripts/generate-multipage-fixtures.sh` (testdoc layout `pages`: a
5.5x8.5in justified, hyphenated book with a running head and page numbers
that are deliberately absent from canonical.txt).

Four scores are ratcheted per fixture and source, in
`testdata/multipage/baselines.json`:

| Score | What it catches |
|---|---|
| accuracy | whole-document order (same LCS as the single-page suite) |
| `#paragraphs` | a canonical paragraph recovered as one output paragraph - falls when a paragraph is cut at a page break |
| `#precision` | extra output - falls when page numbers or running heads leak into the text |
| `#localized` | each token's canonical span really selects that token's text - what highlighting depends on |

**Whole-document accuracy alone cannot see page-break handling.** A
single-column book scores 99.4 left in raw page order. Measured on
english-book: with straddle handling off, accuracy stays 100 while
`#paragraphs` drops to 96.9; with furniture removal off, accuracy is 99.7
with 20+ stray units.

### Scenarios (14 fixtures, 35 scenario/source pairs)

| Fixture | What it exercises |
|---|---|
| english-book | 4 pages, 3 straddling paragraphs, a word hyphenated across a page break |
| english-chapters | chapter headings each starting a page, left/right running heads that differ, corner page numbers, bare first page, one line repeated in every chapter |
| english-two-column | two columns per page |
| english-book-wrapped | canonical hard-wrapped at 70 columns (82 lines instead of 20), like a plain-text transcription |
| english-book-drift | canonical from another edition: a licence block and an editor's note the scan lacks, minus a paragraph the scan has. Scored on the lines that exist in the scan (`absent_lines` in info.json); precision is below 100 by construction |
| english-book-noise05 / noise10 | seeded misreads in 5% / 10% of Apple word blocks |
| english-book-lowres | the PDF rasterized at 72 dpi and OCR'd again: real recognition errors |
| spanish-book, arabic-book, hindi-book, chinese-book, japanese-book | two or three pages each, one page break |
| japanese-vertical | tategaki across three pages |

Scale is checked separately in `pkg/multipage/scale_test.go`: a synthetic
300-page book (1,351 paragraphs of Zipf-distributed pseudo-words) comes
back with 99.9% of paragraphs exact, 246 joined across page breaks, in
about 3 seconds; a second test forces the banded search.

### 2026-11-16: adversarial page alignment and localization

Added 13 challenge scenarios in `pkg/multipage/challenge_test.go`, including
8 small authored inputs in `pkg/multipage/testdata/challenges.json`. These
are synthetic blocks with known positions, not new OCR measurements. The
location score checks the canonical occurrence **and** the physical page
and box; identical words on the wrong page cannot earn credit.

Compared with commit `4a97ff2`, using identical test inputs:

| Challenge | Before | After |
|---|---:|---:|
| Repeated body lines: text order / exact location | 66.67 / 66.67 | 100 / 100 |
| Repeated canonical lines in margins: text order / exact location | 66.67 / 66.67 | 100 / 100 |
| Compound across a page break (ASCII and Unicode hyphens) | 90 / 90 | 100 / 100 |
| Four-page paragraph: text order / exact location | 100 / 50 | 100 / 100 |
| Three content pages separated by a blank: text order / exact location | 100 / 66.67 | 100 / 100 |
| Mixed Japanese/Latin with a page break inside the Japanese run | 100 / 73.91 | 100 / 100 |
| Five-page paragraph, blank sheet, 5 misreads + 5 drops in 300 words | 56.67 / 20.67 | 96.67 / 96.67 |
| 400-page edition drift: correct body-line page | 20.95 | 100 |

The noisy paragraph reaches its 96.67% surviving-input ceiling; all ten
uncovered positions are reported. The drift case contains 9,925 body lines,
2,500 canonical-only lines (prefix, middle and suffix), and three scan-only
pages. All 2,500 absent lines now remain unassigned. Additional tests check
an intervening blank page between only two fragments, title/header
ambiguity, empty input, and separately identified repeated missing lines.

The existing 36 multi-page fixture/source pairs still pass every gate.
Excluding Apple Vision on vertical Japanese, mean text-order accuracy is
unchanged at **98.38%**, paragraph recovery **98.18%**, and precision
**98.10%**. Canonical-span coverage rises **96.69% → 97.75%**: standalone
symbols such as `38 % mayor` no longer cause a whole Spanish paragraph to
lose its spans. The three Spanish localization baselines were ratcheted
up; no baseline was lowered. This existing span-coverage metric does not
verify physical location; the new challenge metric does.

The original 301-page synthetic book (300-page target plus final spill)
still recovers **99.93% of its 1,351 paragraphs exactly**, with 246 straddles
and 602 furniture runs.

Performance on Apple M4 Pro, darwin/arm64, Go 1.24.4; medians of three
`-benchtime=1x` runs, fixture creation excluded:

| Operation | Before | After | Change |
|---|---:|---:|---:|
| Full 301-page sort | 2.911 s | 1.687 s | 42.1% less time |
| Allocated bytes per book sort | 2.588 GB | 1.249 GB | 51.8% fewer bytes |
| 400-page drift segmentation | 70.0 ms | 48.7 ms | 30.4% less time |
| Allocated bytes per segmentation | 67.7 MB | 26.7 MB | 60.6% fewer bytes |

Allocated bytes are cumulative allocations, not peak resident memory.
Small documents can incur extra work from canonical spans and conservative
furniture decisions. In three `-benchtime=200ms` runs, the repeated-body
case rises from 104 to 148 microseconds (it now retains all the words), and
the mixed-script case from 158 to 197 microseconds. Compounds fall from
121 to 116 microseconds and the four-page case from 192 to 171 microseconds.
These changes are not a universal speedup.

```bash
go test ./pkg/multipage -run Challenge -v
go test ./pkg/multipage -run '^$' -bench 'Benchmark(Book300|Challenge)' -benchtime=1x -count=3
```

Implementation: distinctive three-token sequences anchor segmentation in
physical token positions; long unsupported runs consume no alignment cost;
weak search bands widen. Word counts and rarity weights are cached, and
backtracking stores only searched band cells. Furniture needs a separated
margin location and respects canonical occurrences, while still removing
running copies of a title found in the body. Fragment token ranges replace
the single-boundary assumption internally; `Page`, `Straddle`, and `Cut`
remain as compatibility summaries. Original byte spans locate compound and
mixed-script cuts. Multi-page results now expose `Unmatched` (`unmatched`
in CLI JSON), with canonical line and byte range; `Unhandled` retains the
readable missing runs without deduplicating repeated occurrences.

Limits: sequence anchors still need recognizable, reasonably ordered text.
Heavy corruption and ambiguous repetitions can remain unassigned or be
misplaced. Furniture detection is conservative and can retain unusual
headers. Pages are still loaded in memory; this is not a streaming reader.
The added synthetic cases do not establish accuracy on unseen scanned books.

### Real-book evaluation: Meditations (2026-11-16)

The [Meditations corpus report](testdata/books/meditations/README.md) adds the
complete George Long translation from an EPUB: all twelve books and 122
endnotes, 49,884 words. Fresh single-column (103 pages) and double-column
(102 pages) PDFs were rasterized and recognized by Apple Vision and Tesseract.
PDF text extraction supplies a separate control; EPUB XHTML supplies the
reference. The setup script downloads and generates all book artifacts under ignored
`experiments/meditations/`; only the procedure and small accuracy thresholds
are kept under `testdata/books/meditations/`. EPUBs, PDFs, book text, images,
OCR blocks and generated results are not committed.

Sorted token-order accuracy is 99.422/99.286% for single-column Apple/Tesseract
and 98.460/98.366% for double-column. All six input/control scores are identical
before and after the multi-page changes; sorting takes 28–35% less time in
single-run A/B measurements. A real 884-word paragraph spanning three pages
still gets assigned to only two. Greek and endnote references also expose
recognition errors. This is a real-prose test, not a degraded-scan test.

```bash
BOOK_CORPUS="$PWD/experiments/meditations" go test ./integration -run '^TestBookCorpus$' -v -count=1
```

From a fresh checkout, `python3 -B scripts/meditations-corpus.py --jobs 4`
downloads the checksum-pinned EPUB, prepares both layouts, runs OCR and the
accuracy tests, and generates overlays. Rerunning resumes cached preparation
and always reruns the accuracy gates. Use `--skip-overlays` to omit the large
PNG series. See the corpus report for prerequisites, individual stages,
metric definitions and lightweight tests that require no book download.
The book suite is opt-in and enforces saved accuracy baselines; speed is
descriptive.
Set `BOOK_OVERLAYS=1` on that command to also save full word-number and seeded
sentence spot-check PNGs for every page. To render existing results without
repeating sorting or OCR, use `go run ./cmd/book-overlays`. Numbers are global,
one-based canonical whitespace-word indexes; unlinked output is marked with
red `U` labels. See the corpus report for filenames, continuation labels and
sampling limitations.

### Where it stands (2026-11-13)

Means over 34 pairs (Apple Vision on vertical Japanese left out - it
reads nothing there): accuracy 98.4, paragraph recovery 98.2, precision
98.1, localization 96.7, against a mean OCR recall of 98.0 and a raw
page-then-emit-order mean of 95.6. 22 of 34 pairs are at 99.5 or above.

Every remaining low score is within about a point of what the OCR read,
with one exception:

| Pair | Accuracy | OCR recall | Note |
|---|---|---|---|
| arabic-book/tesseract | 87.0 | 87.6 | Tesseract reads many Arabic words as Latin |
| english-book-noise10/apple | 89.1 | 89.8 | |
| chinese-book/tesseract | 90.8 | 93.1 | **2.3 under recall; localization 84.7** - per-character CJK chaining |
| english-book-noise05/apple | 95.1 | 94.7 | |
| english-book-lowres/tesseract | 95.5 | 95.2 | title and 8pt running heads not read at all; paragraph recovery 91.3 |
| hindi-book | 96.7-97.4 | 97.0-97.4 | |

What the scenarios exposed and fixed, beyond the first fixture:

- **Vertical text emitted left to right.** A paginated vertical PDF's
  text layer lists each page's columns left to right. The single-page
  sorter leaned on emit order for vertical pages; it now reverses the
  columns when column-to-column steps run rightward, and then treats each
  column as a line (52 -> 99.4). This contradicts the older note that
  vertical text layers "extract perfectly": that held for the single-page
  fixtures only.
- **Arabic and Devanagari punctuation was not punctuation** (`،` `؛` `؟`
  `।`): words kept the mark glued on. Single-page Arabic gained too.
- **Misread scraps** are spliced back into the gap they sit in when
  their emit-order neighbors on both sides belong to one canonical line.
- **Straddles under noise**: a second, sequence-based test (the head is
  what the head page ends with, the tail what the next page begins with).
- **A single word carried over a page break.**
- **Running heads with a misread** still count as furniture.

Not covered yet: footnotes, tables and figures with captions; a page
whose running head appears only once; multi-column non-Latin pages;
skewed or photographed scans; canonical text whose paragraphs are split
or merged differently from the page (only hard-wrapping is tested);
and a real book end to end. A line assigned to the wrong page is still
not retried on its neighbors.

### What is still open after this pass

- **Tesseract Arabic** sorts 1–2.5 points below its emit order (79.6–87.8
  vs 80.8–90.2) and Tesseract recall there is only 82–90.
- **chinese-two-column/tesseract** 90.8 vs emit 92.4, and the Chinese
  noise fixtures 0.4–0.7 under emit: per-character CJK chaining picks
  wrong instances of duplicate characters (issue 4 below). Five baselines
  were hand-lowered by 0.1–0.7 when sentence splitting started feeding
  it again.
- **japanese-single/pdftext** 94.5 and **arabic-three-column/pdftext**
  93.0 are the only text-layer cases under 97.
- Per-character CJK pathfinding still finds few lines on clean pages
  (chinese-single/apple: 1 line found); the high scores there come from
  leftover assembly and placement.
- A re-joined hyphenated word keeps only its first half's box, so a
  highlight drawn from it misses the piece on the next line.

## Real-world case study: apple.com homepage

A saved apple.com homepage PDF (webpage render, 3848x17576 at 2x) + its
accessibility text as canonical. Not in the suite; re-run by rasterizing the
PDF, `bin/ocr-util` (auto-slices), `bin/gollate --language english`, LCS
score. Findings:

- **Tall images must be sliced before OCR** (now automatic, `pkg/slicing`,
  threshold 4000px): Apple Vision downscales internally — 3% fine-print
  recall unsliced vs 85% on a crop. Slicing took the page 13.6% → 62.5%
  end-to-end.
- Current (2026-07-07, after line-repair promotion + U+2019 normalization
  fix): **Apple Vision 84.6%, Tesseract 75.3%** (2026-07-06: 83.5/72.9;
  2026-07-05 pre-error-tolerance: 62.5/47.1) vs visible text
  (alt-text lines — leading space in the .txt — are invisible to OCR and
  should be excluded from canonical). Rasterize with
  `scripts/pdf-to-png.sh` (CoreGraphics, defaults to the 2x convention →
  3848x17576) — OCR engines don't read PDFs, and this script is the one
  supported rasterization path (no ghostscript/pdftoppm needed).
- Remaining loss is mostly recall: white-on-photo button text ("Stream
  now" etc.) both engines miss — a contrast/inversion preprocessing pass
  is the untried idea there — plus OCR misreads in the fine print
  (e.g. "1-800-MY-APPLE" mangled) that exceed the hole budget.

## Known algorithm issues (history, 2026-07)

These were established by experiment on 2026-07-04/05 on the old
fixtures; don't re-derive them, but don't quote their numbers either.

1. **Pass-loop early exit starves short lines.** Pass 0 skips lines shorter
   than `MinWordsForEarlyPasses` (16), but `shouldExitPassLoop`
   (pkg/sorters/sort_helpers.go) exits at pass 1 when pass 0 made no
   progress — so on pages of short lines, pathfinding never runs at all
   (apple.com: only 7 of 154 lines ever attempted). The exit rule must not
   fire before the early-pass filter relaxes (`EarlyPassThreshold`).
2. **…but fixing #1 alone makes things worse.** Measured: 62.5% → 49.6% on
   apple.com. Cause: duplicate short lines ("Learn more" ×8, "Buy" ×6) are
   scored only by internal path compactness, so an arbitrary instance gets
   grabbed and blocks are stolen from the wrong region. Needed:
   **context-anchored instance selection** — near-tied candidate paths for a
   short line should tie-break by proximity to the matched blocks of the
   line's canonical neighbors.
3. **Wrap filter blocks multi-visual-line matches — fixed behind
   `EnableWrapBridging` (off by default), promotion pending.** In
   `recurse()` candidates with distance > `MaxWordDistance` (0.5) are
   rejected, but a line wrap costs `BaseLineWrap` (1.0) + gap — so
   pathfinding could never follow a canonical line across a visual line
   break; english-single/apple scored 99.4% with `LinesFound=0`, carried
   entirely by emit order + the leftover assembler. With the flag on
   (measured): hindi-three-column +40.8,
   noise fixtures +6 to +22, clean pages unchanged; noisy Tesseract CJK
   regresses −2 to −8 (wrap steps in dense character grids), which is
   what blocks default-on. Fixing this also surfaced and fixed a
   default-path bug: found-line sentences were paired to canonical lines
   by fragile positional sync in post-processing, desyncing whenever
   leftover matching marked extra lines Found (7 baselines ratcheted up
   from that fix alone).
4. **Vertical-text detection — solved for Tesseract (2026-07-07).** The
   real blocker was upstream: the engines were never reading tategaki at
   all (Tesseract lacked jpn_vert in its language list; Apple Vision
   cannot read vertical Japanese, full stop). With jpn_vert added to the
   Japanese OCR invocation and geometry-based orientation inference in the
   sorter (vertical.go: majority of engine lines flowing along y →
   VerticalTTB_RTL), the two vertical fixtures went 0.9→74.2 and 0.4→66.5
   on Tesseract. The Apple rows stay ~0% as a documented engine
   limitation — no sorter change can compensate for empty input.
   Researched 2026-07-15: this is an API-surface gap, not a missing
   download or language hint. Our invocation is already optimal
   (revision 3 default on macOS 15, accurate mode, ja-JP listed and
   supported). On the vertical fixture VNRecognizeTextRequest returns
   only the two horizontal elements (title/footer) and silently skips
   every vertical column; Apple's own forum confirms no tategaki
   support and suggests rotate-and-reassemble preprocessing. Meanwhile
   VisionKit's ImageAnalyzer (Live Text) reads the same fixture
   perfectly (full transcript, correct reading order) on the same
   machine — Apple's recognizer CAN do it — but its public API exposes
   only `transcript`: no per-word geometry, so it cannot feed the
   sorter. macOS 26's RecognizeDocumentsRequest doesn't document
   vertical support and cannot do word-level segmentation for CJK at
   all. Until Apple exposes vertical text with geometry, Tesseract
   jpn_vert and the PDF text layer are the vertical sources.
   Update 2026-07-17: **vertical wrap-bridging measured and rejected.**
   Both vertical branches of `isWrappedToNextLine` had mirrored sides
   (same bug class as the RTL horizontal fix) and could only match
   backwards column jumps; that geometry is now fixed (no score changes
   with bridging off — ratchet-verified). But even with correct
   geometry, `EnableVerticalWrapBridging` (bridge column wraps when
   vertical is detected) measures net negative: tesseract vertical
   74.2→63.2 / 66.5→62.4. Partially-matched per-character chains — every
   CJK char is a high-frequency duplicate, so cross-column pathfinding
   picks wrong instances — displace the emit-order + leftover-assembly
   fallback that currently carries these fixtures. Vertical *matching*
   needs better per-character chaining (e.g. n-gram units) before
   bridging can pay; the flag stays available for that experiment. Do
   not re-enable it in the CJK preset from these numbers.
   Update 2026-07-17 (later): **realistic vertical archetype added**
   (`japanese-book-vertical`, testdoc layout "book"): a true tategaki
   page — vertical title column, right-to-left body, vertical colophon,
   no horizontal bands (the old vertical fixtures wrap a vertical body
   in Western title/footer bands, which real vertical documents don't
   have). Scores on it: pdftext 85.6, tesseract 77.1, apple ~0. Also
   settles the OCR-side orientation-hint question on fair ground:
   Tesseract `--psm 5` ("single uniform block of vertical text") loses
   to default auto-segmentation even on this uniform vertical page
   (71.4 vs 77.1; on the banded fixtures it was 44–47 vs 74) — the
   effective vertical hint is the jpn_vert model, not the PSM.
   `--psm 1` matched default on single-vertical and gained +1.8 on
   two-column-vertical. Apple Vision has no orientation parameter at
   all. (tesseract-util now has a `-psm` passthrough flag.)
   Update 2026-07-17 (later still): **vertical leftover assembly fixed —
   every vertical row improved, nothing else moved.** The leftover
   pipeline ordered sentence fragments by top position and measured
   paragraph gaps vertically — both wrong under a vertical reading
   order, and vertical pages ride this pipeline entirely (lines_found=0).
   Fixes (reading_axis.go): reading-order-aware sentence ordering
   (column-first with top tiebreak) and paragraph-gap insertion. Plus a
   line-id-less orientation fallback (detectVerticalFlow: emit-order
   flow with an aspect guard) so PDF text layers — which carry no line
   ids — get vertical detection at all. Gains: book-vertical/pdftext
   85.6→98.1, single-vertical/pdftext 76.0→84.3, two-col-vertical/
   tesseract 66.5→87.1, single-vertical/tesseract 74.2→78.5,
   two-col-vertical/pdftext 63.3→67.0. One counter-measurement:
   switching AssembleContiguousLines' paragraph check to the advance
   axis (horizontal gaps for vertical text) measured −21 on
   single-vertical/tesseract — tesseract's vertical line ids span
   columns and splitting at column wraps shreds sentences — so that
   check deliberately stays on the vertical axis (noted in code).
5. **Suite blind spot: no fixture exercises #1/#2.** Every generated
   document is long unique paragraphs. Before fixing the above, add a
   product-tile/grid archetype to `cmd/testdoc` (short repeated lines like
   "Learn more"/"Buy" under distinct headlines) so the fix ratchets.

Status update (2026-07-06): #1+#2 fixed (`EnableShortLineAnchoring`), #3
fixed (`EnableWrapBridging`), #5 exists (`english-grid`, 75.9/69.5 before →
99.3/98.6 after). **The per-language error-tolerance config was PROMOTED
into `ConfigForLanguage` (mean +4.65)**: Latin/Hindi get wrap bridging +
chain holes + anchoring + reconciliation (single-word rescue); Arabic gets
anchoring only; CJK gets holes only. Five baselines were deliberately
hand-lowered as the trade (worst hindi-single −3.79 vs gains up to +40.8);
vs prior committed baselines the only net regression is english-three-column
−0.59. Mechanism B (approx chain fallback) measured subsumed everywhere and
was deleted. Only #4 (vertical detection) remains.

Status update (2026-07-15): **wrap bridging promoted for Arabic** after
fixing an RTL bug in the wrap classifier: `isWrappedToNextLine` only
recognized the LTR wrap shape (`w1.Right() > w2.Left()`), so for RTL text
a legitimate wrap — line ends at page left, next line starts at page
right — was never classified as one. Every earlier "wrap bridging
misfires on RTL (−9.5)" measurement was made under that bug and is void:
bridging could only ever admit junk steps for Arabic. Do not relitigate
wrap bridging for Arabic from pre-2026-07-15 numbers. With the
RTL-aware classifier + `EnableWrapBridging` in the Arabic config,
Arabic pathfinding matches lines for the first time (found went 0 →
most; scores previously rode entirely on emit order + the leftover
assembler): pdftext 84.2→100 / 59.3→97.9 / 46.3→89.3, Tesseract
multi-column +19.9/+10.1, Apple unchanged. One baseline hand-lowered
as the trade: arabic-single/tesseract 63.09→60.12 (noisy input; chain
holes re-measured as no rescue: −2.7 to −12.2).

Suggested order (historical): 5 → 1+2 together (+3 while in there) → 4.

## History worth knowing (so it isn't relitigated)

- **Fuzzy matching was removed entirely (2026-07-05).** It was an
  experiment; measured ≤0.3pt effect everywhere, destructively rewrote
  correctly-read OCR words with canonical lookalikes ("home"→"some"), and
  inflated scores by pasting canonical text into the output being measured.
  Removing it cut suite runtime 2.7x. The spelling-corrections subsystem
  (`EnableSpellingCorrections`, metadata-only suggestions) was deleted
  2026-07-07: measured zero effect (by design) and superseded by line
  repair, which writes positional — not similarity-based — correction
  metadata onto blocks. Don't reintroduce similarity-based matching
  without evidence it beats the positional/structural mechanisms; that
  bar has now been tested three times (fuzzy, gap-fill edit distance,
  corrections) and never met.
- **Old accuracy numbers (pre-2026-07 problems.todo, etc.) are untrustworthy**:
  the old harness rendered a title/footer absent from canonical, hardcoded
  612x792 page dims, and passed no language config.
- The tall-image slicer was originally the sibling `image-slicer` project
  via a go.mod `replace`, then inlined, and now comes from the published
  `github.com/goodblaster/vertigo/slicer` module.
- **Hand-built unit tests can pass for the wrong reason.** Until
  2026-11-13 a block with an empty `Extractor` made every pathfinding step
  cost zero, and seven tests built their blocks without one — they
  asserted on emit order, not sorting. If a geometry test passes
  surprisingly easily, check that its blocks have page dimensions and
  that the Arabic ones are actually laid out right to left.
