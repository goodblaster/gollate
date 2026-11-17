# README reading-order example

`reading-order.png` is a small, intentionally committed illustration. Full
books, page renders, OCR data and diagnostic image series remain ignored.

The image uses actual Apple Vision OCR and gollate's saved canonical links
for words 8786–8869 of the pinned George Long *Meditations* corpus. It places
the bottom of the left column beside the top of the right column on page 18.
Coordinates within each crop are preserved. Green and purple identify two
sentences; `8805` and `8805+` locate the two pieces of “capacity”. No word
positions or matches are invented for the illustration.

Rebuild from the repository root:

```sh
python3 -B scripts/meditations-corpus.py --skip-overlays
go run ./cmd/readme-preview
```

The generator fails if the selected words or the cross-column continuation
are missing, so a future OCR change cannot silently produce a misleading
example. Source provenance and test procedure:
[Meditations corpus](../../testdata/books/meditations/README.md).
