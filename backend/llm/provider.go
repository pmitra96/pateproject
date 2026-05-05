package llm

import "github.com/pmitra96/pateproject/config"

// Provider defines the interface that all LLM adapters must implement.
type Provider interface {
	Chat(messages []Message) (string, Usage, error)
	ChatWithTools(messages []Message, tools []Tool) (*Message, Usage, error)
}

// NewProvider returns the configured LLM provider.
func NewProvider() Provider {
	providerType := config.GetEnv("LLM_PROVIDER", "smart")
	
	switch providerType {
	case "smart":
		return NewSmartRouterProvider()
	case "gemini":
		return NewGeminiProvider()
	case "openai":
		return NewOpenAIProvider()
	default:
		return NewSmartRouterProvider() // Default to smart
	}
}
