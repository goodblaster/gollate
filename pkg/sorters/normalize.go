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

	// Extract URLs before processing.
	urls := urlRe.FindAllString(s, -1)
	urlPlaceholders := make(map[string]string)
	for i, url := range urls {
		// Use lower-case placeholder keys.
		placeholder := fmt.Sprintf("urlplaceholder%d", i)
		urlPlaceholders[placeholder] = url
		s = strings.Replace(s, url, placeholder, -1)
	}

	// Extract email addresses before processing.
	emails := emailRe.FindAllString(s, -1)
	emailPlaceholders := make(map[string]string)
	for i, email := range emails {
		placeholder := fmt.Sprintf("emailplaceholder%d", i)
		// Wrap with markers to protect underscores.
		emailPlaceholders[placeholder] = "qzemailinitqz" + strings.ToLower(email) + "qzemailendqz"
		s = strings.Replace(s, email, placeholder, -1)
	}

	// Extract phone numbers before processing.
	phones := phoneRe.FindAllString(s, -1)
	phonePlaceholders := make(map[string]string)
	for i, phone := range phones {
		placeholder := fmt.Sprintf("phoneplaceholder%d", i)
		transformed := phone
		if strings.HasPrefix(transformed, "+") {
			transformed = transformed[1:]
		}
		transformed = strings.ReplaceAll(transformed, "-", "_")
		phonePlaceholders[placeholder] = "qzphoneinitqz" + transformed + "qzphoneendqz"
		s = strings.Replace(s, phone, placeholder, -1)
	}

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
	for placeholder, val := range phonePlaceholders {
		result = strings.Replace(result, placeholder, val, -1)
	}
	for placeholder, val := range emailPlaceholders {
		result = strings.Replace(result, placeholder, val, -1)
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
	for placeholder, url := range urlPlaceholders {
		result = strings.Replace(result, placeholder, url, -1)
	}

	return result
}
