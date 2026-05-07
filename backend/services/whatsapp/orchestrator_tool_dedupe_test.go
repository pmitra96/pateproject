package whatsapp

import (
	"testing"

	"github.com/pmitra96/pateproject/llm"
)

func TestDedupeToolCalls_SkipsExactDuplicatesWithDifferentArgOrder(t *testing.T) {
	calls := []llm.ToolCall{
		{
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "modify_logged_meal",
				Arguments: `{"action":"delete","meal_id":123}`,
			},
		},
		{
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "modify_logged_meal",
				Arguments: `{"meal_id":123,"action":"delete"}`,
			},
		},
		{
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "get_daily_summary",
				Arguments: `{}`,
			},
		},
	}

	unique, skipped := dedupeToolCalls(calls)
	if skipped != 1 {
		t.Fatalf("expected 1 skipped duplicate, got %d", skipped)
	}
	if len(unique) != 2 {
		t.Fatalf("expected 2 unique calls, got %d", len(unique))
	}
}

func TestDedupeToolCalls_KeepsDifferentArguments(t *testing.T) {
	calls := []llm.ToolCall{
		{
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "modify_logged_meal",
				Arguments: `{"action":"delete","meal_id":1}`,
			},
		},
		{
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "modify_logged_meal",
				Arguments: `{"action":"delete","meal_id":2}`,
			},
		},
	}

	unique, skipped := dedupeToolCalls(calls)
	if skipped != 0 {
		t.Fatalf("expected 0 skipped duplicates, got %d", skipped)
	}
	if len(unique) != 2 {
		t.Fatalf("expected 2 unique calls, got %d", len(unique))
	}
}
