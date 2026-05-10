package whatsapp

import (
	"regexp"
	"strings"
)

var (
	reListPrefix = regexp.MustCompile(`^\s*(?:[-*]|\d+[.)])\s*`)
	reQtyPrefix  = regexp.MustCompile(`^\s*(?:an?|some|half|quarter|\d+(?:\.\d+)?)(?:\s*(?:x|pcs?|pieces?|nos?|numbers?|gm|g|kg|ml|l|tbsp|tsp|cup|cups))?\s*(?:of\s+)?`)
)

func canonicalDishName(dishName, mealType, ingredients string) string {
	name := strings.TrimSpace(dishName)
	if !isGenericMealLabel(name) {
		return name
	}
	derived := deriveDishNameFromIngredients(ingredients)
	if derived != "" {
		return derived
	}
	if strings.TrimSpace(mealType) == "" {
		return "Mixed Meal"
	}
	return "Mixed " + strings.Title(strings.ToLower(strings.TrimSpace(mealType)))
}

func isGenericMealLabel(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	return n == "" || n == "breakfast" || n == "lunch" || n == "snack" || n == "dinner" || n == "meal"
}

func deriveDishNameFromIngredients(ingredients string) string {
	if strings.TrimSpace(ingredients) == "" {
		return ""
	}
	replacer := strings.NewReplacer("\n", ",", ";", ",")
	raw := strings.Split(replacer.Replace(ingredients), ",")
	parts := make([]string, 0, len(raw))
	for _, p := range raw {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		s = reListPrefix.ReplaceAllString(s, "")
		s = reQtyPrefix.ReplaceAllString(strings.ToLower(s), "")
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		parts = append(parts, strings.Title(s))
		if len(parts) == 3 {
			break
		}
	}
	return strings.Join(parts, " + ")
}
