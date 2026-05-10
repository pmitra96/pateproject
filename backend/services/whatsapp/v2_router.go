package whatsapp

import (
	"strings"

	"github.com/pmitra96/pateproject/models"
)

type RouteDecision struct {
	Intent      string
	ToolName    string
	Args        map[string]interface{}
	DirectReply string
	NeedsLLM    bool
}

func routeWhatsAppMessage(text string, state *models.ConversationState) RouteDecision {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return RouteDecision{Intent: "unknown", NeedsLLM: true}
	}
	if isPlainAffirmation(lower) {
		if state != nil && (state.LastTool == "log_meals" || state.LastTool == "get_daily_summary") {
			return RouteDecision{Intent: "followup_yes", ToolName: "get_leftover_budget", Args: map[string]interface{}{}}
		}
		return RouteDecision{
			Intent:      "followup_yes",
			DirectReply: "Got it. Tell me what you want next: log a meal, get today's summary, or check remaining budget.",
		}
	}
	if isPlainNegation(lower) {
		return RouteDecision{
			Intent:      "followup_no",
			DirectReply: "Okay. Tell me what you'd like to do next.",
		}
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
	if strings.Contains(lower, "give me today's meals") || strings.Contains(lower, "what is my meals for today") || strings.Contains(lower, "meal history") || strings.Contains(lower, "show my meals") || strings.Contains(lower, "give my meal history") || strings.Contains(lower, "what are my meals") || strings.Contains(lower, "what did i have") {
		return RouteDecision{Intent: "daily_summary", ToolName: "get_daily_summary", Args: map[string]interface{}{}}
	}
	if strings.Contains(lower, "what's my budget") || strings.Contains(lower, "what is my budget") || strings.Contains(lower, "how many calories do i have left") {
		return RouteDecision{Intent: "budget", ToolName: "get_leftover_budget", Args: map[string]interface{}{}}
	}
	if strings.Contains(lower, "clear all my meals") || strings.Contains(lower, "delete all my meals") || strings.Contains(lower, "clear my day") {
		return RouteDecision{Intent: "clear_day", ToolName: "clear_all_meals_today", Args: map[string]interface{}{}}
	}
	if isCanIEatQuestion(lower) {
		food := extractFoodDescriptionFromCanIEat(text)
		if strings.TrimSpace(food) == "" {
			return RouteDecision{
				Intent:      "advice_clarify",
				DirectReply: "Tell me the food and quantity, for example: 'can I have 1 carrot?'",
			}
		}
		return RouteDecision{
			Intent:   "advice",
			ToolName: "ask_advice",
			Args: map[string]interface{}{
				"food_description": food,
			},
		}
	}
	if isNutritionQuestion(lower) {
		food := extractFoodDescriptionFromNutritionQuestion(text)
		if strings.TrimSpace(food) == "" {
			return RouteDecision{
				Intent:      "nutrition_clarify",
				DirectReply: "Tell me the food and quantity, for example: 'how many calories in 1 carrot?'",
			}
		}
		return RouteDecision{
			Intent:   "nutrition_qa",
			ToolName: "get_food_nutrition",
			Args: map[string]interface{}{
				"food_description": food,
			},
		}
	}
	if deterministic, ok := parseDeterministicCRUD(text); ok {
		return deterministic
	}
	if looksLikeMealCRUD(lower) {
		return RouteDecision{
			Intent:      "crud_format_clarification",
			DirectReply: "I can do that. Use: 'delete <dish>' or '<dish> is actually <new quantity>'. If there are multiple matches, reply with the option number (0, 1, 2...).",
		}
	}

	return RouteDecision{Intent: "llm", NeedsLLM: true}
}

func isCanIEatQuestion(lower string) bool {
	return strings.HasPrefix(lower, "can i have ") ||
		strings.HasPrefix(lower, "can i eat ") ||
		strings.Contains(lower, "can i have ") ||
		strings.Contains(lower, "can i eat ")
}

func isNutritionQuestion(lower string) bool {
	if !looksLikeQuestion(lower) {
		return false
	}
	return strings.Contains(lower, "calories") ||
		strings.Contains(lower, "protein") ||
		strings.Contains(lower, "carbs") ||
		strings.Contains(lower, "fat") ||
		strings.Contains(lower, "fibre") ||
		strings.Contains(lower, "fiber") ||
		strings.Contains(lower, "nutrition")
}

func extractFoodDescriptionFromCanIEat(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	patterns := []string{"can i have ", "can i eat "}
	for _, p := range patterns {
		idx := strings.Index(lower, p)
		if idx >= 0 {
			food := strings.TrimSpace(text[idx+len(p):])
			food = strings.Trim(food, " .!?")
			return food
		}
	}
	return ""
}

func extractFoodDescriptionFromNutritionQuestion(text string) string {
	raw := strings.TrimSpace(text)
	lower := strings.ToLower(raw)
	replacements := []string{
		"how many calories in ",
		"how much calories in ",
		"calories in ",
		"what is the nutrition in ",
		"what is nutrition in ",
		"what is the nutrition of ",
		"what is nutrition of ",
		"what are the calories in ",
	}
	for _, p := range replacements {
		if strings.HasPrefix(lower, p) {
			food := strings.TrimSpace(raw[len(p):])
			return strings.Trim(food, " .!?")
		}
	}
	return ""
}

func isGreeting(lower string) bool {
	return lower == "hi" || lower == "hey" || lower == "hello" || lower == "good morning" || lower == "good evening"
}

func isPlainAffirmation(lower string) bool {
	switch lower {
	case "yes", "y", "yeah", "yep", "sure", "ok", "okay":
		return true
	default:
		return false
	}
}

func isPlainNegation(lower string) bool {
	switch lower {
	case "no", "n", "nope", "nah":
		return true
	default:
		return false
	}
}

func looksLikeMealCRUD(lower string) bool {
	return strings.Contains(lower, "delete") ||
		strings.Contains(lower, "remove") ||
		strings.Contains(lower, "is actually") ||
		strings.Contains(lower, "correct") ||
		strings.Contains(lower, "update") ||
		strings.Contains(lower, "replace") ||
		strings.Contains(lower, "meant ")
}
