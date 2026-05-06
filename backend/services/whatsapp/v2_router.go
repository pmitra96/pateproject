package whatsapp

import (
	"strings"

	"github.com/pmitra96/pateproject/models"
)

type RouteDecision struct {
	Intent     string
	ToolName   string
	Args       map[string]interface{}
	DirectReply string
	NeedsLLM   bool
}

func routeWhatsAppMessage(text string, state *models.ConversationState) RouteDecision {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return RouteDecision{Intent: "unknown", NeedsLLM: true}
	}

	if isGreeting(lower) {
		return RouteDecision{
			Intent:      "greeting",
			DirectReply: "Hello! How can I help with your nutrition today?",
		}
	}

	if strings.Contains(lower, "what all did i eat today") || strings.Contains(lower, "what did i eat today") || strings.Contains(lower, "today's meal history") || strings.Contains(lower, "today's summary") {
		return RouteDecision{Intent: "daily_summary", ToolName: "get_daily_summary", Args: map[string]interface{}{}}
	}
	if strings.Contains(lower, "what's my budget") || strings.Contains(lower, "what is my budget") || strings.Contains(lower, "how many calories do i have left") {
		return RouteDecision{Intent: "budget", ToolName: "get_leftover_budget", Args: map[string]interface{}{}}
	}
	if strings.Contains(lower, "clear all my meals") || strings.Contains(lower, "delete all my meals") || strings.Contains(lower, "clear my day") {
		return RouteDecision{Intent: "clear_day", ToolName: "clear_all_meals_today", Args: map[string]interface{}{}}
	}

	return RouteDecision{Intent: "llm", NeedsLLM: true}
}

func isGreeting(lower string) bool {
	return lower == "hi" || lower == "hey" || lower == "hello" || lower == "good morning" || lower == "good evening"
}
