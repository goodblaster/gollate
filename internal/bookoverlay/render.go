package bookoverlay

import (
	"encoding/json"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/goodblaster/gollate/pkg/multipage"
	"golang.org/x/image/font/gofont/goregular"
)

type Options struct {
	ImageDir     string
	OutputDir    string
	Title        string
	Seed         int64
	Samples      int
	AllSentences bool
	// Pages optionally restricts rendering to these one-based page numbers.
	Pages []int
}

var blue = color.RGBA{25, 70, 174, 255}
var red = color.RGBA{183, 43, 38, 255}
var sampleColors = []color.RGBA{{0, 114, 86, 255}, {146, 62, 156, 255}}

// Write renders both views using existing normalized boxes. Each PNG doubles
// the source resolution for legible labels and adds a legend outside the page.
func Write(doc *multipage.Document, canonical []string, opts Options) error {
	modes := []string{"order", "spotcheck"}
	if opts.AllSentences {
		opts.Samples = 0
		modes = []string{"highlight"}
	}
	m, err := Build(doc, canonical, opts.Seed, opts.Samples)
	if err != nil {
		return err
	}
	pages := opts.Pages
	if len(pages) == 0 {
		for i := range m.Pages {
			pages = append(pages, i+1)
		}
	}
	for _, n := range pages {
		if n < 1 || n > len(m.Pages) {
			return fmt.Errorf("page %d out of range", n)
		}
	}
	for _, mode := range modes {
		if err := os.MkdirAll(filepath.Join(opts.OutputDir, mode), 0755); err != nil {
			return err
		}
	}
	for _, n := range pages {
		path := filepath.Join(opts.ImageDir, fmt.Sprintf("page-%d.png", n))
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for _, mode := range modes {
			out, err := render(img, m, m.Pages[n-1], opts.Title, mode)
			if err != nil {
				return err
			}
			path := filepath.Join(opts.OutputDir, mode, fmt.Sprintf("page-%03d.png", n))
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			err = (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(f, out)
			closeErr := f.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(opts.OutputDir, "manifest.json"), append(data, '\n'), 0644); err != nil {
		return err
	}
	readme := fmt.Sprintf("# %s overlays\n\nRendered pages: %v\n\n- order/page-NNN.png: every final output token with its canonical word number.\n- spotcheck/page-NNN.png: up to %d seeded sentence samples per page (seed %d).\n- manifest.json: all canonical words and byte spans, sentences, per-page boxes, final output token order, samples, and unlinked canonical word IDs.\n\nNumbers start at 1 across the complete canonical text and count whitespace-separated words, including headings and note numbers. Compounds count as one word; split tokens may repeat that number. A range means one token maps to several canonical words. A + marks a continuation of the same word on another line/page. U123 is an unlinked output token at final output position 123, not canonical word 123. Running headers and page numbers removed by the sorter have no labels.\n\nBoxes show the sorter's saved coordinates and canonical links, including incorrect links. They are not independent ground truth or new OCR. Missing canonical words have no inferred coordinates and are listed in the manifest. A word is considered linked if any token overlaps its canonical span; this does not establish full recognition. Sentence boundaries are a punctuation-based heuristic. Samples are drawn from sentences with at least one linked word on that page, without requiring complete recognition; pages with no links cannot be sampled. Sentence fragments on other pages are counted separately in the sidebar. Full source sentences are preserved in the manifest.\n", opts.Title, pages, opts.Samples, opts.Seed)
	if opts.AllSentences {
		readme = fmt.Sprintf("# %s — complete sentence highlights\n\nRendered pages: %v\n\n`highlight/page-NNN.png` highlights every linked sentence, using alternating green and purple by global sentence ID (stable across columns and pages). Red U boxes show unlinked output. Canonical words with no matched box cannot be drawn; they are listed in `manifest.json`. Its `sampled_sentences` lists every linked sentence when `sentence_selection` is `all`.\n\n", opts.Title, pages)
	}
	readme += "\nSource word boxes are retained in manifest.json. Display smoothing connects only neighboring words in the same sentence and visual line, dividing interior whitespace and sharing a vertical extent. Sentence starts/ends, line breaks and column gutters are not extended.\n"
	if opts.AllSentences {
		var gallery strings.Builder
		fmt.Fprintf(&gallery, "<!doctype html><html lang=\"en\"><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width\"><title>%s</title><style>body{font-family:system-ui;margin:24px;background:#f2f4f7;color:#172033}nav{display:flex;flex-wrap:wrap;gap:10px}img{display:block;max-width:100%%;height:auto;background:white}section{margin:32px 0}a{color:#204c99}</style><h1>%s</h1><p>Every linked sentence, across both columns and all rendered pages. Green and purple alternate by sentence; red boxes mark unlinked output. Click an image for full resolution.</p><nav>", html.EscapeString(opts.Title), html.EscapeString(opts.Title))
		for _, n := range pages {
			fmt.Fprintf(&gallery, "<a href=\"#page-%d\">%d</a> ", n, n)
		}
		gallery.WriteString("</nav>")
		for _, n := range pages {
			fmt.Fprintf(&gallery, "<section id=\"page-%d\"><h2>Page %d</h2><a href=\"highlight/page-%03d.png\"><img loading=\"lazy\" src=\"highlight/page-%03d.png\" alt=\"Page %d with numbered sentence highlights\"></a></section>", n, n, n, n, n)
		}
		gallery.WriteString("</html>\n")
		if err := os.WriteFile(filepath.Join(opts.OutputDir, "index.html"), []byte(gallery.String()), 0644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(opts.OutputDir, "README.md"), []byte(readme), 0644)
}

func label(mark Mark) string {
	s := "U" + strconv.Itoa(mark.OutputOrder)
	if len(mark.Words) > 0 {
		s = strconv.Itoa(mark.Words[0])
		if len(mark.Words) > 1 {
			s += "-" + strconv.Itoa(mark.Words[len(mark.Words)-1])
		}
	}
	if mark.Continuation {
		s += "+"
	}
	return s
}

func render(img image.Image, m *Manifest, page Page, title, mode string) (image.Image, error) {
	const scale = 2
	const header = 96
	w, h := img.Bounds().Dx()*scale, img.Bounds().Dy()*scale
	dc := gg.NewContext(w+620, h+header)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.Push()
	dc.Translate(0, header)
	dc.Scale(scale, scale)
	dc.DrawImage(img, -img.Bounds().Min.X, -img.Bounds().Min.Y)
	dc.Pop()
	ttf, err := truetype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	face := func(size float64) { dc.SetFontFace(truetype.NewFace(ttf, &truetype.Options{Size: size, DPI: 72})) }
	face(25)
	dc.SetRGB(.08, .1, .16)
	dc.DrawString(fmt.Sprintf("%s  |  page %d / %d  |  %s", title, page.Number, len(m.Pages), mode), 24, 34)
	face(18)
	dc.DrawString("Canonical word numbers, starting at 1. Boxes use the final sorter's saved positions.", 24, 66)
	palette := make(map[int]color.RGBA)
	for i, id := range page.Sentences {
		if mode == "highlight" {
			i = id - 1 // A sentence keeps its color across columns and pages.
		}
		palette[id] = sampleColors[i%len(sampleColors)]
	}
	markColor := func(mark Mark) (color.RGBA, bool) {
		for _, n := range mark.Words {
			if c, ok := palette[m.Words[n-1].Sentence]; ok {
				return c, true
			}
		}
		if len(mark.Words) == 0 {
			return red, false
		}
		return blue, false
	}
	// Paint all boxes before badges, so later boxes cannot cover earlier labels.
	marks := SmoothMarks(page.Marks, m, float64(w), float64(h))
	sort.SliceStable(marks, func(i, j int) bool {
		a, b := math.Round(marks[i].Bounds.Top*500), math.Round(marks[j].Bounds.Top*500)
		if a == b {
			return marks[i].Bounds.Left < marks[j].Bounds.Left
		}
		return a < b
	})
	var visible []Mark
	for _, mark := range marks {
		c, selected := markColor(mark)
		if mode == "spotcheck" && !selected {
			continue
		}
		visible = append(visible, mark)
		b := mark.Bounds
		x, y, bw, bh := b.Left*float64(w), header+b.Top*float64(h), b.Width*float64(w), b.Height*float64(h)
		dc.SetRGBA255(int(c.R), int(c.G), int(c.B), 22)
		dc.DrawRectangle(x, y, bw, bh)
		dc.Fill()
		dc.SetColor(c)
		dc.SetLineWidth(1.3)
		dc.DrawRectangle(x, y, bw, bh)
		dc.Stroke()
	}
	// Avoid badge collisions where short adjacent words have five-digit labels.
	// Leader lines retain the connection when a badge needs a second position.
	face(13)
	var occupied []image.Rectangle
	for _, mark := range visible {
		c, _ := markColor(mark)
		b := mark.Bounds
		x, y, bw, bh := b.Left*float64(w), header+b.Top*float64(h), b.Width*float64(w), b.Height*float64(h)
		text := label(mark)
		tw, _ := dc.MeasureString(text)
		lw, lh := int(math.Ceil(tw))+6, 17
		var chosen image.Rectangle
		found := false
		for _, cy := range []float64{y - float64(lh), y, y + bh - float64(lh), y + bh} {
			for _, cx := range []float64{x, x + bw - float64(lw), x + (bw-float64(lw))/2} {
				left, top := max(0, min(w-lw, int(cx))), max(header, min(header+h-lh, int(cy)))
				r := image.Rect(left, top, left+lw, top+lh)
				collision := false
				for _, used := range occupied {
					if r.Overlaps(used) {
						collision = true
						break
					}
				}
				if !collision {
					chosen, found = r, true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			chosen = image.Rect(int(x), int(y), int(x)+lw, int(y)+lh)
		}
		occupied = append(occupied, chosen)
		lx, ly := float64(chosen.Min.X), float64(chosen.Min.Y)
		dc.SetColor(c)
		dc.SetLineWidth(.8)
		dc.DrawLine(x+bw/2, y, lx+float64(lw)/2, ly+float64(lh))
		dc.Stroke()
		dc.SetRGBA(1, 1, 1, .94)
		dc.DrawRectangle(lx, ly, float64(lw), float64(lh))
		dc.Fill()
		dc.SetColor(c)
		dc.DrawString(text, lx+3, ly+13)
	}
	// Legend and canonical sentence excerpts sit beside the untouched page area.
	dc.SetRGB(.94, .96, .98)
	dc.DrawRectangle(float64(w), header, 620, float64(h))
	dc.Fill()
	x, y := float64(w+28), float64(header+38)
	paragraph := func(text string, size float64, c color.RGBA) {
		face(size)
		dc.SetColor(c)
		for _, line := range dc.WordWrap(text, 560) {
			dc.DrawString(line, x, y)
			y += size * 1.4
		}
		y += 16
	}
	dark := color.RGBA{35, 43, 57, 255}
	paragraph("HOW TO READ", 22, dark)
	paragraph("117 = canonical word 117\n117+ = continuation of the same word\nU117 = unlinked output token 117", 19, blue)
	if mode == "highlight" {
		paragraph("Green / purple = every linked sentence, alternating by sentence number across columns and pages. Red U labels = output without a canonical link.", 18, dark)
	} else {
		paragraph("Green / purple = sampled sentences. Red U labels = output without a canonical link. Removed running headers and page numbers are unnumbered.", 18, dark)
	}
	linked, unlinked := 0, 0
	seen := make(map[int]bool)
	for _, mark := range page.Marks {
		if len(mark.Words) == 0 {
			unlinked++
		} else {
			linked++
		}
		for _, n := range mark.Words {
			seen[n] = true
		}
	}
	paragraph(fmt.Sprintf("This page: %d linked boxes, %d unlinked boxes. Whole book: %d canonical words have no link.", linked, unlinked, len(m.Missing)), 18, dark)
	paragraph("Highlights connect neighboring words within a sentence and line. Outer word edges are retained; source boxes are saved in manifest.json.", 17, dark)
	if mode == "highlight" {
		paragraph(fmt.Sprintf("ALL %d LINKED SENTENCES ON THIS PAGE", len(page.Sentences)), 21, dark)
		paragraph("Sentence text and word ranges are in manifest.json. Missing canonical words have no guessed positions. These are OCR/matcher results, not verified ground truth.", 18, dark)
		return dc.Image(), nil
	}
	paragraph(fmt.Sprintf("SENTENCE SPOT CHECKS / seed %d", m.Seed), 21, dark)
	for _, id := range page.Sentences {
		s := m.Sentences[id-1]
		count := 0
		for n := s.First; n <= s.Last; n++ {
			if seen[n] {
				count++
			}
		}
		paragraph(fmt.Sprintf("Sentence %d | words %d-%d", id, s.First, s.Last), 20, palette[id])
		paragraph(fmt.Sprintf("%d / %d words linked on this page (the rest may be missing or on another page).", count, s.Last-s.First+1), 17, dark)
		text := []rune(s.Text)
		if len(text) > 900 {
			text = append(text[:900], []rune(" ... [full sentence in manifest.json]")...)
		}
		paragraph(string(text), 18, dark)
	}
	if len(page.Sentences) == 0 {
		paragraph("No canonical-linked sentences on this page.", 18, red)
	}
	paragraph("Labels show existing matcher links, not verified ground truth. Missing words have no guessed positions. manifest.json includes exact text, boxes and final output order.", 17, dark)
	return dc.Image(), nil
}

// Lines preserves the book loader's exact source-line indexes, including blanks.
func Lines(text string) []string {
	return strings.Split(text, "\n")
}
