// readme-preview crops a small, real OCR example for the GitHub README.
// The two panels retain word geometry within their original columns; only
// the vertical whitespace between the excerpts is removed for presentation.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/goodblaster/gollate/internal/bookoverlay"
	"github.com/goodblaster/gollate/pkg/multipage"
	"golang.org/x/image/font/gofont/goregular"
)

func main() {
	corpus := flag.String("corpus", "experiments/meditations", "Prepared and evaluated book corpus")
	output := flag.String("output", "docs/images/reading-order.png", "README PNG")
	flag.Parse()
	if err := preview(*corpus, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func preview(corpus, output string) error {
	const page, first, last = 18, 8786, 8869
	const scale, margin, gutter, top = 2, 36, 116, 158
	dir := filepath.Join(corpus, "double-column")
	canonical, err := os.ReadFile(filepath.Join(dir, "canonical.txt"))
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(dir, "current/apple-sorted.json"))
	if err != nil {
		return err
	}
	var result multipage.Result
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	m, err := bookoverlay.Build(result.Document, bookoverlay.Lines(string(canonical)), 117, 2)
	if err != nil {
		return err
	}
	if len(m.Pages) < page || len(m.Words) < last || m.Words[8804].Text != "capacity" {
		return fmt.Errorf("preview expects the pinned Meditations edition and its original pagination")
	}
	f, err := os.Open(filepath.Join(dir, "page-18.png"))
	if err != nil {
		return err
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	var marks []bookoverlay.Mark
	seen := make(map[int]bool)
	for _, mark := range bookoverlay.SmoothMarks(m.Pages[page-1].Marks, m, float64(w), float64(h)) {
		for _, n := range mark.Words {
			if n >= first && n <= last {
				marks = append(marks, mark)
				seen[n] = true
				break
			}
		}
	}
	for n := first; n <= last; n++ {
		if !seen[n] {
			return fmt.Errorf("preview word %d has no location; inspect the OCR result before replacing the example", n)
		}
	}
	// Crop the bottom of the left column and top of the right column.
	// Column knowledge is used ONLY in this presentation tool, never sorting.
	crops := [2]image.Rectangle{
		{Min: image.Pt(int(.075*float64(w)), h), Max: image.Pt(int(.49*float64(w)), 0)},
		{Min: image.Pt(int(.515*float64(w)), h), Max: image.Pt(int(.93*float64(w)), 0)},
	}
	for _, mark := range marks {
		col := 0
		if mark.Bounds.Left >= .5 {
			col = 1
		}
		crops[col].Min.Y = min(crops[col].Min.Y, int(mark.Bounds.Top*float64(h))-3)
		crops[col].Max.Y = max(crops[col].Max.Y, int(math.Ceil((mark.Bounds.Top+mark.Bounds.Height)*float64(h)))+3)
	}
	panelW := max(crops[0].Dx(), crops[1].Dx()) * scale
	panelH := max(crops[0].Dy(), crops[1].Dy()) * scale
	if panelH > h {
		return fmt.Errorf("selected excerpt is too tall for a compact README preview")
	}
	width, height := 2*margin+2*panelW+gutter, top+panelH+112
	dc := gg.NewContext(width, height)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	ttf, err := truetype.Parse(goregular.TTF)
	if err != nil {
		return err
	}
	face := func(size float64) { dc.SetFontFace(truetype.NewFace(ttf, &truetype.Options{Size: size, DPI: 72})) }
	ink := color.RGBA{28, 39, 56, 255}
	green := color.RGBA{0, 112, 85, 255}
	purple := color.RGBA{137, 55, 155, 255}
	face(31)
	dc.SetColor(ink)
	dc.DrawString("Recover reading order. Keep each word's location.", margin, 45)
	face(21)
	dc.SetRGB(.35, .39, .45)
	dc.DrawString("Two excerpts from the same page  /  real Apple Vision OCR, matched to the book's reference text", margin, 81)
	for col, crop := range crops {
		x := margin + col*(panelW+gutter)
		face(19)
		dc.SetColor(ink)
		title := "1  LEFT COLUMN / bottom"
		if col == 1 {
			title = "2  RIGHT COLUMN / top"
		}
		dc.DrawString(title, float64(x), 126)
		// A tightly cropped bitmap contains no hidden full-page source image.
		clipped := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
		draw.Draw(clipped, clipped.Bounds(), img, crop.Min, draw.Src)
		dc.Push()
		dc.Translate(float64(x), top)
		dc.Scale(scale, scale)
		dc.DrawImage(clipped, 0, 0)
		dc.Pop()
	}
	type box struct{ x, y, w, h float64 }
	locate := func(mark bookoverlay.Mark) box {
		col := 0
		if mark.Bounds.Left >= .5 {
			col = 1
		}
		b := mark.Bounds
		return box{float64(margin+col*(panelW+gutter)) + (b.Left*float64(w)-float64(crops[col].Min.X))*scale,
			top + (b.Top*float64(h)-float64(crops[col].Min.Y))*scale, b.Width * float64(w) * scale, b.Height * float64(h) * scale}
	}
	markColor := func(mark bookoverlay.Mark) color.RGBA {
		if m.Words[mark.Words[0]-1].Sentence == m.Words[first-1].Sentence {
			return green
		}
		return purple
	}
	var head, tail *box
	for _, mark := range marks {
		b, c := locate(mark), markColor(mark)
		dc.SetRGBA255(int(c.R), int(c.G), int(c.B), 22)
		dc.DrawRectangle(b.x, b.y, b.w, b.h)
		dc.Fill()
		dc.SetColor(c)
		dc.SetLineWidth(1.4)
		dc.DrawRectangle(b.x, b.y, b.w, b.h)
		dc.Stroke()
		if mark.Words[0] == 8805 {
			if mark.Continuation {
				tail = &b
			} else {
				head = &b
			}
		}
	}
	if head == nil || tail == nil {
		return fmt.Errorf("preview no longer contains the cross-column continuation for capacity")
	}
	// The arrow joins the two observed boxes, not inferred text positions.
	dc.SetColor(green)
	dc.SetLineWidth(3)
	sx, sy, ex, ey := head.x+head.w+6, head.y+head.h/2, tail.x-8, tail.y+tail.h/2
	mid := float64(margin + panelW + gutter/2)
	dc.MoveTo(sx, sy)
	dc.CubicTo(mid, sy, mid, ey, ex, ey)
	dc.Stroke()
	dc.MoveTo(ex-10, ey-7)
	dc.LineTo(ex, ey)
	dc.LineTo(ex-10, ey+7)
	dc.Stroke()
	face(17)
	var occupied []image.Rectangle
	for _, mark := range marks {
		b, c := locate(mark), markColor(mark)
		label := strconv.Itoa(mark.Words[0])
		if mark.Continuation {
			label += "+"
		}
		tw, _ := dc.MeasureString(label)
		lw, lh := int(math.Ceil(tw))+6, 21
		chosen := image.Rect(int(b.x), int(b.y)-lh, int(b.x)+lw, int(b.y))
		for _, dy := range []float64{-float64(lh), 0, b.h - float64(lh)} {
			r := image.Rect(int(b.x), int(b.y+dy), int(b.x)+lw, int(b.y+dy)+lh)
			collision := false
			for _, used := range occupied {
				if r.Overlaps(used) {
					collision = true
					break
				}
			}
			if !collision {
				chosen = r
				break
			}
		}
		occupied = append(occupied, chosen)
		dc.SetRGBA(1, 1, 1, .95)
		dc.DrawRectangle(float64(chosen.Min.X), float64(chosen.Min.Y), float64(lw), float64(lh))
		dc.Fill()
		dc.SetColor(c)
		dc.DrawString(label, float64(chosen.Min.X+3), float64(chosen.Min.Y+17))
	}
	y := float64(top + crops[0].Dy()*scale + 68)
	face(18)
	dc.SetColor(ink)
	dc.DrawString("SAME WORD, TWO LOCATIONS", margin, y)
	face(29)
	dc.SetColor(green)
	dc.DrawString("ca-  +  pacity  =  capacity", margin, y+43)
	face(21)
	dc.DrawString("8805 and 8805+ keep both original positions.", margin, y+81)
	y = float64(top + panelH + 38)
	face(20)
	dc.SetColor(ink)
	dc.DrawString("Numbers = word positions in the reference text. Colors = different sentences.", margin, y)
	face(17)
	dc.SetRGB(.4, .43, .48)
	dc.DrawString("Meditations, George Long translation / page 18 / two cropped column excerpts; original word positions preserved", margin, y+35)
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	if err := dc.SavePNG(output); err != nil {
		return err
	}
	fmt.Printf("Wrote %s (%d x %d), canonical words %d-%d\n", output, width, height, first, last)
	return nil
}
