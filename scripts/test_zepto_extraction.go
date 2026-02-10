package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pmitra96/pateproject/extractor"
)

// Import the extractor package - we'll run this from backend dir

func main() {
	// Get the zepto.pdf path
	pdfPath := filepath.Join("..", "zepto.pdf")

	fmt.Println("Testing Zepto PDF extraction...")
	fmt.Println("PDF Path:", pdfPath)

	result, err := extractor.ParseImage(pdfPath)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	// Pretty print the result
	jsonData, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println("\n=== EXTRACTION RESULT ===")
	fmt.Println(string(jsonData))

	fmt.Printf("\n=== SUMMARY ===\n")
	fmt.Printf("Provider: %s\n", result.Provider)
	fmt.Printf("Items Found: %d\n", len(result.Items))

	if len(result.Items) == 0 {
		fmt.Println("\n⚠️  WARNING: No items extracted!")
		os.Exit(1)
	}

	fmt.Println("\n✅ Extraction successful!")
}
