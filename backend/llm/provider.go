package llm

import "github.com/pmitra96/pateproject/config"

// Provider defines the interface that all LLM adapters must implement.
type Provider interface {
	Chat(messages []Message) (string, Usage, error)
	ChatWithTools(messages []Message, tools []Tool) (*Message, Usage, error)
}

// NewProvider returns the configured LLM provider.
func NewProvider() Provider {
	providerType := config.GetEnv("LLM_PROVIDER", "openai")
	
	switch providerType {
	case "openai":
		return NewOpenAIProvider()
	case "smart":
		// Keep backward compatibility, but route to OpenAI-only mode.
		return NewOpenAIProvider()
	case "gemini":
		// Keep backward compatibility, but route to OpenAI-only mode.
		return NewOpenAIProvider()
	default:
		return NewOpenAIProvider()
	}
}
