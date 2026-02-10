package main

import (
	"fmt"

	"github.com/pmitra96/pateproject/extractor" // Import the extractor package - we'll run this from backend dir
)

func main() {
	fmt.Println("=== Testing Blinkit PDF ===")
	result, err := extractor.ParseImage("../blinkit.pdf")
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	fmt.Printf("Provider: %s\n", result.Provider)
	fmt.Printf("Items Found: %d\n\n", len(result.Items))

	for i, item := range result.Items {
		fmt.Printf("%d. %s (qty: %.0f, unit: %.0f %s)\n", i+1, item.Name, item.Count, item.UnitValue, item.Unit)
	}

	fmt.Println("\n=== Testing Zepto PDF ===")
	result2, err := extractor.ParseImage("../zepto.pdf")
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	fmt.Printf("Provider: %s\n", result2.Provider)
	fmt.Printf("Items Found: %d\n\n", len(result2.Items))

	for i, item := range result2.Items {
		fmt.Printf("%d. %s (qty: %.0f, unit: %.0f %s)\n", i+1, item.Name, item.Count, item.UnitValue, item.Unit)
	}
}
