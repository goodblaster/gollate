package sorters

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Patterns and tables used by NormalizeText. Compiled once: NormalizeText
// runs for every OCR block and canonical line (and repeatedly during
// post-processing), and per-call compilation used to account for ~94% of
// all allocation in the integration suite.
var (
	unicodeEscapeRe = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)
	urlRe           = regexp.MustCompile(`https?://[^\s]+`)
	emailRe         = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)
	phoneRe         = regexp.MustCompile(`\+?[0-9]+(?:-[0-9]+)+`)
	whitespaceRunRe = regexp.MustCompile(`\s+`)
)

// decodeUnicodeEscapes converts escaped Unicode sequences (e.g., \u2019) to actual characters.
func decodeUnicodeEscapes(s string) string {
	if !strings.Contains(s, `\u`) {
		return s
	}
	return unicodeEscapeRe.ReplaceAllStringFunc(s, func(match string) string {
		hex := match[2:]
		code, err := strconv.ParseInt(hex, 16, 32)
		if err != nil {
			return match
		}
		return string(rune(code))
	})
}

// normalizeToASCII converts Unicode text to ASCII while preserving key punctuation.
func normalizeToASCII(s string) string {
	// Handle special cases before normalization
	// German ß (eszett/sharp s) -> ss
	s = strings.ReplaceAll(s, "ß", "ss")
	s = strings.ReplaceAll(s, "ẞ", "SS")

	// Strip combining marks: decompose, drop Mn, recompose. Stateless
	// (safe for concurrent sorters) and equivalent to the transform.Chain
	// this replaced.
	result := norm.NFC.String(strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, norm.NFD.String(s)))
	// Typographic apostrophe (U+2019) -> ASCII, so "Apple's" normalizes the
	// same way regardless of which apostrophe the source used. (This line
	// historically replaced ' with itself - a no-op - which made curly-quote
	// possessives split into two tokens: "apple s".)
	result = strings.ReplaceAll(result, "’", "'")
	return result
}

// initialismRe matches any occurrence of two or more letter+period groups.
var initialismRe = regexp.MustCompile(`((?:[A-Za-z]\.){2,})`)

// replacements are the characters NormalizeText turns into spaces.
// '_' is handled specially in NormalizeText to protect initialisms/emails/phones.
// Includes Western and CJK (Chinese/Japanese/Korean) punctuation for language-agnostic processing
var replacements = map[rune]struct{}{
	// Western punctuation
	'-': {}, '.': {}, ',': {}, ':': {}, ';': {}, '!': {}, '?': {},
	'\u201C': {}, '\u201D': {}, '\u2018': {}, '\u2019': {}, '"': {}, '\u2014': {}, '\u2013': {},
	'\u2026': {}, '\u00B7': {}, '\u2022': {}, '\u00AB': {}, '\u00BB': {}, '\u2012': {},
	'\u2015': {}, '\u2011': {}, '(': {}, ')': {}, '[': {}, ']': {}, '{': {}, '}': {},
	'|': {}, '@': {}, '#': {}, '*': {}, '&': {}, '+': {}, '=': {},
	'~': {}, '^': {}, '`': {}, '\u00B0': {}, '\u00A9': {}, '\u00AE': {}, '\u2122': {},
	'\u00BF': {}, '\u00A1': {},

	// Arabic-script and Devanagari punctuation. Without these a word kept
	// its comma or full stop glued on ("سبت،", "है।") and only matched an
	// OCR token carrying the identical mark - one misread comma and the
	// word was absent.
	'\u060C': {}, // ، Arabic comma
	'\u061B': {}, // ؛ Arabic semicolon
	'\u061F': {}, // ؟ Arabic question mark
	'\u06D4': {}, // ۔ Arabic full stop (Urdu)
	'\u0964': {}, // । Devanagari danda
	'\u0965': {}, // ॥ Devanagari double danda
	'\u2010': {}, // ‐ Hyphen (what PDF text layers carry)

	// CJK punctuation (Chinese/Japanese/Korean)
	'\u3002': {}, // 。 Ideographic full stop
	'\uFF0C': {}, // ， Fullwidth comma
	'\uFF01': {}, // ！ Fullwidth exclamation mark
	'\uFF1F': {}, // ？ Fullwidth question mark
	'\uFF1B': {}, // ； Fullwidth semicolon
	'\uFF1A': {}, // ： Fullwidth colon
	'\u3001': {}, // 、 Ideographic comma
	'\uFF08': {}, // （ Fullwidth left parenthesis
	'\uFF09': {}, // ） Fullwidth right parenthesis
	'\u300A': {}, // 《 Left double angle bracket
	'\u300B': {}, // 》 Right double angle bracket
	'\u300C': {}, // 「 Left corner bracket
	'\u300D': {}, // 」 Right corner bracket
	'\u300E': {}, // 『 Left white corner bracket
	'\u300F': {}, // 』 Right white corner bracket
	'\u3010': {}, // 【 Left black lenticular bracket
	'\u3011': {}, // 】 Right black lenticular bracket
	'\uFF5B': {}, // ｛ Fullwidth left curly bracket
	'\uFF5D': {}, // ｝ Fullwidth right curly bracket
}

// NormalizeText performs text normalization but preserves URLs, email addresses,
// and phone numbers. It converts initialisms so that their periods become underscores,
// phone numbers like "+1-800-425-1267" become "1_800_425_1267", and email addresses are
// preserved with their underscores intact.
func NormalizeText(s string) string {
	// Decode Unicode escape sequences.
	s = decodeUnicodeEscapes(s)

	// Protect URLs, email addresses and phone numbers from the punctuation
	// handling below by swapping each for a placeholder, restored later.
	// Each match gets its own numbered placeholder in a single pass; the
	// trailing "z" keeps "…1z" from being a prefix of "…10z", which used to
	// corrupt strings with ten or more of one kind (restoration ran in map
	// order, so the damage was also random).
	var urls, emails, phones []string
	s = urlRe.ReplaceAllStringFunc(s, func(url string) string {
		urls = append(urls, url)
		return fmt.Sprintf("urlplaceholder%dz", len(urls)-1)
	})
	s = emailRe.ReplaceAllStringFunc(s, func(email string) string {
		// Wrap with markers to protect underscores.
		emails = append(emails, "qzemailinitqz"+strings.ToLower(email)+"qzemailendqz")
		return fmt.Sprintf("emailplaceholder%dz", len(emails)-1)
	})
	s = phoneRe.ReplaceAllStringFunc(s, func(phone string) string {
		transformed := strings.ReplaceAll(strings.TrimPrefix(phone, "+"), "-", "_")
		phones = append(phones, "qzphoneinitqz"+transformed+"qzphoneendqz")
		return fmt.Sprintf("phoneplaceholder%dz", len(phones)-1)
	})

	// Normalize Unicode to ASCII.
	s = normalizeToASCII(s)

	// Preprocess initialisms:
	// For any match (e.g. "U.S.A.", "D.C.", "p.m."), if it contains at least two periods,
	// convert it to lowercase, replace periods with underscores, and wrap with markers.
	s = initialismRe.ReplaceAllStringFunc(s, func(match string) string {
		if strings.Count(match, ".") < 2 {
			return match
		}
		converted := strings.ReplaceAll(strings.ToLower(match), ".", "_")
		if strings.HasSuffix(converted, "_") {
			converted = converted[:len(converted)-1]
		}
		return "qzinitqz" + converted + "qzendqz"
	})

	var builder strings.Builder
	inWord := false
	for _, r := range s {
		if r == '\'' {
			// Skip apostrophes entirely for better OCR matching
			// (OCR engines are inconsistent with apostrophes)
			continue
		} else if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
			builder.WriteRune(unicode.ToLower(r))
			inWord = true
		} else if _, found := replacements[r]; found {
			if inWord {
				builder.WriteRune(' ')
				inWord = false
			}
		} else if unicode.IsSpace(r) {
			if inWord {
				builder.WriteRune(' ')
				inWord = false
			}
		} else {
			builder.WriteRune(r) // Preserve any unrecognized characters.
		}
	}

	// Trim and collapse spaces.
	result := strings.TrimSpace(builder.String())
	result = whitespaceRunRe.ReplaceAllString(result, " ")

	// Restore phone and email placeholders BEFORE token post-processing.
	for i, val := range phones {
		result = strings.Replace(result, fmt.Sprintf("phoneplaceholder%dz", i), val, -1)
	}
	for i, val := range emails {
		result = strings.Replace(result, fmt.Sprintf("emailplaceholder%dz", i), val, -1)
	}

	// Post-process tokens:
	// Tokens marked as initialisms, phone numbers, or email addresses have their markers stripped,
	// preserving underscores. For other tokens, underscores are converted to spaces.
	tokens := strings.Split(result, " ")
	for i, token := range tokens {
		if strings.Contains(token, "qzinitqz") {
			token = strings.ReplaceAll(token, "qzinitqz", "")
			token = strings.ReplaceAll(token, "qzendqz", "")
			tokens[i] = token
		} else if strings.Contains(token, "qzphoneinitqz") {
			token = strings.ReplaceAll(token, "qzphoneinitqz", "")
			token = strings.ReplaceAll(token, "qzphoneendqz", "")
			tokens[i] = token
		} else if strings.Contains(token, "qzemailinitqz") {
			token = strings.ReplaceAll(token, "qzemailinitqz", "")
			token = strings.ReplaceAll(token, "qzemailendqz", "")
			tokens[i] = token
		} else {
			tokens[i] = strings.ReplaceAll(token, "_", " ")
		}
	}
	result = strings.Join(tokens, " ")

	// Collapse any multiple spaces created by underscore conversion
	result = whitespaceRunRe.ReplaceAllString(result, " ")

	// Restore URL placeholders AFTER token post-processing.
	for i, url := range urls {
		result = strings.Replace(result, fmt.Sprintf("urlplaceholder%dz", i), url, -1)
	}

	return result
}
