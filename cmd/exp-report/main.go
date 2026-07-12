// exp-report prints a per-case accuracy delta table comparing experiment
// runs against a default-config baseline run. Inputs are the RESULTS_JSON
// files written by the integration suite.
//
// Usage:
//
//	exp-report baseline.json variant1.json [variant2.json ...]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type resultsFile struct {
	Flags   string             `json:"flags"`
	Results map[string]float64 `json:"results"`
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: exp-report baseline.json variant.json [variant.json ...]")
		os.Exit(2)
	}

	baseline := load(os.Args[1])
	var variants []resultsFile
	for _, path := range os.Args[2:] {
		variants = append(variants, load(path))
	}

	var cases []string
	for k := range baseline.Results {
		cases = append(cases, k)
	}
	sort.Strings(cases)

	// Header.
	fmt.Printf("%-34s %9s", "case", "baseline")
	for i := range variants {
		fmt.Printf(" %8s %7s", fmt.Sprintf("V%d", i+1), "delta")
	}
	fmt.Println()

	// Per-case rows plus mean delta per variant.
	sums := make([]float64, len(variants))
	counts := make([]int, len(variants))
	for _, c := range cases {
		base := baseline.Results[c]
		fmt.Printf("%-34s %8.2f%%", c, base)
		for i, v := range variants {
			score, ok := v.Results[c]
			if !ok {
				fmt.Printf(" %8s %7s", "-", "-")
				continue
			}
			fmt.Printf(" %7.2f%% %+7.2f", score, score-base)
			sums[i] += score - base
			counts[i]++
		}
		fmt.Println()
	}

	fmt.Printf("%-34s %9s", "mean delta", "")
	for i := range variants {
		if counts[i] == 0 {
			fmt.Printf(" %8s %7s", "-", "-")
			continue
		}
		fmt.Printf(" %8s %+7.2f", "", sums[i]/float64(counts[i]))
	}
	fmt.Println()

	fmt.Println()
	for i, v := range variants {
		flags := v.Flags
		if flags == "" {
			flags = "(default config)"
		}
		fmt.Printf("V%d: %s\n", i+1, flags)
	}
}

func load(path string) resultsFile {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var rf resultsFile
	if err := json.Unmarshal(data, &rf); err != nil {
		fmt.Fprintf(os.Stderr, "parsing %s: %v\n", path, err)
		os.Exit(1)
	}
	return rf
}
