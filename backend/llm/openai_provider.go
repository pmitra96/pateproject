package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pmitra96/pateproject/config"
)

type OpenAIProvider struct {
	apiKey  string
	baseURL string
	model   string
}

func NewOpenAIProvider() *OpenAIProvider {
	apiKey := config.GetEnv("OPENAI_API_KEY", "")
	if apiKey == "" {
		apiKey = config.GetEnv("LLM_API_KEY", "")
	}
	model := config.GetEnv("OPENAI_MODEL", "")
	if model == "" {
		model = config.GetEnv("LLM_MODEL", "gpt-4o-mini")
	}
	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: config.GetEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
		model:   model,
	}
}

func (p *OpenAIProvider) Chat(messages []Message) (string, Usage, error) {
	if p.apiKey == "" {
		return "", Usage{}, fmt.Errorf("LLM_API_KEY not configured")
	}

	reqBody := ChatRequest{
		Model:       p.model,
		Messages:    messages,
		MaxTokens:   1000,
		Temperature: 0.7,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", Usage{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", p.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", Usage{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return "", Usage{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", Usage{}, fmt.Errorf("OpenAI error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", Usage{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("no response choices returned")
	}

	contentStr, _ := chatResp.Choices[0].Message.Content.(string)
	return contentStr, chatResp.Usage, nil
}

func (p *OpenAIProvider) ChatWithTools(messages []Message, tools []Tool) (*Message, Usage, error) {
	if p.apiKey == "" {
		return nil, Usage{}, fmt.Errorf("LLM_API_KEY not configured")
	}

	reqBody := ChatRequest{
		Model:       p.model,
		Messages:    messages,
		MaxTokens:   1000,
		Temperature: 0.7,
		Tools:       tools,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, Usage{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", p.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, Usage{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return nil, Usage{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, Usage{}, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, Usage{}, fmt.Errorf("OpenAI error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, Usage{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, Usage{}, fmt.Errorf("no response choices returned")
	}

	return &chatResp.Choices[0].Message, chatResp.Usage, nil
}
