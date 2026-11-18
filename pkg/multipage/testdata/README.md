# Adversarial multi-page inputs

`challenges.json` contains small authored canonical texts and explicitly
positioned page fragments. `challenge_test.go` turns them into synthetic
word blocks and records the expected canonical occurrence, page and box
before sorting. No page assignments or expected locations are given to the
sorter. Japanese inputs use character blocks.

Cases cover repeated body text, canonical text in margins, compounds at a
page break, paragraphs spanning several pages, blank pages, and mixed
Japanese/Latin text. The Go test also generates a 400-page edition-drift
case and a five-content-page paragraph with misreads and dropped words.
It verifies missing occurrences separately, including empty input.

Run `go test ./pkg/multipage -run Challenge -v` for accuracy and location
scores. See `TESTING.md` at the repository root for before/after results,
benchmark commands, and the distinction from the committed OCR fixtures.
