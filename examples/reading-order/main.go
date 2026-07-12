package main

import (
	"fmt"

	"github.com/goodblaster/gollate/pkg/sorters"
)

// This example demonstrates how to configure reading order for different languages.
// Reading order affects how the algorithm calculates distances between OCR blocks.
func main() {
	fmt.Println("Reading Order Configuration Examples")
	fmt.Println("====================================")
	fmt.Println()

	// 1. Using preset configurations
	fmt.Println("1. Preset Configurations:")
	fmt.Println("   - DefaultConfig: Horizontal LTR (English, most European languages)")
	fmt.Println("   - CJKConfig: Vertical TTB, RTL (Traditional Chinese, Japanese)")
	fmt.Println("   - RTLConfig: Horizontal RTL (Arabic, Hebrew)")
	fmt.Println()

	// Example: Using CJK preset for vertical text
	cjkConfig := sorters.CJKConfig()
	fmt.Printf("CJK Config uses: %s\n", cjkConfig.ReadingOrder.String())
	fmt.Println()

	// Example: Using RTL preset for Arabic
	rtlConfig := sorters.RTLConfig()
	fmt.Printf("RTL Config uses: %s\n", rtlConfig.ReadingOrder.String())
	fmt.Println()

	// 2. Custom configuration with specific reading order
	fmt.Println("2. Custom Configuration:")
	customConfig := sorters.DefaultConfig()

	// For modern Chinese (horizontal), override the reading order
	customConfig.ReadingOrder = sorters.HorizontalLTR_TTB
	fmt.Printf("Modern Chinese config: %s\n", customConfig.ReadingOrder.String())
	fmt.Println()

	// For Arabic documents
	arabicConfig := sorters.DefaultConfig()
	arabicConfig.ReadingOrder = sorters.HorizontalRTL_TTB
	fmt.Printf("Arabic config: %s\n", arabicConfig.ReadingOrder.String())
	fmt.Println()

	// For traditional Japanese (vertical)
	japaneseConfig := sorters.DefaultConfig()
	japaneseConfig.ReadingOrder = sorters.VerticalTTB_RTL
	fmt.Printf("Traditional Japanese config: %s\n", japaneseConfig.ReadingOrder.String())
	fmt.Println()

	// 3. Available reading orders
	fmt.Println("3. All Available Reading Orders:")
	orders := []sorters.ReadingOrder{
		sorters.HorizontalLTR_TTB,
		sorters.HorizontalRTL_TTB,
		sorters.VerticalTTB_RTL,
		sorters.VerticalTTB_LTR,
	}

	for _, order := range orders {
		fmt.Printf("   - %s\n", order.String())
		fmt.Printf("     Horizontal: %t, Vertical: %t, LTR: %t, RTL: %t\n",
			order.IsHorizontal(),
			order.IsVertical(),
			order.IsLeftToRight(),
			order.IsRightToLeft(),
		)
	}
	fmt.Println()

	// 4. Example usage in sorting
	fmt.Println("4. Example Usage:")
	fmt.Println("   blocks := []ocr.Block{...}  // Your OCR blocks")
	fmt.Println("   lines := []string{...}       // Your canonical text")
	fmt.Println()
	fmt.Println("   // For Arabic text:")
	fmt.Println("   config := sorters.RTLConfig()")
	fmt.Println("   sorter := sorters.NewOcrSorterWithConfig(blocks, lines, logger.NewLogos(), config)")
	fmt.Println("   sorted, err := sorter.Sort()")
	fmt.Println()
	fmt.Println("   // Or customize:")
	fmt.Println("   config := sorters.DefaultConfig()")
	fmt.Println("   config.ReadingOrder = sorters.HorizontalRTL_TTB")
	fmt.Println("   sorter := sorters.NewOcrSorterWithConfig(blocks, lines, logger.NewLogos(), config)")
	fmt.Println()

	fmt.Println("Note: Reading order affects:")
	fmt.Println("  - Distance calculation between blocks")
	fmt.Println("  - Which direction is considered 'sequential'")
	fmt.Println("  - How line wrapping is detected")
	fmt.Println("  - Multi-column layout handling")
}
