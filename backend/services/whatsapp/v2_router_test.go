package whatsapp

import (
	"testing"

	"github.com/pmitra96/pateproject/models"
)

func TestRouteWhatsAppMessage_YesAfterLogMealsUsesBudgetTool(t *testing.T) {
	state := &models.ConversationState{LastTool: "log_meals"}
	decision := routeWhatsAppMessage("yes", state)
	if decision.NeedsLLM {
		t.Fatalf("expected deterministic route")
	}
	if decision.ToolName != "get_leftover_budget" {
		t.Fatalf("expected get_leftover_budget, got %q", decision.ToolName)
	}
}

func TestRouteWhatsAppMessage_NoIsDirectReply(t *testing.T) {
	decision := routeWhatsAppMessage("no", &models.ConversationState{})
	if decision.NeedsLLM {
		t.Fatalf("expected deterministic route")
	}
	if decision.ToolName != "" {
		t.Fatalf("expected no tool call, got %q", decision.ToolName)
	}
	if decision.DirectReply == "" {
		t.Fatalf("expected direct reply")
	}
}

func TestRouteWhatsAppMessage_CRUDLikeButUnparsed_GivesFormatReply(t *testing.T) {
	decision := routeWhatsAppMessage("update please", &models.ConversationState{})
	if decision.NeedsLLM {
		t.Fatalf("expected deterministic clarification route")
	}
	if decision.DirectReply == "" {
		t.Fatalf("expected clarification reply")
	}
	if decision.ToolName != "" {
		t.Fatalf("expected no tool execution, got %q", decision.ToolName)
	}
}

func TestRouteWhatsAppMessage_QuestionForLunchDoesNotLogMeal(t *testing.T) {
	decision := routeWhatsAppMessage("what should i have for lunch", &models.ConversationState{})
	if decision.ToolName == "log_meals" {
		t.Fatalf("question should not be treated as meal log")
	}
}

func TestRouteWhatsAppMessage_NutritionQuestionRoutesToNutritionTool(t *testing.T) {
	decision := routeWhatsAppMessage("how many calories in 1 carrot?", &models.ConversationState{})
	if decision.NeedsLLM {
		t.Fatalf("expected deterministic route")
	}
	if decision.ToolName != "get_food_nutrition" {
		t.Fatalf("expected get_food_nutrition, got %q", decision.ToolName)
	}
}

func TestRouteWhatsAppMessage_CanIEatRoutesToAdviceTool(t *testing.T) {
	decision := routeWhatsAppMessage("can i have 1 carrot?", &models.ConversationState{})
	if decision.NeedsLLM {
		t.Fatalf("expected deterministic route")
	}
	if decision.ToolName != "ask_advice" {
		t.Fatalf("expected ask_advice, got %q", decision.ToolName)
	}
}
