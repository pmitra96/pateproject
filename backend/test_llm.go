//go:build tools
// +build tools

package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/pmitra96/pateproject/llm"
)

func main() {
	// Load environment variables for LLM_API_KEY
	err := godotenv.Load("../.env")
	if err != nil {
		fmt.Println("Warning: Could not load .env file")
	}

	client := llm.NewClient()

	tests := []string{
		"Hi",
		"I just ate an apple",
		"Can I eat a slice of pizza?",
		"How many calories do I have left today?",
		"What have I eaten so far?",
		"well, 250 gms of kichidi with minimal oil, carrot capsicum, onion, and broccoli, and 100 gm of paneer",
	}

	for _, text := range tests {
		fmt.Printf("\n--- Testing: \"%s\" ---\n", text)
		msg, usage, err := client.ProcessWhatsAppConversation(text, nil, "Goal: 2000 kcal, Mode: NORMAL, Remaining: 1200 kcal")
		_ = usage // Ignore usage in test
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		if len(msg.ToolCalls) == 0 {
			fmt.Printf("Result: [Conversational Reply] -> %s\n", msg.Content)
		} else {
			tool := msg.ToolCalls[0]
			fmt.Printf("Result: [Tool Call] -> Name: %s | Args: %s\n", tool.Function.Name, tool.Function.Arguments)
		}
	}
}
