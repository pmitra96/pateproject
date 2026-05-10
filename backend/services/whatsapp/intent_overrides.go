package whatsapp

import (
	"regexp"
	"strings"
)

var mealTypeDeleteRe = regexp.MustCompile(`(?i)^(?:delete|remove)\s+(breakfast|lunch|dinner|snack)\b`)
var deleteByIDRe = regexp.MustCompile(`(?i)^(?:delete|remove)\s+(?:meal\s+)?(\d+)$`)

func classifyIntentOverrides(text string) (IntentResult, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return IntentResult{Intent: IntentFallback, Confidence: 1, MissingSlots: []string{"request"}, Entities: map[string]any{}}, true
	}

	switch lower {
	case "help":
		return IntentResult{Intent: IntentHelp, Confidence: 1, Entities: map[string]any{}}, true
	case "clear all meals today", "delete all meals today", "clear my day", "delete all meals for the day":
		return IntentResult{Intent: IntentDeleteMeal, Confidence: 1, Entities: map[string]any{"scope": "all_today"}}, true
	}

	if m := mealTypeDeleteRe.FindStringSubmatch(lower); len(m) > 1 {
		return IntentResult{
			Intent:     IntentDeleteMeal,
			Confidence: 1,
			Entities: map[string]any{
				"meal_type": strings.Title(strings.ToLower(m[1])),
				"scope":     "meal_type",
			},
			MissingSlots: []string{"delete_scope_choice"},
		}, true
	}

	if m := deleteByIDRe.FindStringSubmatch(lower); len(m) > 1 {
		return IntentResult{
			Intent:     IntentDeleteMeal,
			Confidence: 1,
			Entities:   map[string]any{"meal_id_raw": m[1], "scope": "single_id"},
		}, true
	}

	return IntentResult{}, false
}
