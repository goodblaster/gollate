package sorters

import (
	"testing"
)

// TestShouldSkipLineInEarlyPass tests the early pass line filtering logic
func TestShouldSkipLineInEarlyPass(t *testing.T) {
	tests := []struct {
		name      string
		pass      int
		wordCount int
		minWords  int
		expected  bool
	}{
		{
			name:      "early pass with short line",
			pass:      2,
			wordCount: 3,
			minWords:  5,
			expected:  true,
		},
		{
			name:      "early pass with long line",
			pass:      2,
			wordCount: 10,
			minWords:  5,
			expected:  false,
		},
		{
			name:      "later pass with short line",
			pass:      5,
			wordCount: 3,
			minWords:  5,
			expected:  false,
		},
		{
			name:      "boundary: exactly at early pass threshold",
			pass:      EarlyPassThreshold,
			wordCount: 3,
			minWords:  5,
			expected:  true,
		},
		{
			name:      "boundary: just after early pass threshold",
			pass:      EarlyPassThreshold + 1,
			wordCount: 3,
			minWords:  5,
			expected:  false,
		},
		{
			name:      "boundary: exactly at min words",
			pass:      2,
			wordCount: 5,
			minWords:  5,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkipLineInEarlyPass(tt.pass, tt.wordCount, tt.minWords)
			if result != tt.expected {
				t.Errorf("shouldSkipLineInEarlyPass(%d, %d, %d) = %v, expected %v",
					tt.pass, tt.wordCount, tt.minWords, result, tt.expected)
			}
		})
	}
}

// TestShouldExitPassLoop tests the multi-pass loop exit condition logic
func TestShouldExitPassLoop(t *testing.T) {
	tests := []struct {
		name               string
		pass               int
		linesFoundThisPass int
		previousLineCount  int
		currentLineCount   int
		minPass            int
		expected           bool
	}{
		{
			name:               "first pass, no lines found",
			pass:               0,
			linesFoundThisPass: 0,
			previousLineCount:  10,
			currentLineCount:   10,
			expected:           false, // Don't exit on first pass
		},
		{
			name:               "later pass, found lines",
			pass:               3,
			linesFoundThisPass: 5,
			previousLineCount:  10,
			currentLineCount:   10,
			expected:           false, // Still making progress
		},
		{
			name:               "later pass, no lines found, no splits",
			pass:               3,
			linesFoundThisPass: 0,
			previousLineCount:  10,
			currentLineCount:   10,
			expected:           true, // No progress, should exit
		},
		{
			name:               "later pass, no lines found, but lines were split",
			pass:               3,
			linesFoundThisPass: 0,
			previousLineCount:  10,
			currentLineCount:   12,
			expected:           false, // Lines split, keep going
		},
		{
			name:               "no progress but pass at or below minPass",
			pass:               3,
			linesFoundThisPass: 0,
			previousLineCount:  10,
			currentLineCount:   10,
			minPass:            EarlyPassThreshold + 1,
			expected:           false, // Short-line anchoring: survive until the early-pass filter relaxes
		},
		{
			name:               "no progress just past minPass",
			pass:               EarlyPassThreshold + 2,
			linesFoundThisPass: 0,
			previousLineCount:  10,
			currentLineCount:   10,
			minPass:            EarlyPassThreshold + 1,
			expected:           true, // Short lines had their attempt; exit as usual
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldExitPassLoop(tt.pass, tt.linesFoundThisPass, tt.previousLineCount, tt.currentLineCount, tt.minPass)
			if result != tt.expected {
				t.Errorf("shouldExitPassLoop(%d, %d, %d, %d, %d) = %v, expected %v",
					tt.pass, tt.linesFoundThisPass, tt.previousLineCount, tt.currentLineCount, tt.minPass, result, tt.expected)
			}
		})
	}
}

// TestShouldSkipChain tests chain validation logic
func TestShouldSkipChain(t *testing.T) {
	tests := []struct {
		name     string
		chain    [][]Block
		expected bool
	}{
		{
			name:     "empty chain",
			chain:    [][]Block{},
			expected: true,
		},
		{
			name: "single word chain",
			chain: [][]Block{
				{{Index: 0, NormedText: "hello"}},
			},
			expected: true,
		},
		{
			name: "two word chain",
			chain: [][]Block{
				{{Index: 0, NormedText: "hello"}},
				{{Index: 1, NormedText: "world"}},
			},
			expected: false,
		},
		{
			name: "longer chain",
			chain: [][]Block{
				{{Index: 0, NormedText: "the"}},
				{{Index: 1, NormedText: "quick"}},
				{{Index: 2, NormedText: "brown"}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkipChain(tt.chain)
			if result != tt.expected {
				t.Errorf("shouldSkipChain(chain with %d words) = %v, expected %v",
					len(tt.chain), result, tt.expected)
			}
		})
	}
}

// TestGetPassPermutationLimit tests permutation limit calculation
func TestGetPassPermutationLimit(t *testing.T) {
	tests := []struct {
		name                string
		permutationsPerPass int
		maxPermutations     int
		expectedLimit       int
	}{
		{
			name:                "PermutationsPerPass set",
			permutationsPerPass: 10000,
			maxPermutations:     100000,
			expectedLimit:       10000,
		},
		{
			name:                "PermutationsPerPass zero, use max",
			permutationsPerPass: 0,
			maxPermutations:     100000,
			expectedLimit:       100000,
		},
		{
			name:                "both values set",
			permutationsPerPass: 5000,
			maxPermutations:     50000,
			expectedLimit:       5000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.PermutationsPerPass = tt.permutationsPerPass
			config.MaxPermutations = tt.maxPermutations

			sorter := &Sorter{config: config}
			result := sorter.getPassPermutationLimit()

			if result != tt.expectedLimit {
				t.Errorf("getPassPermutationLimit() = %d, expected %d", result, tt.expectedLimit)
			}
		})
	}
}

// TestInitializePathfinding tests pathfinding state reset
func TestInitializePathfinding(t *testing.T) {
	sorter := &Sorter{
		candidatePaths: []Path{{Length: 1.5}},
		shortest:       0.5,
		perm:           1000,
	}

	sorter.initializePathfinding()

	if sorter.candidatePaths != nil {
		t.Errorf("candidatePaths should be nil after initialization, got %v", sorter.candidatePaths)
	}
	// Note: We can't easily test shortest and perm values as they involve math.MaxFloat64
	// which is checked in integration tests
}

// TestBuildChainForLine tests chain building with exact matches
func TestBuildChainForLine(t *testing.T) {
	sorter := &Sorter{
		config: DefaultConfig(),
		mapped: map[string][]Block{
			"hello": {{Index: 0, NormedText: "hello"}},
			"world": {{Index: 1, NormedText: "world"}},
		},
	}

	tests := []struct {
		name               string
		canonicalWords     []string
		expectedChainLen   int
		expectedMissingLen int
	}{
		{
			name:               "all words found",
			canonicalWords:     []string{"hello", "world"},
			expectedChainLen:   2,
			expectedMissingLen: 0,
		},
		{
			name:               "some words missing",
			canonicalWords:     []string{"hello", "missing", "world"},
			expectedChainLen:   3,
			expectedMissingLen: 1,
		},
		{
			name:               "all words missing",
			canonicalWords:     []string{"foo", "bar"},
			expectedChainLen:   2,
			expectedMissingLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sorter.buildChainForLine(tt.canonicalWords)

			if len(result.chain) != tt.expectedChainLen {
				t.Errorf("Expected chain length %d, got %d", tt.expectedChainLen, len(result.chain))
			}

			if len(result.missingWords) != tt.expectedMissingLen {
				t.Errorf("Expected %d missing words, got %d", tt.expectedMissingLen, len(result.missingWords))
			}
		})
	}
}
