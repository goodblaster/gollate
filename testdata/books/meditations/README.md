# Meditations: real-book OCR evaluation

Downloaded the George Long translation from [Public Domain Library](https://publicdomainlibrary.org/en/ebooks/meditations#formats). The repository keeps the setup script, tests, this procedure and small accuracy thresholds. The EPUB, extracted book text, PDFs, images, OCR blocks, measurements and sorted results are local generated data and are not committed.

The corpus contains all twelve books and 122 endnotes: 49,884 whitespace-separated words, 681 canonical lines, and 50,134 normalized scoring units. Cover art, publisher notices and navigation backlinks are excluded. Greek quotations, verse, citations, superscript references and long paragraphs are retained. Endnote list numbers appear explicitly in both reference and rendering. The reference comes from EPUB XHTML in spine order, independently of PDF extraction and OCR. A separate HTML-body validator checks that visible source text was not omitted; this caught five standalone citations during preparation.

## Run from a fresh checkout

Prerequisites: macOS (Apple Vision), Go 1.24.4 or newer, Python 3, Google Chrome, Poppler (`pdftoppm`, `pdftotext`) and Tesseract available on `PATH`. The default Chrome path is `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`; override it with `--chrome /path/to/chrome` if needed. The first run needs network access for the EPUB and any missing Go modules.

From the repository root:

```sh
python3 -B scripts/meditations-corpus.py --jobs 4
```

This single command:

1. Downloads the pinned EPUB into `experiments/meditations/source/` and verifies its SHA-256 before publishing the file.
2. Extracts the reference and prepares both layouts.
3. Produces the PDFs and every rasterized page.
4. Builds missing OCR utilities and runs Apple Vision, Tesseract and the PDF text control.
5. Runs the six book accuracy tests against the current code, enforcing the committed baselines.
6. Produces the numbered and sentence-sample overlay series from the saved results.

Run the same command again to resume or verify the corpus. Content hashes cover inputs and outputs: valid downloads, renders, OCR pages and overlays are reused; missing or changed artifacts are regenerated. OCR checkpoints are saved per completed page. Accuracy tests always run, even when preparation is fully cached. Cached stages preserve their original timing records. Interrupted stages do not count as complete. The script never automatically updates accuracy baselines.

Overlay PNGs use about 2.2 GB for this edition; add `--skip-overlays` to prepare and test without them. Use `--out experiments/meditations-other` for a separate corpus. Tool upgrades, recognition-language changes or font changes require a fresh output directory to measure their effects; existing caches intentionally retain the prior rendered/OCR inputs. To rebuild existing utility binaries after changing their source, run `make utils`.

## Local artifacts and method

All new downloaded/generated material lives under the ignored output directory. With the default output path:

| Path under `experiments/meditations/` | Contents |
|---|---|
| `source/meditations.epub` | Original, checksum-verified download |
| `source/canonical.txt`, `source/canonical-map.json`, `source/provenance.json` | Reference text, source locations and provenance |
| `single-column/document.pdf` | Single-column PDF (103 pages in the recorded run) |
| `double-column/document.pdf` | Double-column PDF (102 pages in the recorded run) |
| `<layout>/page-N.png` | Original page images |
| `<layout>/<source>-N.json` | OCR or PDF text blocks |
| `<layout>/current/` | Sorted results, paragraph failures and overlays |
| `report-current.json` | Current accuracy and timing measurements |

The small [regression baselines](baselines.json) are committed. Legacy `source/`, `renders/` and measurement files under this testdata directory are also ignored, so existing local copies cannot be accidentally included by `git add .`.

Both versions use 7-by-9-inch pages, 11-point Georgia, 1.35 line spacing, running titles and page numbers. Chrome printed the HTML; Poppler rasterized every page at 144 dpi (1008 by 1296 pixels). Apple Vision and Tesseract each recognized all 205 images: 410 OCR page runs. Both used the existing utilities with slicing disabled and default recognition settings. A third input uses Poppler PDF text blocks as a control; it is not an OCR result.

The full generated data lives in the ignored `experiments/meditations/` directory: HTML, PDFs, every PNG, OCR/control blocks, logs, sorted text/JSON, and per-paragraph failures. Each layout has `current/` evaluation artifacts; `before/` is present only if a separate A/B run was performed. This is real published prose freshly typeset into test pages, not a historical scan with stains, skew or damaged type.

## Accuracy

Order accuracy is normalized-token longest-common-subsequence length divided by reference length. Raw order follows page then extractor emission order; sorted order uses gollate output. Precision divides the same matches by output length. These scores are not character-level OCR accuracy and do not score punctuation or typography.

| Layout | Input | Raw order | Sorted order | Sorted precision | Mean paragraph recovery |
|---|---|---:|---:|---:|---:|
| Single | Apple Vision | 98.586% | 99.422% | 99.507% | 98.168% |
| Single | Tesseract | 98.446% | 99.286% | 99.504% | 98.052% |
| Double | Apple Vision | 92.466% | 98.460% | 98.247% | 97.957% |
| Double | Tesseract | 97.303% | 98.366% | 98.535% | 97.680% |
| Single | PDF text control | 99.078% | 99.922% | 99.866% | 99.855% |
| Double | PDF text control | 67.938% | 99.047% | 98.922% | 99.694% |

The previous committed code (`4a97ff2`) and the working tree after the multi-page changes produce **identical accuracy, precision, paragraph recovery and canonical-span coverage on all six inputs**. The earlier changes improve speed here, but do not improve book accuracy. Matching code was not tuned during this evaluation.

The report also records input bag recall and canonical-span coverage. Dehyphenation can make sorted accuracy exceed raw bag recall. Span coverage counts distinct reference units linked to matching output text; it does not independently verify physical page coordinates. Mean paragraph recovery weights short and long canonical lines equally.

## Speed

The following historical measurements used Apple M4 Pro, Go 1.24.4, macOS. Fresh runs save timings in each layout’s `timing.json` and in `report-current.json`. OCR used four concurrent page workers and one Tesseract thread per process. Times below are whole-book wall times.

| Layout / input | OCR | Previous sort | Current sort | Sort time reduction |
|---|---:|---:|---:|---:|
| Single / Apple | 23.48 s | 2.914 s | 1.912 s | 34.4% |
| Single / Tesseract | 22.58 s | 2.945 s | 1.927 s | 34.6% |
| Double / Apple | 25.63 s | 3.083 s | 2.211 s | 28.3% |
| Double / Tesseract | 23.95 s | 3.083 s | 2.212 s | 28.3% |

PDF-control sorting took 1.890 s single-column and 2.186 s double-column, down from 2.905 s and 3.074 s. Rendering plus rasterization took 21.93 s and 26.79 s respectively. Scoring took about 0.13 s per input, outside sort timing. These are single A/B runs on identical saved input, not repeated benchmark medians; small timing differences are not significant.

## Failures exposed

The 884-word Book I paragraph beginning “In my father” (canonical line index 19, file line 20) spans physical pages 3–5 in the double-column PDF. All three inputs assign it only to pages 3–4. Its recovery is 62.32% for Apple, 60.52% for Tesseract and 60.97% for PDF text; the third-page tail is not correctly linked into the paragraph. This remains a real multi-page segmentation failure despite the synthetic three-plus-page tests passing. The last sentence ends with “the illness of Maximus” on page 5.

Other concentrated failures involve Greek quotations/endnotes, small note references, and the paragraph-reference symbol ¶. For example, Tesseract joins the double-column heading “Book I” into “BookI”. The PDF control's remaining errors show that recognition alone does not explain the full accuracy gap. Per-paragraph failure files preserve reference and recovered text for inspection.

## Reproduce

### Numbered page overlays

Every saved result can produce two image series:

- `current/<source>-overlays/order/page-NNN.png`: boxes around every final output token, labeled with its **one-based canonical word number across the whole book**.
- `current/<source>-overlays/spotcheck/page-NNN.png`: the same labels for up to two randomly selected sentences per page, colored green and purple. The default seed is 117, so reruns with the same input choose the same samples.

For example, inspect `<layout>/current/apple-overlays/order/page-006.png` and `spotcheck/page-006.png` after setup. These PNGs are generated locally, not checked in.

Numbering counts whitespace-separated canonical words, including the title, headings and note references: the complete book has 49,884 word IDs. Compounds count as one word, so multiple matcher tokens can share a number. `117+` identifies a continuation of word 117 on another line or page. A number range means one output token maps to several canonical words. Red `U117` means an unlinked output token at **output position** 117; it does not claim to be canonical word 117. Removed running headers and page numbers are not numbered.

Images preserve the page layout at twice the source resolution, with the legend and sampled source text beside the page. They display the final sorter's existing boxes and links, including mistakes; they do not realign the text or invent coordinates for missing words. `manifest.json` contains source words and byte spans, sentence text, per-page boxes, final output token order, sampled sentence IDs and all unlinked canonical word IDs. A word counts as linked if any output token overlaps its source span; this is not a recognition-accuracy score.

Sentence splitting uses punctuation and paragraph boundaries. Samples come from sentences with at least one canonical-linked word on the page, without requiring full recognition. Cross-page sentences may be only partly highlighted, and pages without links have no sample. This is a visual review aid rather than an independent or unbiased accuracy audit.

Generate from the existing saved results without rerunning OCR or sorting:

```sh
go run ./cmd/book-overlays -corpus experiments/meditations
```

The default generates both views for every page in both layouts, for Apple, Tesseract and the PDF text control: **1,230 PNGs**. Large overlays stay in the ignored experiment directory. Use `-layouts double-column -sources apple -pages 1,6,102` for a preview; omit `-pages` for the complete series. `-seed 42 -sentences 1` changes sampling. `-label before` renders another saved evaluation. Previews replace only the requested PNGs, so use a full render after changing the seed to keep the whole series consistent.

To generate overlays as part of evaluation (outside the recorded sort/scoring timing):

```sh
BOOK_CORPUS="$PWD/experiments/meditations" BOOK_OVERLAYS=1 go test ./integration -run '^TestBookCorpus$' -v -count=1
```

`make build utils` also builds `bin/book-overlays`. Overlay-specific tests cover canonical word 117, UTF-8 byte offsets, blank source lines, compounds, cross-page continuations, unmapped tokens, deterministic sampling and PNG coordinate placement.

### Individual stages and tests

`--stage prepare`, `render`, `ocr`, `evaluate` and `overlays` run one stage; the default `all` runs them in that order. A stage assumes its preceding stages are ready. For example, rerun accuracy without rendering or OCR:

```sh
python3 -B scripts/meditations-corpus.py --stage evaluate
# Equivalent direct invocation:
BOOK_CORPUS="$PWD/experiments/meditations" go test ./integration -run '^TestBookCorpus$' -v -count=1
```

Run the lightweight checks without downloading the book or installing OCR/rendering tools:

```sh
python3 -B -m unittest discover -s scripts -p test_meditations_corpus.py -v
go test ./...
go vet ./...
```

The Python tests use a tiny authored EPUB and mocked external tools. They check source extraction, atomic/checksummed downloads, repeat-run caching, repair of missing files, and invalidation when inputs change. They do not read any local real-book artifact. Go overlay tests check numbering and coordinates; the exact book LCS scorer is checked against the existing quadratic scorer.

`TestBookCorpus` is opt-in; ordinary `go test ./...` skips the large corpus. Set `BOOK_LABEL` on the direct Go command to keep separate evaluation artifacts; `before` bypasses the current-code accuracy gate. Current-label scores enforce four accuracy metrics with a 0.05 percentage-point tolerance. Only an explicit `UPDATE_BASELINES=1` on a direct Go test command should ratchet passing accuracy thresholds upward; timings are descriptive and are not gates. Different operating-system OCR or rendering versions can change scores, so inspect failures before updating thresholds.
