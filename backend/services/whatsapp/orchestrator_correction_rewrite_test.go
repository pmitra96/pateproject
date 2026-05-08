package whatsapp

import (
	"testing"

	"github.com/pmitra96/pateproject/llm"
)

func TestRewriteCorrectionLogMisfire_RewritesLogMealsToModify(t *testing.T) {
	tc := llm.ToolCall{
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      "log_meals",
			Arguments: `{"meals":[{"dish_name":"Whey Protein","ingredients":"30g whey protein","meal_type":"Snack"}]}`,
		},
	}

	rewritten := rewriteCorrectionLogMisfire("no it is 30g of whey protein", tc)
	if rewritten.Function.Name != "modify_logged_meal" {
		t.Fatalf("expected modify_logged_meal, got %q", rewritten.Function.Name)
	}
}

func TestRewriteCorrectionLogMisfire_LeavesNonCorrectionUntouched(t *testing.T) {
	tc := llm.ToolCall{
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      "log_meals",
			Arguments: `{"meals":[{"dish_name":"Papaya","ingredients":"100g papaya","meal_type":"Snack"}]}`,
		},
	}

	rewritten := rewriteCorrectionLogMisfire("for snack i had papaya", tc)
	if rewritten.Function.Name != "log_meals" {
		t.Fatalf("expected log_meals, got %q", rewritten.Function.Name)
	}
}

func TestShouldBlockCorrectionLogMisfire_BlocksAmbiguousCorrection(t *testing.T) {
	tc := llm.ToolCall{
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      "log_meals",
			Arguments: `{"meals":[{"dish_name":"Whey Protein","ingredients":"30g whey protein","meal_type":"Snack"}]}`,
		},
	}

	if !shouldBlockCorrectionLogMisfire("whey is wrong", tc) {
		t.Fatalf("expected correction log misfire to be blocked")
	}
}

func TestShouldBlockCorrectionLogMisfire_DoesNotBlockDeterministicCorrection(t *testing.T) {
	tc := llm.ToolCall{
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      "log_meals",
			Arguments: `{"meals":[{"dish_name":"Whey Protein","ingredients":"30g whey protein","meal_type":"Snack"}]}`,
		},
	}

	if shouldBlockCorrectionLogMisfire("no it is 30g of whey protein", tc) {
		t.Fatalf("did not expect deterministic correction to be blocked")
	}
}

func TestShouldBlockCorrectionLogMisfire_ExpandedCorrectionPhrases(t *testing.T) {
	tc := llm.ToolCall{
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      "log_meals",
			Arguments: `{"meals":[{"dish_name":"Whey Protein","ingredients":"30g whey protein","meal_type":"Snack"}]}`,
		},
	}

	blockedPhrases := []string{
		"that's wrong",
		"replace with 30g whey protein",
		"that was 30g whey protein",
	}
	for _, p := range blockedPhrases {
		if !shouldBlockCorrectionLogMisfire(p, tc) {
			t.Fatalf("expected phrase to be blocked: %q", p)
		}
	}

	nonBlockedPhrases := []string{
		"it should be 30g whey protein",
		"meant 30g whey protein",
		"it's 30g whey protein",
	}
	for _, p := range nonBlockedPhrases {
		if shouldBlockCorrectionLogMisfire(p, tc) {
			t.Fatalf("expected phrase to be rewritten, not blocked: %q", p)
		}
	}
}

func TestIsMealCreateToolCall(t *testing.T) {
	logCall := llm.ToolCall{
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      "log_meals",
			Arguments: `{"meals":[{"dish_name":"Papaya","ingredients":"100g papaya","meal_type":"Snack"}]}`,
		},
	}
	if !isMealCreateToolCall(logCall) {
		t.Fatalf("expected log_meals to be treated as create")
	}

	addCall := llm.ToolCall{
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      "modify_logged_meal",
			Arguments: `{"action":"add","meal_type":"Snack","new_ingredients":"papaya"}`,
		},
	}
	if !isMealCreateToolCall(addCall) {
		t.Fatalf("expected modify add to be treated as create")
	}

	updateCall := llm.ToolCall{
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      "modify_logged_meal",
			Arguments: `{"action":"update","meal_type":"Snack","target_dish_name":"Papaya","new_ingredients":"120g papaya"}`,
		},
	}
	if isMealCreateToolCall(updateCall) {
		t.Fatalf("did not expect modify update to be treated as create")
	}
}
