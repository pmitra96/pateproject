package whatsapp

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	reMealType = regexp.MustCompile(`(?i)\b(breakfast|lunch|dinner|snack)\b`)
	reBullet   = regexp.MustCompile(`^\s*(?:\d+[\)\.\-:]?\s*|[-*]\s*)`)
	reQtyOnly  = regexp.MustCompile(`(?i)^\s*(?:half|quarter|\d+(?:\.\d+)?)\s*(?:g|gm|kg|ml|l|tsp|tbsp|cup|cups|slice|slices|piece|pieces|serving|servings)?\s*$`)
)

func parseDeterministicCRUD(text string) (RouteDecision, bool) {
	raw := strings.TrimSpace(text)
	lower := strings.ToLower(raw)
	if raw == "" {
		return RouteDecision{}, false
	}
	if strings.Contains(lower, "pantry") || strings.Contains(lower, "recipe") {
		return RouteDecision{}, false
	}

	mealType := detectMealType(raw)

	if isDeleteIntent(lower) {
		args := map[string]interface{}{
			"action":    "delete",
			"meal_type": mealType,
		}
		if id := extractMealID(lower); id > 0 {
			args["meal_id"] = float64(id)
		}
		if target := extractDeleteTarget(raw, mealType); target != "" {
			args["target_dish_name"] = target
		}
		return RouteDecision{
			Intent:   "modify_meal_delete",
			ToolName: "modify_logged_meal",
			Args:     args,
		}, true
	}

	if isUpdateIntent(lower) {
		target, replacement := extractUpdateTargetAndReplacement(raw, mealType)
		if target != "" && replacement != "" {
			args := map[string]interface{}{
				"action":           "update",
				"meal_type":        mealType,
				"target_dish_name": target,
				"new_ingredients":  replacement,
			}
			return RouteDecision{
				Intent:   "modify_meal_update",
				ToolName: "modify_logged_meal",
				Args:     args,
			}, true
		}
	}

	if mealType == "" {
		return RouteDecision{}, false
	}
	if !isMealLogIntent(lower) {
		return RouteDecision{}, false
	}

	items := extractMealItems(raw, mealType)
	if len(items) == 0 {
		return RouteDecision{}, false
	}

	meals := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		item = cleanFoodPhrase(item, mealType)
		if item == "" {
			continue
		}
		meals = append(meals, map[string]interface{}{
			"dish_name":   buildDishName(item),
			"ingredients": item,
			"meal_type":   mealType,
		})
	}
	if len(meals) == 0 {
		return RouteDecision{}, false
	}

	return RouteDecision{
		Intent:   "log_meals",
		ToolName: "log_meals",
		Args: map[string]interface{}{
			"meals": meals,
		},
	}, true
}

func isDeleteIntent(lower string) bool {
	return strings.Contains(lower, "delete") || strings.Contains(lower, "remove")
}

func isUpdateIntent(lower string) bool {
	if strings.Contains(lower, " is actually ") {
		return true
	}
	if strings.HasPrefix(lower, "no it is ") || strings.HasPrefix(lower, "it is ") || strings.HasPrefix(lower, "it's ") {
		return true
	}
	return strings.Contains(lower, " change ") ||
		strings.HasPrefix(lower, "change ") ||
		strings.Contains(lower, " update ") ||
		strings.HasPrefix(lower, "update ") ||
		strings.Contains(lower, " correct ") ||
		strings.HasPrefix(lower, "correct ") ||
		strings.Contains(lower, " wrong") ||
		strings.Contains(lower, " incorrect") ||
		strings.Contains(lower, " typo") ||
		strings.Contains(lower, " should be ") ||
		strings.HasPrefix(lower, "should be ") ||
		strings.Contains(lower, " meant ") ||
		strings.HasPrefix(lower, "meant ") ||
		strings.Contains(lower, " instead ") ||
		strings.Contains(lower, " replace ") ||
		strings.HasPrefix(lower, "replace ") ||
		strings.Contains(lower, " that was ") ||
		strings.HasPrefix(lower, "that was ") ||
		strings.HasPrefix(lower, "make ")
}

func isMealLogIntent(lower string) bool {
	return strings.Contains(lower, " i had") ||
		strings.HasPrefix(lower, "had ") ||
		strings.HasPrefix(lower, "ate ") ||
		strings.Contains(lower, " i ate") ||
		strings.Contains(lower, " for breakfast") ||
		strings.Contains(lower, " for lunch") ||
		strings.Contains(lower, " for dinner") ||
		strings.Contains(lower, " for snack") ||
		strings.HasPrefix(lower, "add ")
}

func detectMealType(text string) string {
	m := reMealType.FindStringSubmatch(strings.ToLower(text))
	if len(m) < 2 {
		return ""
	}
	switch strings.ToLower(m[1]) {
	case "breakfast":
		return "Breakfast"
	case "lunch":
		return "Lunch"
	case "dinner":
		return "Dinner"
	case "snack":
		return "Snack"
	default:
		return ""
	}
}

func extractMealID(lower string) int {
	re := regexp.MustCompile(`(?i)\bmeal\s*id\s*(\d+)\b`)
	m := re.FindStringSubmatch(lower)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func extractDeleteTarget(text, mealType string) string {
	lower := strings.ToLower(text)
	idx := strings.Index(lower, "delete")
	verbLen := len("delete")
	if idx < 0 {
		idx = strings.Index(lower, "remove")
		verbLen = len("remove")
	}
	if idx < 0 {
		return ""
	}
	target := strings.TrimSpace(text[idx+verbLen:])
	target = cleanFoodPhrase(target, mealType)
	if strings.HasPrefix(strings.ToLower(target), "meal id") {
		return ""
	}
	return target
}

func extractUpdateTargetAndReplacement(text, mealType string) (string, string) {
	raw := strings.TrimSpace(text)
	reActually := regexp.MustCompile(`(?i)^(.*?)\s+is actually\s+(.+)$`)
	if m := reActually.FindStringSubmatch(raw); len(m) == 3 {
		target := cleanFoodPhrase(m[1], mealType)
		repl := cleanFoodPhrase(m[2], mealType)
		return target, coerceReplacement(target, repl)
	}

	reTo := regexp.MustCompile(`(?i)^(?:please\s+)?(?:change|update|correct|make)\s+(.+?)\s+(?:to|as)\s+(.+)$`)
	if m := reTo.FindStringSubmatch(raw); len(m) == 3 {
		target := cleanFoodPhrase(m[1], mealType)
		repl := cleanFoodPhrase(m[2], mealType)
		return target, coerceReplacement(target, repl)
	}

	reMakeQty := regexp.MustCompile(`(?i)^(?:please\s+)?make\s+(.+?)\s+((?:half|quarter|\d+(?:\.\d+)?)\s*(?:g|gm|kg|ml|l|tsp|tbsp|cup|cups|slice|slices|piece|pieces|serving|servings).*)$`)
	if m := reMakeQty.FindStringSubmatch(raw); len(m) == 3 {
		target := cleanFoodPhrase(m[1], mealType)
		repl := cleanFoodPhrase(m[2], mealType)
		return target, coerceReplacement(target, repl)
	}

	reNoItIs := regexp.MustCompile(`(?i)^(?:no\s+)?it\s+is\s+(.+)$`)
	if m := reNoItIs.FindStringSubmatch(raw); len(m) == 2 {
		repl := cleanFoodPhrase(m[1], mealType)
		target := inferTargetFromReplacement(repl)
		if target != "" && repl != "" {
			return target, coerceReplacement(target, repl)
		}
	}

	reIts := regexp.MustCompile(`(?i)^it's\s+(.+)$`)
	if m := reIts.FindStringSubmatch(raw); len(m) == 2 {
		repl := cleanFoodPhrase(m[1], mealType)
		target := inferTargetFromReplacement(repl)
		if target != "" && repl != "" {
			return target, coerceReplacement(target, repl)
		}
	}

	reShouldBe := regexp.MustCompile(`(?i)^(?:no[, ]+)?(?:it\s+)?should\s+be\s+(.+)$`)
	if m := reShouldBe.FindStringSubmatch(raw); len(m) == 2 {
		repl := cleanFoodPhrase(m[1], mealType)
		target := inferTargetFromReplacement(repl)
		if target != "" && repl != "" {
			return target, coerceReplacement(target, repl)
		}
	}

	reMeant := regexp.MustCompile(`(?i)^(?:i\s+)?meant\s+(.+)$`)
	if m := reMeant.FindStringSubmatch(raw); len(m) == 2 {
		repl := cleanFoodPhrase(m[1], mealType)
		target := inferTargetFromReplacement(repl)
		if target != "" && repl != "" {
			return target, coerceReplacement(target, repl)
		}
	}

	return "", ""
}

func inferTargetFromReplacement(replacement string) string {
	base := strings.TrimSpace(replacement)
	reQtyPrefix := regexp.MustCompile(`(?i)^\s*(?:about|around)?\s*(?:half|quarter|\d+(?:\.\d+)?)\s*(?:g|gm|kg|ml|l|tsp|tbsp|cup|cups|slice|slices|piece|pieces|serving|servings)?\s*(?:of\s+)?`)
	base = strings.TrimSpace(reQtyPrefix.ReplaceAllString(base, ""))
	base = strings.Trim(base, " .,!?:;")
	return strings.Join(strings.Fields(base), " ")
}

func coerceReplacement(target, replacement string) string {
	if target == "" || replacement == "" {
		return replacement
	}
	if reQtyOnly.MatchString(strings.ToLower(replacement)) {
		return strings.TrimSpace(replacement + " of " + target)
	}
	return replacement
}

func extractMealItems(text, mealType string) []string {
	cleaned := strings.TrimSpace(text)
	cleaned = regexp.MustCompile(`(?i)^for\s+(breakfast|lunch|dinner|snack)\s*,?\s*i\s*(?:had|ate|logged)\s*`).ReplaceAllString(cleaned, "")
	cleaned = regexp.MustCompile(`(?i)^i\s*(?:had|ate|logged)\s*`).ReplaceAllString(cleaned, "")
	cleaned = regexp.MustCompile(`(?i)^(?:for\s+)?(breakfast|lunch|dinner|snack)\s*[:\-]?\s*`).ReplaceAllString(cleaned, "")
	cleaned = regexp.MustCompile(`(?i)^add\s+`).ReplaceAllString(cleaned, "")
	cleaned = regexp.MustCompile(`(?i)\s+to\s+(breakfast|lunch|dinner|snack)\s*$`).ReplaceAllString(cleaned, "")
	cleaned = strings.TrimSpace(cleaned)

	rawItems := make([]string, 0)
	if strings.Contains(cleaned, "\n") {
		for _, line := range strings.Split(cleaned, "\n") {
			line = strings.TrimSpace(reBullet.ReplaceAllString(line, ""))
			if line == "" {
				continue
			}
			rawItems = append(rawItems, line)
		}
	} else {
		chunks := strings.Split(cleaned, ",")
		for _, c := range chunks {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			parts := strings.Split(c, " and ")
			for _, p := range parts {
				p = strings.TrimSpace(reBullet.ReplaceAllString(strings.TrimSpace(p), ""))
				if p != "" {
					rawItems = append(rawItems, p)
				}
			}
		}
	}

	items := make([]string, 0, len(rawItems))
	for _, item := range rawItems {
		item = cleanFoodPhrase(item, mealType)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func cleanFoodPhrase(s, mealType string) string {
	out := strings.TrimSpace(s)
	out = strings.Trim(out, " .,!?:;")
	out = regexp.MustCompile(`(?i)^(?:the|my)\s+`).ReplaceAllString(out, "")
	out = regexp.MustCompile(`(?i)\bfrom\s+(breakfast|lunch|dinner|snack)\b`).ReplaceAllString(out, "")
	out = regexp.MustCompile(`(?i)\bin\s+(breakfast|lunch|dinner|snack)\b`).ReplaceAllString(out, "")
	if mealType != "" {
		out = regexp.MustCompile(`(?i)\b`+strings.ToLower(mealType)+`\b`).ReplaceAllString(out, "")
	}
	out = strings.TrimSpace(out)
	out = strings.Trim(out, " .,!?:;")
	return strings.Join(strings.Fields(out), " ")
}

func buildDishName(item string) string {
	name := strings.TrimSpace(item)
	name = regexp.MustCompile(`(?i)^(?:about|around)\s+`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`(?i)^\d+(?:\.\d+)?\s*(?:g|gm|kg|ml|l|tsp|tbsp|cup|cups|slice|slices|piece|pieces|serving|servings)\s+(?:of\s+)?`).ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	if name == "" {
		name = item
	}
	parts := strings.Fields(name)
	if len(parts) > 5 {
		parts = parts[:5]
	}
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
