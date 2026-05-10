package services

import (
	"regexp"
	"strings"
)

type ServingNormalization struct {
	OriginalQuery       string
	NormalizedQuery     string
	AssumedServing      string
	AssumedAmount       float64
	AssumedUnit         string
	HasExplicitQuantity bool
	Notes               []string
}

var explicitQuantityPattern = regexp.MustCompile(`(?i)\b(\d+(?:\.\d+)?)\s*(x|g|gram|grams|kg|ml|l|litre|liter|cup|cups|bowl|bowls|glass|glasses|tbsp|tablespoon|tablespoons|tsp|teaspoon|teaspoons|piece|pieces|pc|pcs|no|nos|number|numbers|slice|slices|serving|servings|plate|plates|egg|eggs|whole|half)\b`)

func NormalizeServingQuery(query string) ServingNormalization {
	clean := strings.TrimSpace(query)
	normalized := ServingNormalization{
		OriginalQuery:   clean,
		NormalizedQuery: clean,
		AssumedServing:  "1 serving",
		AssumedAmount:   100,
		AssumedUnit:     "g",
		Notes:           []string{"defaulted_to_serving"},
	}

	if clean == "" {
		return normalized
	}

	if explicitQuantityPattern.MatchString(clean) {
		normalized.HasExplicitQuantity = true
		normalized.AssumedServing = clean
		normalized.Notes = []string{"explicit_quantity_detected"}
		return normalized
	}

	lower := strings.ToLower(clean)
	switch {
	case containsAny(lower, []string{"water", "juice", "milk", "tea", "coffee", "lassi", "smoothie", "soup", "shake", "buttermilk"}):
		normalized.AssumedServing = "1 serving"
		normalized.AssumedAmount = 200
		normalized.AssumedUnit = "ml"
		normalized.Notes = append(normalized.Notes, "liquid_default_200ml")
	case containsAny(lower, []string{"papaya", "mango", "apple", "orange", "guava", "pear", "banana", "grapes", "watermelon", "melon", "pineapple"}):
		normalized.AssumedServing = "1 standard serving"
		normalized.AssumedAmount = fruitDefaultAmount(lower)
		normalized.AssumedUnit = "g"
		normalized.Notes = append(normalized.Notes, "fruit_default")
	case containsAny(lower, []string{"rice", "dal", "sabzi", "curry", "gravy", "paneer", "chicken", "fish", "egg", "bread", "roti", "chapati", "paratha", "dosa", "idli", "upma", "poha", "biryani", "pulao", "pasta", "noodles"}):
		normalized.AssumedServing = "1 serving"
		normalized.AssumedAmount = mealDefaultAmount(lower)
		normalized.AssumedUnit = "g"
		normalized.Notes = append(normalized.Notes, "meal_default")
	default:
		normalized.AssumedServing = "1 serving"
		normalized.AssumedAmount = 100
		normalized.AssumedUnit = "g"
		normalized.Notes = append(normalized.Notes, "generic_solid_default_100g")
	}

	return normalized
}

func containsAny(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func fruitDefaultAmount(lower string) float64 {
	switch {
	case strings.Contains(lower, "banana"):
		return 120
	case strings.Contains(lower, "watermelon"):
		return 150
	case strings.Contains(lower, "grapes"):
		return 100
	case strings.Contains(lower, "papaya"):
		return 100
	default:
		return 100
	}
}

func mealDefaultAmount(lower string) float64 {
	switch {
	case strings.Contains(lower, "milk"):
		return 200
	case strings.Contains(lower, "egg"):
		return 1
	case strings.Contains(lower, "bread"):
		return 30
	case strings.Contains(lower, "roti"), strings.Contains(lower, "chapati"):
		return 35
	case strings.Contains(lower, "dosa"):
		return 120
	case strings.Contains(lower, "idli"):
		return 50
	case strings.Contains(lower, "rice"), strings.Contains(lower, "dal"), strings.Contains(lower, "pulao"), strings.Contains(lower, "biryani"), strings.Contains(lower, "pasta"), strings.Contains(lower, "noodles"):
		return 150
	case strings.Contains(lower, "paneer"), strings.Contains(lower, "chicken"), strings.Contains(lower, "fish"):
		return 100
	case strings.Contains(lower, "sabzi"), strings.Contains(lower, "curry"), strings.Contains(lower, "gravy"):
		return 150
	default:
		return 100
	}
}
