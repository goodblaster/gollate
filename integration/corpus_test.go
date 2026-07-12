package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestCorpus scores real-world documents that can't be committed to the repo
// (copyright, PII, size). Point OCRSORT_CORPUS at a directory of entries and
// this runs the same order-sensitive accuracy scoring as the committed
// suite against them.
//
// Each entry is a subdirectory shaped exactly like a suite fixture:
//
//	<corpus>/<name>/canonical.txt
//	<corpus>/<name>/<engine>-ocr.json   (apple-ocr.json and/or tesseract-ocr.json)
//	<corpus>/<name>/test-info.json      (language, width, height)
//
// Build an entry from a PDF or image with scripts/corpus-add.sh. Baselines
// live in <corpus>/baselines.json - alongside the private data, never in the
// repo - and ratchet with UPDATE_BASELINES=1 just like the committed suite.
//
// The test skips when OCRSORT_CORPUS is unset, so it never blocks CI or
// contributors who don't have the corpus. Only this harness is committed;
// the copyrighted documents and their scores stay entirely local.
func TestCorpus(t *testing.T) {
	root := os.Getenv("OCRSORT_CORPUS")
	if root == "" {
		t.Skip("set OCRSORT_CORPUS to a corpus directory to score real documents")
	}

	dirs, err := corpusDirs(root)
	if err != nil {
		t.Fatalf("reading corpus %s: %v", root, err)
	}
	if len(dirs) == 0 {
		t.Skipf("no entries in corpus %s (expected <name>/canonical.txt)", root)
	}

	baselineFile := filepath.Join(root, "baselines.json")
	baselines := map[string]float64{}
	if data, err := os.ReadFile(baselineFile); err == nil {
		if err := json.Unmarshal(data, &baselines); err != nil {
			t.Fatalf("parsing %s: %v", baselineFile, err)
		}
	}
	update := os.Getenv("UPDATE_BASELINES") != ""
	results := map[string]float64{}

	for _, name := range dirs {
		dir := filepath.Join(root, name)
		for _, engine := range engines {
			if _, err := os.Stat(filepath.Join(dir, engine+"-ocr.json")); err != nil {
				continue
			}
			key := name + "/" + engine
			t.Run(key, func(t *testing.T) {
				accuracy, detail, err := runCase(dir, engine)
				if err != nil {
					t.Fatalf("sorting failed: %v", err)
				}
				results[key] = accuracy
				t.Logf("accuracy %.2f%% (%s)", accuracy, detail)

				baseline, ok := baselines[key]
				switch {
				case !ok && !update:
					t.Errorf("no baseline for %s (accuracy %.2f%%); run with UPDATE_BASELINES=1", key, accuracy)
				case ok && accuracy < baseline-baselineTolerance:
					t.Errorf("accuracy %.2f%% dropped below baseline %.2f%%", accuracy, baseline)
				}
			})
		}
	}

	if update {
		changed := false
		for key, accuracy := range results {
			if rounded := roundDown2(accuracy); rounded > baselines[key] {
				baselines[key] = rounded
				changed = true
			}
		}
		if changed {
			data, err := json.MarshalIndent(baselines, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(baselineFile, append(data, '\n'), 0644); err != nil {
				t.Fatal(err)
			}
			t.Logf("corpus baselines updated: %s", baselineFile)
		}
	}
}

// corpusDirs returns the names of corpus entries (subdirs with a
// canonical.txt), sorted.
func corpusDirs(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, e.Name(), "canonical.txt")); err == nil {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}
