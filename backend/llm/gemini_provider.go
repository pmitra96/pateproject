package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pmitra96/pateproject/config"
)

type GeminiProvider struct {
	apiKey string
	model  string
}

func NewGeminiProvider() *GeminiProvider {
	apiKey := config.GetEnv("GEMINI_API_KEY", "")
	if apiKey == "" {
		apiKey = config.GetEnv("LLM_API_KEY", "")
	}
	model := config.GetEnv("GEMINI_MODEL", "")
	if model == "" {
		model = config.GetEnv("LLM_MODEL", "gemini-flash-latest")
	}
	return &GeminiProvider{
		apiKey: apiKey,
		model:  model,
	}
}

// Gemini Native API Structures
type GeminiPart struct {
	Text       string           `json:"text,omitempty"`
	InlineData *GeminiImageData `json:"inline_data,omitempty"`
	FunctionCall *GeminiFunctionCall `json:"function_call,omitempty"`
}

type GeminiImageData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type GeminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type GeminiContent struct {
	Role  string       `json:"role"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiTool struct {
	FunctionDeclarations []FunctionDef `json:"function_declarations"`
}

type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
	Tools    []GeminiTool    `json:"tools,omitempty"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content GeminiContent `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func (p *GeminiProvider) Chat(messages []Message) (string, Usage, error) {
	req := GeminiRequest{
		Contents: p.mapMessages(messages),
	}

	return p.callGemini(req)
}

func (p *GeminiProvider) ChatWithTools(messages []Message, tools []Tool) (*Message, Usage, error) {
	geminiTools := []GeminiTool{{
		FunctionDeclarations: make([]FunctionDef, len(tools)),
	}}
	for i, t := range tools {
		geminiTools[0].FunctionDeclarations[i] = t.Function
	}

	req := GeminiRequest{
		Contents: p.mapMessages(messages),
		Tools:    geminiTools,
	}

	return p.callGeminiWithTools(req)
}

func (p *GeminiProvider) mapMessages(messages []Message) []GeminiContent {
	contents := make([]GeminiContent, 0, len(messages))
	for _, m := range messages {
		role := m.Role
		if role == "system" || role == "user" {
			role = "user" // Gemini uses user/model
		} else if role == "assistant" {
			role = "model"
		}

		parts := []GeminiPart{}
		
		// Handle multi-modal or string content
		switch v := m.Content.(type) {
		case string:
			parts = append(parts, GeminiPart{Text: v})
		case []ContentPart:
			for _, cp := range v {
				if cp.Type == "text" {
					parts = append(parts, GeminiPart{Text: cp.Text})
				} else if cp.Type == "image_url" && cp.ImageURL != nil {
					// Handle base64 image
					data := cp.ImageURL.URL
					if strings.HasPrefix(data, "data:image/jpeg;base64,") {
						data = strings.TrimPrefix(data, "data:image/jpeg;base64,")
						parts = append(parts, GeminiPart{
							InlineData: &GeminiImageData{
								MimeType: "image/jpeg",
								Data:     data,
							},
						})
					}
				}
			}
		}

		contents = append(contents, GeminiContent{
			Role:  role,
			Parts: parts,
		})
	}
	return contents
}

func (p *GeminiProvider) callGemini(geminiReq GeminiRequest) (string, Usage, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.model, p.apiKey)
	
	jsonData, _ := json.Marshal(geminiReq)
	resp, err := sharedHTTPClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", Usage{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", Usage{}, fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, string(body))
	}

	var geminiResp GeminiResponse
	json.Unmarshal(body, &geminiResp)

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", Usage{}, fmt.Errorf("empty response from Gemini")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, Usage{
		PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
		CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
		TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
	}, nil
}

func (p *GeminiProvider) callGeminiWithTools(geminiReq GeminiRequest) (*Message, Usage, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.model, p.apiKey)
	
	jsonData, _ := json.Marshal(geminiReq)
	resp, err := sharedHTTPClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, Usage{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, Usage{}, fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, string(body))
	}

	var geminiResp GeminiResponse
	json.Unmarshal(body, &geminiResp)

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, Usage{}, fmt.Errorf("empty response from Gemini")
	}

	part := geminiResp.Candidates[0].Content.Parts[0]
	msg := &Message{Role: "assistant"}

	if part.FunctionCall != nil {
		argsBytes, _ := json.Marshal(part.FunctionCall.Args)
		msg.ToolCalls = []ToolCall{
			{
				Type: "function",
				Function: ToolCallFunction{
					Name:      part.FunctionCall.Name,
					Arguments: string(argsBytes),
				},
			},
		}
	} else {
		msg.Content = part.Text
	}

	return msg, Usage{
		PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
		CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
		TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
	}, nil
}
