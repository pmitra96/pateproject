package llm

import (

	"strings"

	"github.com/pmitra96/pateproject/logger"
)

type SmartRouterProvider struct {
	gemini Provider
	openai Provider
}

func NewSmartRouterProvider() *SmartRouterProvider {
	return &SmartRouterProvider{
		gemini: NewGeminiProvider(),
		openai: NewOpenAIProvider(),
	}
}

func (p *SmartRouterProvider) Chat(messages []Message) (string, Usage, error) {
	// 1. Try Gemini
	resp, usage, err := p.gemini.Chat(messages)
	if err == nil {
		return resp, usage, nil
	}

	// 2. Check if we should failover
	if p.shouldFailover(err) {
		logger.Warn("Gemini failed/busy, failing over to OpenAI", "error", err)
		return p.openai.Chat(messages)
	}

	return resp, usage, err
}

func (p *SmartRouterProvider) ChatWithTools(messages []Message, tools []Tool) (*Message, Usage, error) {
	// 1. Try Gemini
	resp, usage, err := p.gemini.ChatWithTools(messages, tools)
	if err == nil {
		return resp, usage, nil
	}

	// 2. Check if we should failover
	if p.shouldFailover(err) {
		logger.Warn("Gemini failed/busy, failing over to OpenAI (Tools)", "error", err)
		return p.openai.ChatWithTools(messages, tools)
	}

	return resp, usage, err
}

func (p *SmartRouterProvider) shouldFailover(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	// Failover on Quota (429), High Demand (503), or TLS timeouts
	return strings.Contains(errStr, "429") || 
		   strings.Contains(errStr, "503") || 
		   strings.Contains(errStr, "timeout") ||
		   strings.Contains(errStr, "unavailable")
}
