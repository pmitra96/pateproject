//go:build tools
// +build tools

package main

import (
	"encoding/base64"
	"fmt"
	"github.com/pmitra96/pateproject/llm"
	"os"
)

func main() {
	// Load env
	os.Setenv("LLM_MODEL", "gpt-4o-mini")

	imagePath := "/Users/pushya/Documents/pushya_projects/pateproject/backend/tests/1000174823.jpg"
	imageBytes, err := os.ReadFile(imagePath)
	if err != nil {
		fmt.Printf("Error reading image: %v\n", err)
		return
	}

	client := llm.NewClient()

	fmt.Println("--- STEP 1: Vision Analysis ---")
	analysis, err := client.AnalyzeMealImage(base64.StdEncoding.EncodeToString(imageBytes))
	if err != nil {
		fmt.Printf("Vision Error: %v\n", err)
		return
	}
	fmt.Printf("Vision Output:\n%s\n\n", analysis)

	fmt.Println("--- STEP 2: NIRA Processing ---")
	// Mock user and history
	userContext := "User ID: 1, Name: Test User"
	response, usage, err := client.ProcessWhatsAppConversation(analysis, nil, userContext)
	if err != nil {
		fmt.Printf("NIRA Error: %v\n", err)
		return
	}

	fmt.Printf("NIRA Response: %s\n", response)
	fmt.Printf("Usage: %+v\n", usage)
}
