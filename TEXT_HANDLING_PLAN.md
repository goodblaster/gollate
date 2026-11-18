# Language, direction, and OCR continuation handling

Follow-up backlog from the Meditations OCR work, 2026-11-17. These items
describe future work, not capabilities implemented by the current change.

## Current limitations

The new `wrappedSuccessors` and `hyphenContextAgrees` helpers in
`pkg/sorters/hyphen_geometry.go` assume horizontal left-to-right text. They
do not receive `ReadingOrder`; normalization also runs before vertical
direction detection. Existing multilingual fixtures pass, but do not
establish RTL or vertical correctness for these helpers. Highlight
smoothing also needs a direction-aware review.

Language and reading direction are separate concerns. Mirroring English
hyphen recovery is not sufficient: ordinary Arabic and Persian text wraps
between words, whereas Uyghur permits hyphenation despite sharing the
Arabic script. Hebrew maqaf can connect words and must not automatically
be discarded at a wrap. Embedded Latin text needs its own rules within an
RTL paragraph.

## Prioritized work

1. **Make geometric recovery respect supported directions.** Pass the
   reading-order abstraction to normalization, or conservatively gate
   unsupported geometry until direction is available. Audit line starts,
   line ends, column progression, cross-page continuation, and smoothing.
   Avoid inferring the direction of every text run from the page alone.
2. **Separate continuation policy from geometry.** Use language/script
   information and canonical context to distinguish discretionary wrap
   marks from lexical hyphens, maqaf, dashes, and minus signs. Preserve
   punctuation when the reference requires it. Unknown language or
   ambiguous evidence should not force a join. Include Uyghur as a case
   where Arabic-script hyphenation is meaningful.
3. **Recover OCR splits without a hyphen.** Consider original OCR emission
   order first as evidence, together with canonical gap context and nearby
   geometry. Support fragments separated by OCR segmentation errors; do
   not assume every visible gap separates words. Competing occurrences or
   intervening text must prevent an unsupported join.
4. **Handle joining controls and elongation during matching.** Cover ZWNJ
   (half-space), ZWJ, contextual letter forms, ligatures, combining marks,
   and tatweel. Preserve original text and boxes; do not indiscriminately
   remove meaningful characters. Tatweel normally extends a connection,
   while some language-specific hyphenation uses a disconnected
   kashida-like mark. Appearance alone cannot distinguish their roles.
5. **Handle mixed-direction passages.** Test logical versus visual OCR
   order with Arabic/Hebrew, Latin words, numbers, brackets, and punctuation.
   Preserve canonical numbering across line, column, and page boundaries.
6. **Extend context-supported spelling repair cautiously.** Use anchored
   gaps and original OCR order to nominate misspelled candidates. Require
   evidence for the canonical occurrence, preserve original OCR spelling
   and geometry, and leave ambiguous candidates unresolved. Measure false
   corrections as well as recovered words for each language.

## Acceptance fixtures

- Arabic and Persian ordinary wraps: no invented intraword hyphen joins.
- Uyghur genuine hyphenation, plus lookalike punctuation that must remain.
- Hebrew maqaf and lexical compounds, including line boundaries.
- Persian ZWNJ and OCR that replaces it with a space or omits it.
- Tatweel, disconnected continuation marks, ligatures, and diacritics.
- RTL paragraphs containing hyphenated Latin words and numbers.
- Interleaved columns, repeated suffixes, ambiguous canonical contexts,
  missing line IDs, cross-page splits, and vertical text.
- Positive and negative cases asserting canonical occurrence, original
  text, both continuation boxes, and complete outer highlight boundaries.

Retain the existing English regression cases. For each new corpus, report
ordering accuracy, precision, canonical coverage, unlinked words, false
joins/corrections, and stage timings. Passing unrelated multilingual
fixtures is not a substitute for these targeted cases.

## References

- [W3C: Approaches to line breaking](https://www.w3.org/International/articles/typography/linebreak.en.html)
- [W3C: Arabic & Persian Layout Requirements](https://www.w3.org/TR/alreq/)
- [Hebrew orthography notes](https://r12a.github.io/scripts/hebr/he.html)
- [Unicode UAX #14: Line Breaking Algorithm](https://www.unicode.org/reports/tr14/)

Consult current Unicode guidance when implementing character-specific
rules; older descriptions of Hebrew line-breaking rules can differ.
