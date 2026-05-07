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
