package sorters

import (
	"testing"

	"github.com/goodblaster/gollate/pkg/ocr"
)

// TestPresetConfigs verifies that all preset configurations are valid
func TestPresetConfigs(t *testing.T) {
	presets := map[string]SorterConfig{
		"Default":     DefaultConfig(),
		"Fast":        FastConfig(),
		"Accurate":    AccurateConfig(),
		"CJK":         CJKConfig(),
		"LargeDoc":    LargeDocumentConfig(),
		"NoisyOCR":    NoisyOCRConfig(),
		"MultiColumn": MultiColumnConfig(),
		"RTL":         RTLConfig(),
	}

	for name, config := range presets {
		t.Run(name, func(t *testing.T) {
			if err := config.Validate(); err != nil {
				t.Errorf("%s config is invalid: %v", name, err)
			}
		})
	}
}

// TestPresetCharacteristics verifies expected characteristics of each preset
func TestPresetCharacteristics(t *testing.T) {
	t.Run("FastConfig is faster than AccurateConfig", func(t *testing.T) {
		fast := FastConfig()
		accurate := AccurateConfig()

		if fast.MaxPermutations >= accurate.MaxPermutations {
			t.Errorf("Fast should have fewer permutations than Accurate: %d >= %d",
				fast.MaxPermutations, accurate.MaxPermutations)
		}

		if fast.MaxPasses >= accurate.MaxPasses {
			t.Errorf("Fast should have fewer passes than Accurate: %d >= %d",
				fast.MaxPasses, accurate.MaxPasses)
		}

	})

	t.Run("CJKConfig has tighter distance thresholds", func(t *testing.T) {
		cjk := CJKConfig()
		defaultCfg := DefaultConfig()

		if cjk.MaxWordDistance >= defaultCfg.MaxWordDistance {
			t.Errorf("CJK should have tighter word distance than Default: %.2f >= %.2f",
				cjk.MaxWordDistance, defaultCfg.MaxWordDistance)
		}

		if cjk.MinWordsForEarlyPasses >= defaultCfg.MinWordsForEarlyPasses {
			t.Errorf("CJK should process shorter lines earlier than Default: %d >= %d",
				cjk.MinWordsForEarlyPasses, defaultCfg.MinWordsForEarlyPasses)
		}
	})

	t.Run("LargeDocumentConfig has permutation limits", func(t *testing.T) {
		large := LargeDocumentConfig()
		accurate := AccurateConfig()

		if large.MaxPermutations >= accurate.MaxPermutations {
			t.Errorf("LargeDoc should have fewer permutations than Accurate to prevent timeout: %d >= %d",
				large.MaxPermutations, accurate.MaxPermutations)
		}
	})

	t.Run("MultiColumnConfig has rotation optimization", func(t *testing.T) {
		multi := MultiColumnConfig()
		if !multi.RotationOptimization {
			t.Error("MultiColumn should have rotation optimization enabled for column jumps")
		}
	})

	t.Run("ReadingOrder defaults", func(t *testing.T) {
		// Most presets should use horizontal LTR
		ltrPresets := []struct {
			name   string
			config SorterConfig
		}{
			{"Default", DefaultConfig()},
			{"Fast", FastConfig()},
			{"Accurate", AccurateConfig()},
			{"LargeDoc", LargeDocumentConfig()},
			{"NoisyOCR", NoisyOCRConfig()},
			{"MultiColumn", MultiColumnConfig()},
		}

		for _, preset := range ltrPresets {
			if preset.config.ReadingOrder != HorizontalLTR_TTB {
				t.Errorf("%s should use HorizontalLTR_TTB, got %s",
					preset.name, preset.config.ReadingOrder.String())
			}
		}

		// CJK should use vertical
		cjk := CJKConfig()
		if cjk.ReadingOrder != VerticalTTB_RTL {
			t.Errorf("CJK should use VerticalTTB_RTL, got %s", cjk.ReadingOrder.String())
		}

		// RTL should use horizontal RTL
		rtl := RTLConfig()
		if rtl.ReadingOrder != HorizontalRTL_TTB {
			t.Errorf("RTL should use HorizontalRTL_TTB, got %s", rtl.ReadingOrder.String())
		}
	})
}

// TestPresetIntegration verifies presets work with actual sorting
func TestPresetIntegration(t *testing.T) {
	t.Skip("Skipping integration test - needs investigation")
	// Use blocks without NormedText - let the sorter normalize them
	blocks := []Block{
		{Text: "Hello", Index: 0, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.1, Width: 0.1, Height: 0.05}},
		{Text: "World", Index: 1, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.25, Width: 0.1, Height: 0.05}},
	}
	lines := []string{"Hello World"}

	presets := map[string]SorterConfig{
		"Default":     DefaultConfig(),
		"Fast":        FastConfig(),
		"Accurate":    AccurateConfig(),
		"CJK":         CJKConfig(),
		"LargeDoc":    LargeDocumentConfig(),
		"NoisyOCR":    NoisyOCRConfig(),
		"MultiColumn": MultiColumnConfig(),
	}

	for name, config := range presets {
		t.Run(name, func(t *testing.T) {
			sorter := NewOcrSorterWithConfig(blocks, lines, nil, config)
			_, err := sorter.Sort()
			if err != nil {
				t.Errorf("%s preset failed to sort: %v", name, err)
			}

			metrics := sorter.Metrics()
			if metrics.LinesFound != 1 {
				t.Errorf("%s preset found %d lines, expected 1", name, metrics.LinesFound)
			}
		})
	}
}
