// book-overlays renders word-number diagnostics from saved book-test results.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goodblaster/gollate/internal/bookoverlay"
	"github.com/goodblaster/gollate/pkg/multipage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	corpus := flag.String("corpus", "experiments/meditations", "Corpus directory containing layout subdirectories")
	layouts := flag.String("layouts", "single-column,double-column", "Comma-separated layout directories")
	sources := flag.String("sources", "apple,tesseract,pdftext", "Comma-separated saved result sources")
	label := flag.String("label", "current", "Saved evaluation label")
	seed := flag.Int64("seed", 117, "Reproducible sentence sampling seed")
	samples := flag.Int("sentences", 2, "Sentence samples per page (1 or 2)")
	pages := flag.String("pages", "", "Optional comma-separated 1-based page numbers for a preview")
	jobs := flag.Int("jobs", 2, "Concurrent layout/source renders")
	flag.Parse()
	if *jobs < 1 {
		return fmt.Errorf("jobs must be positive")
	}
	var selected []int
	if *pages != "" {
		for _, value := range strings.Split(*pages, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || n < 1 {
				return fmt.Errorf("invalid page %q", value)
			}
			selected = append(selected, n)
		}
	}
	for _, name := range append(append(strings.Split(*layouts, ","), strings.Split(*sources, ",")...), *label) {
		if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
			return fmt.Errorf("invalid directory component %q", name)
		}
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, *jobs)
	errors := make(chan error, len(strings.Split(*layouts, ","))*len(strings.Split(*sources, ",")))
	for _, layout := range strings.Split(*layouts, ",") {
		for _, source := range strings.Split(*sources, ",") {
			wg.Add(1)
			go func(layout, source string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				start := time.Now()
				dir := filepath.Join(*corpus, layout)
				err := func() error {
					text, err := os.ReadFile(filepath.Join(dir, "canonical.txt"))
					if err != nil {
						return err
					}
					path := filepath.Join(dir, *label, source+"-sorted.json")
					data, err := os.ReadFile(path)
					if err != nil {
						return fmt.Errorf("%w; run TestBookCorpus first", err)
					}
					var result multipage.Result
					if err := json.Unmarshal(data, &result); err != nil {
						return fmt.Errorf("%s: %w", path, err)
					}
					out := filepath.Join(dir, *label, source+"-overlays")
					fmt.Printf("Rendering %s/%s -> %s\n", layout, source, out)
					if err := bookoverlay.Write(result.Document, bookoverlay.Lines(string(text)), bookoverlay.Options{ImageDir: dir, OutputDir: out, Title: layout + " / " + source, Seed: *seed, Samples: *samples, Pages: selected}); err != nil {
						return err
					}
					fmt.Printf("Finished %s/%s in %.1fs\n", layout, source, time.Since(start).Seconds())
					return nil
				}()
				if err != nil {
					errors <- fmt.Errorf("%s/%s: %w", layout, source, err)
				}
			}(layout, source)
		}
	}
	wg.Wait()
	close(errors)
	var failures []string
	for err := range errors {
		failures = append(failures, err.Error())
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "\n"))
	}
	return nil
}
