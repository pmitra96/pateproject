package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pmitra96/pateproject/config"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

type Choice struct {
	Index   int     `json:"index"`
	Message Message `json:"message"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Choices []Choice `json:"choices"`
}

type Client struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func NewClient() *Client {
	return &Client{
		apiKey:  config.GetEnv("LLM_API_KEY", ""),
		baseURL: config.GetEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
		model:   config.GetEnv("LLM_MODEL", "gpt-3.5-turbo"),
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) Chat(messages []Message) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("LLM_API_KEY not configured")
	}

	reqBody := ChatRequest{
		Model:       c.model,
		Messages:    messages,
		MaxTokens:   1000,
		Temperature: 0.7,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return chatResp.Choices[0].Message.Content, nil
}

func (c *Client) GenerateStory(topic string) (string, error) {
	prompt := fmt.Sprintf("Tell me a short, creative story about: %s. Keep it under 200 words.", topic)
	if topic == "" {
		prompt = "Tell me a short, creative story. Keep it under 200 words."
	}

	messages := []Message{
		{Role: "system", Content: "You are a creative storyteller."},
		{Role: "user", Content: prompt},
	}

	return c.Chat(messages)
}

type InventoryItem struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
}

func (c *Client) SuggestMeals(inventory []InventoryItem) (string, error) {
	if len(inventory) == 0 {
		return "", fmt.Errorf("no inventory items provided")
	}

	// Build inventory list for prompt
	var items string
	for _, item := range inventory {
		items += fmt.Sprintf("- %s: %.0f %s\n", item.Name, item.Quantity, item.Unit)
	}

	prompt := fmt.Sprintf(`Based on these ingredients in my pantry:

%s

Suggest 3 meals I can make. For each meal:
1. Name of the dish
2. Which ingredients from my pantry it uses
3. Brief cooking instructions (2-3 sentences)

Format each meal clearly with the dish name as a header.`, items)

	messages := []Message{
		{Role: "system", Content: "You are a helpful cooking assistant. Suggest practical, easy-to-make meals based on available ingredients."},
		{Role: "user", Content: prompt},
	}

	return c.Chat(messages)
}
