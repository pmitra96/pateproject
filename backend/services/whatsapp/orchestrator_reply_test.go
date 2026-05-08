package whatsapp

import (
	"strings"
	"testing"
)

func TestDeterministicToolReply_LogMealsReturnsHumanText(t *testing.T) {
	reply := deterministicToolReply([]toolExecutionResult{
		{
			ToolName: "log_meals",
			Response: map[string]any{
				"ok": true,
				"logged_meals": []any{
					map[string]any{
						"ok":          true,
						"dish_name":   "Curd",
						"display_time": "May 07, 2026 12:20 PM IST",
						"calories":    120.0,
						"protein":     4.0,
						"carbs":       22.0,
						"fat":         2.0,
						"fiber":       3.0,
					},
				},
				"remaining": map[string]any{
					"calories": 1760.0,
					"protein":  142.0,
					"carbs":    156.0,
					"fat":      61.0,
					"fiber":    22.0,
				},
			},
		},
	})

	if reply == "" || reply == "Done." {
		t.Fatalf("expected human-readable reply, got %q", reply)
	}
	lower := strings.ToLower(reply)
	if strings.Contains(lower, "{") || strings.Contains(lower, "\"logged_meals\"") {
		t.Fatalf("reply should not leak raw JSON: %q", reply)
	}
	if !strings.Contains(lower, "logged meals") {
		t.Fatalf("expected summary header in reply: %q", reply)
	}
}

func TestDeterministicToolReply_ModifyDeleteReturnsHumanText(t *testing.T) {
	reply := deterministicToolReply([]toolExecutionResult{
		{
			ToolName: "modify_logged_meal",
			Response: map[string]any{
				"ok":        true,
				"action":    "delete",
				"dish_name": "Curd",
				"meal_type": "Breakfast",
				"meal_id":   1.0,
			},
		},
	})

	lower := strings.ToLower(reply)
	if strings.Contains(lower, "{") || strings.Contains(lower, "\"action\"") {
		t.Fatalf("reply should not leak raw JSON: %q", reply)
	}
	if !strings.Contains(lower, "deleted curd from breakfast") {
		t.Fatalf("unexpected reply: %q", reply)
	}
}
