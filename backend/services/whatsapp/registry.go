package whatsapp

import (
	"encoding/json"
	"fmt"

	"github.com/pmitra96/pateproject/llm"
)

// ToolHandler is a function that processes a specific AI tool call
type ToolHandler func(s *Session, args map[string]interface{}) (string, error)

// ToolRegistry holds the mapping between LLM tool names and their Go handlers
type ToolRegistry struct {
	handlers map[string]ToolHandler
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		handlers: make(map[string]ToolHandler),
	}
}

func (r *ToolRegistry) Register(name string, handler ToolHandler) {
	r.handlers[name] = handler
}

func (r *ToolRegistry) Execute(s *Session, toolCall llm.ToolCall) (string, error) {
	handler, ok := r.handlers[toolCall.Function.Name]
	if !ok {
		return "", fmt.Errorf("tool handler not found: %s", toolCall.Function.Name)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	s.Logger.Info("Executing Tool", "name", toolCall.Function.Name)
	return handler(s, args)
}
