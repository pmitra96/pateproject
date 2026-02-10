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
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test_extraction.go <pdf_file>")
		os.Exit(1)
	}

	// Get the PDF path
	pdfPath := os.Args[1]

	fmt.Printf("Testing PDF extraction: %s\n", filepath.Base(pdfPath))

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
