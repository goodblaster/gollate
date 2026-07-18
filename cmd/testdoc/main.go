// Command testdoc generates OCR test documents.
//
// It renders canonical text through HTML/CSS layouts using headless Chrome,
// producing a matched set of artifacts in the output directory:
//
//	document.pdf   - the test document (for humans)
//	document.png   - 2x raster of the same page (input for OCR engines)
//	document.html  - the intermediate HTML (for debugging layout issues)
//	canonical.txt  - ground truth text in reading order, including title/footer
//	test-info.json - metadata (language, layout, direction, image dimensions)
//
// The PDF, PNG, and canonical text are all derived from the same in-memory
// document structure, so ground truth cannot drift from what is rendered.
// Generation fails if content overflows the single fixed-size page, which
// prevents the historical problem of text overwriting or spilling off-page.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// Page geometry: US Letter at 96 CSS px/inch, rendered at 2x for OCR quality.
const (
	pageWidthPx  = 816  // 8.5in
	pageHeightPx = 1056 // 11in
	renderScale  = 2.0
)

type spec struct {
	Language    string
	Layout      string // single, two-column, three-column, mixed-sizes, sidebar
	Direction   string // ltr, rtl, vertical
	Title       string
	Footer      string
	FontSizePt  int
	LineSpacing float64
	ColumnGapPt int
	Paragraphs  []string
}

type testInfo struct {
	Language  string `json:"language"`
	Layout    string `json:"layout"`
	Direction string `json:"direction"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

var titles = map[string]string{
	"english":  "The Fabric of Everyday Life",
	"spanish":  "El Tejido de la Vida Cotidiana",
	"chinese":  "日常生活的织锦",
	"japanese": "日々の暮らしの織物",
	"arabic":   "نسيج الحياة اليومية",
	"hindi":    "दैनिक जीवन का ताना-बाना",
}

var headings = map[string][3]string{
	"english":  {"Introduction", "Observations", "Conclusion"},
	"spanish":  {"Introducción", "Observaciones", "Conclusión"},
	"chinese":  {"介绍", "观察", "结论"},
	"japanese": {"はじめに", "観察", "結論"},
	"arabic":   {"مقدمة", "ملاحظات", "خاتمة"},
	"hindi":    {"परिचय", "अवलोकन", "निष्कर्ष"},
}

// gridTiles is the product-tile content for the "grid" layout (issue #5 in
// TESTING.md): distinct headlines and descriptions over heavily repeated
// short action lines, the archetype where duplicate-line anchoring matters.
// Hardcoded (English only) so ground truth never drifts with content files.
var gridTiles = [12][2]string{
	{"Aurora Lamp", "Soft northern light for any room."},
	{"Breeze Fan", "Quiet airflow with adaptive speed."},
	{"Cascade Kettle", "Pour-over precision at exact temperatures."},
	{"Drift Pillow", "Cooling foam that shapes to you."},
	{"Ember Heater", "Focused warmth without the noise."},
	{"Flux Charger", "One pad for every device."},
	{"Glide Mouse", "Effortless tracking on any surface."},
	{"Halo Speaker", "Room-filling sound in a small ring."},
	{"Iris Camera", "Sharp detail from dawn to dusk."},
	{"Juno Clock", "Sunrise alarms tuned to your sleep."},
	{"Kite Router", "Fast mesh coverage for the whole home."},
	{"Lumen Torch", "Pocket light with a week of power."},
}

var fontStacks = map[string]string{
	"english":  `"Helvetica Neue", Arial, sans-serif`,
	"spanish":  `"Helvetica Neue", Arial, sans-serif`,
	"chinese":  `"PingFang SC", "Hiragino Sans GB", sans-serif`,
	"japanese": `"Hiragino Sans", "Hiragino Kaku Gothic ProN", sans-serif`,
	"arabic":   `"Geeza Pro", "Arial Unicode MS", sans-serif`,
	"hindi":    `"Kohinoor Devanagari", "Devanagari MT", sans-serif`,
}

func main() {
	content := flag.String("content", "", "Input text file: paragraphs separated by blank lines (required)")
	outDir := flag.String("out", "", "Output directory (required)")
	lang := flag.String("lang", "english", "Language: english, spanish, chinese, japanese, arabic, hindi")
	layout := flag.String("layout", "single", "Layout: single, two-column, three-column, mixed-sizes, sidebar, grid")
	direction := flag.String("direction", "", "Text direction: ltr, rtl, vertical (default: by language)")
	title := flag.String("title", "", "Document title (default: by language)")
	footer := flag.String("footer", "", "Footer text (default: derived from lang/layout)")
	fontSize := flag.Int("font-size", 12, "Body font size in points")
	lineSpacing := flag.Float64("line-spacing", 1.5, "Line spacing multiplier")
	columnGap := flag.Int("column-gap", 24, "Column gap in points")
	timeout := flag.Duration("timeout", 60*time.Second, "Render timeout")
	flag.Parse()

	if *content == "" || *outDir == "" {
		flag.Usage()
		os.Exit(1)
	}

	raw, err := os.ReadFile(*content)
	if err != nil {
		fatal("reading content: %v", err)
	}
	paragraphs := splitParagraphs(string(raw))
	if len(paragraphs) == 0 {
		fatal("no paragraphs found in %s", *content)
	}

	s := spec{
		Language:    *lang,
		Layout:      *layout,
		Direction:   *direction,
		Title:       *title,
		Footer:      *footer,
		FontSizePt:  *fontSize,
		LineSpacing: *lineSpacing,
		ColumnGapPt: *columnGap,
		Paragraphs:  paragraphs,
	}
	if s.Direction == "" {
		if baseLang(s.Language) == "arabic" {
			s.Direction = "rtl"
		} else {
			s.Direction = "ltr"
		}
	}
	if s.Title == "" {
		s.Title = titles[baseLang(s.Language)]
	}
	if s.Footer == "" {
		s.Footer = fmt.Sprintf("OCR Test: %s (%s, %s)", s.Language, s.Layout, s.Direction)
	}

	doc, canonical := buildDocument(s)

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fatal("creating output dir: %v", err)
	}
	htmlPath := filepath.Join(*outDir, "document.html")
	if err := os.WriteFile(htmlPath, []byte(doc), 0644); err != nil {
		fatal("writing HTML: %v", err)
	}
	if err := os.WriteFile(filepath.Join(*outDir, "canonical.txt"), []byte(strings.Join(canonical, "\n")+"\n"), 0644); err != nil {
		fatal("writing canonical: %v", err)
	}

	pdfData, pngData, err := render(htmlPath, *timeout)
	if err != nil {
		fatal("rendering: %v", err)
	}
	if err := os.WriteFile(filepath.Join(*outDir, "document.pdf"), pdfData, 0644); err != nil {
		fatal("writing PDF: %v", err)
	}
	if err := os.WriteFile(filepath.Join(*outDir, "document.png"), pngData, 0644); err != nil {
		fatal("writing PNG: %v", err)
	}

	info := testInfo{
		Language:  s.Language,
		Layout:    s.Layout,
		Direction: s.Direction,
		Width:     int(pageWidthPx * renderScale),
		Height:    int(pageHeightPx * renderScale),
	}
	infoData, _ := json.MarshalIndent(info, "", "  ")
	if err := os.WriteFile(filepath.Join(*outDir, "test-info.json"), append(infoData, '\n'), 0644); err != nil {
		fatal("writing test-info: %v", err)
	}

	fmt.Printf("Generated %s: %d paragraphs, layout=%s direction=%s\n", *outDir, len(paragraphs), s.Layout, s.Direction)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "testdoc: "+format+"\n", args...)
	os.Exit(1)
}

// baseLang maps a language variant like "english-legal" to its base
// ("english") for title/heading/font lookups, so archetype fixtures can
// carry a descriptive language name without needing their own assets.
func baseLang(lang string) string {
	return strings.SplitN(lang, "-", 2)[0]
}

func splitParagraphs(text string) []string {
	var out []string
	for _, p := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
		p = strings.TrimSpace(strings.Join(strings.Fields(p), " "))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// buildDocument produces the page HTML and the canonical lines (ground truth
// in human reading order) from the same structure. Canonical format: one line
// per text run, blank line between paragraphs.
func buildDocument(s spec) (string, []string) {
	var body strings.Builder
	var canonical []string

	addPara := func(sb *strings.Builder, text, class string) {
		fmt.Fprintf(sb, "<p class=%q>%s</p>\n", class, html.EscapeString(text))
		canonical = append(canonical, text, "")
	}

	// Title. The "book" layout is a realistic tategaki page: no horizontal
	// title/footer bands — the title is itself the first (rightmost)
	// vertical column and the attribution the last, so the whole page is
	// one vertical flow (the layout OCR engines actually meet in novels
	// and vertical letters).
	if s.Layout != "book" {
		body.WriteString(fmt.Sprintf("<div class=\"title\">%s</div>\n", html.EscapeString(s.Title)))
		canonical = append(canonical, s.Title, "")
	}

	contentClass := "content"
	if s.Layout == "two-column" || s.Layout == "three-column" {
		contentClass += " columns"
	}
	if s.Direction == "vertical" {
		contentClass += " vertical"
	}
	if s.Layout == "book" {
		contentClass += " book"
	}
	body.WriteString(fmt.Sprintf("<div class=%q>\n", contentClass))

	switch s.Layout {
	case "book":
		body.WriteString(fmt.Sprintf("<h1>%s</h1>\n", html.EscapeString(s.Title)))
		canonical = append(canonical, s.Title, "")
		for _, p := range s.Paragraphs {
			addPara(&body, p, "body")
		}
		body.WriteString(fmt.Sprintf("<p class=\"colophon\">%s</p>\n", html.EscapeString(s.Footer)))
		canonical = append(canonical, s.Footer)
	case "grid":
		// Product-tile grid: reading order is tile-major (left to right,
		// top to bottom). Content comes from gridTiles, not the input file.
		for _, tile := range gridTiles {
			body.WriteString("<div class=\"tile\">\n")
			body.WriteString(fmt.Sprintf("<h3>%s</h3>\n", html.EscapeString(tile[0])))
			canonical = append(canonical, tile[0])
			body.WriteString(fmt.Sprintf("<p class=\"desc\">%s</p>\n", html.EscapeString(tile[1])))
			canonical = append(canonical, tile[1])
			body.WriteString("<p class=\"action\">Learn more</p>\n<p class=\"action\">Buy</p>\n")
			canonical = append(canonical, "Learn more", "Buy", "")
			body.WriteString("</div>\n")
		}
	case "mixed-sizes":
		h := headings[baseLang(s.Language)]
		groups := splitGroups(s.Paragraphs, 3)
		for i, group := range groups {
			body.WriteString(fmt.Sprintf("<h2>%s</h2>\n", html.EscapeString(h[i])))
			canonical = append(canonical, h[i], "")
			for j, p := range group {
				class := "body"
				// One small-print paragraph per section exercises font-size variety.
				if j == len(group)-1 && len(group) > 1 {
					class = "small"
				}
				addPara(&body, p, class)
			}
		}
	case "sidebar":
		nSide := len(s.Paragraphs) / 4
		if nSide < 1 {
			nSide = 1
		}
		main := s.Paragraphs[:len(s.Paragraphs)-nSide]
		side := s.Paragraphs[len(s.Paragraphs)-nSide:]
		body.WriteString("<div class=\"main\">\n")
		for _, p := range main {
			addPara(&body, p, "body")
		}
		body.WriteString("</div>\n<aside class=\"side\">\n")
		for _, p := range side {
			addPara(&body, p, "body")
		}
		body.WriteString("</aside>\n")
	default: // single, two-column, three-column
		for _, p := range s.Paragraphs {
			addPara(&body, p, "body")
		}
	}
	body.WriteString("</div>\n")

	// Footer
	if s.Layout != "book" {
		body.WriteString(fmt.Sprintf("<div class=\"footer\">%s</div>\n", html.EscapeString(s.Footer)))
		canonical = append(canonical, s.Footer)
	}

	dirAttr := "ltr"
	if s.Direction == "rtl" {
		dirAttr = "rtl"
	}

	columnCount := 1
	switch s.Layout {
	case "two-column":
		columnCount = 2
	case "three-column":
		columnCount = 3
	}

	css := fmt.Sprintf(`
	@page { size: letter; margin: 0; }
	* { box-sizing: border-box; }
	html, body { margin: 0; padding: 0; }
	.page {
		width: %dpx; height: %dpx;
		padding: 1in;
		display: flex; flex-direction: column;
		font-family: %s;
		font-size: %dpt;
		line-height: %.2f;
	}
	.title { font-size: 2em; font-weight: bold; text-align: center; margin-bottom: 18pt; flex: none; }
	.content { flex: 1 1 auto; min-height: 0; }
	.content.columns { column-count: %d; column-gap: %dpt; column-fill: auto; height: 100%%; }
	.content.vertical { writing-mode: vertical-rl; }
	/* book: whole flow is vertical; spacing goes between columns (to the
	   left in vertical-rl), not below. Scoped to .book so the existing
	   vertical fixtures keep their committed rendering. */
	.content.book h1 { font-size: 1.6em; margin: 0 0 0 16pt; }
	.content.book p.colophon { font-size: 0.75em; font-style: italic; margin: 0 12pt 0 0; }
	p { margin: 0 0 10pt 0; }
	.content.book p { margin: 0 0 0 12pt; }
	p.small { font-size: 0.8em; }
	h2 { font-size: 1.4em; margin: 6pt 0 8pt 0; }
	.footer { flex: none; text-align: center; font-size: 0.75em; font-style: italic; margin-top: 8pt; }
`, pageWidthPx, pageHeightPx, fontStacks[baseLang(s.Language)], s.FontSizePt, s.LineSpacing, columnCount, s.ColumnGapPt)

	if s.Layout == "grid" {
		css += `
	.content { display: grid; grid-template-columns: repeat(3, 1fr); grid-auto-rows: 1fr; gap: 10pt; }
	.tile { border: 1px solid #ccc; border-radius: 6pt; padding: 8pt; text-align: center; }
	.tile h3 { font-size: 1.1em; margin: 0 0 4pt 0; }
	.tile .desc { font-size: 0.85em; margin: 0 0 6pt 0; }
	.tile .action { color: #0066cc; font-size: 0.9em; margin: 0 0 2pt 0; }
`
	}

	if s.Layout == "sidebar" {
		css += fmt.Sprintf(`
	.content { display: flex; gap: %dpt; }
	.main { flex: 2 1 0; min-width: 0; }
	.side { flex: 1 1 0; min-width: 0; font-size: 0.85em; }
`, s.ColumnGapPt)
	}

	doc := fmt.Sprintf(`<!DOCTYPE html>
<html dir=%q>
<head><meta charset="UTF-8"><style>%s</style></head>
<body><div class="page">
%s</div></body>
</html>
`, dirAttr, css, body.String())

	return doc, canonical
}

// splitGroups divides paragraphs into n roughly equal contiguous groups.
func splitGroups(paragraphs []string, n int) [][]string {
	if len(paragraphs) < n {
		n = len(paragraphs)
	}
	var groups [][]string
	per := len(paragraphs) / n
	rem := len(paragraphs) % n
	i := 0
	for g := 0; g < n; g++ {
		size := per
		if g < rem {
			size++
		}
		groups = append(groups, paragraphs[i:i+size])
		i += size
	}
	return groups
}

type overflowResult struct {
	OverflowX float64 `json:"x"`
	OverflowY float64 `json:"y"`
}

func render(htmlPath string, timeout time.Duration) (pdf []byte, png []byte, err error) {
	absPath, err := filepath.Abs(htmlPath)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()
	ctx, cancelT := context.WithTimeout(ctx, timeout)
	defer cancelT()

	var overflow overflowResult
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(pageWidthPx, pageHeightPx, chromedp.EmulateScale(renderScale)),
		chromedp.Navigate("file://"+absPath),
		// Wait for web fonts so the raster matches the final typography.
		chromedp.Evaluate(`document.fonts.ready.then(() => true)`, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
		chromedp.Evaluate(`(() => {
			const el = document.querySelector('.page');
			return { x: el.scrollWidth - el.clientWidth, y: el.scrollHeight - el.clientHeight };
		})()`, &overflow),
		chromedp.FullScreenshot(&png, 100),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdf, _, err = page.PrintToPDF().
				WithPaperWidth(8.5).WithPaperHeight(11).
				WithMarginTop(0).WithMarginBottom(0).WithMarginLeft(0).WithMarginRight(0).
				WithPrintBackground(true).
				Do(ctx)
			return err
		}),
	)
	if err != nil {
		return nil, nil, err
	}

	// A page that overflows would clip or overlap text in ways ground truth
	// cannot describe. Fail loudly so content gets trimmed instead.
	if overflow.OverflowX > 2 || overflow.OverflowY > 2 {
		return nil, nil, fmt.Errorf("content overflows page by %.0fpx horizontally, %.0fpx vertically; shorten content or reduce font size", overflow.OverflowX, overflow.OverflowY)
	}

	return pdf, png, nil
}
