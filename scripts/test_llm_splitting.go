package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pmitra96/pateproject/llm"
)

func main() {
	if os.Getenv("LLM_API_KEY") == "" {
		fmt.Println("WARNING: LLM_API_KEY is not set. This test will likely fail unless a mock is used.")
	}

	client := llm.NewClient()

	testNames := []string{
		"Amul Taaza Toned Milk 1L",
		"Akshayakalpa Artisanal Organic Set Curd Cup",
		"Blinkit - Bread 400g",
		"Zepto - Whole Wheat Bread",
		"Organic Tomato 1kg",
	}

	fmt.Println("=== Testing LLM Splitting ===")
	for _, name := range testNames {
		fmt.Printf("\nRaw Name: %s\n", name)
		extraction, err := client.ExtractPantryItemInfo(name)
		if err != nil {
			fmt.Printf("Error extracting: %v\n", err)
			continue
		}

		jsonData, _ := json.MarshalIndent(extraction, "", "  ")
		fmt.Println(string(jsonData))
	}
}
