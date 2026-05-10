package services

import (
	"regexp"
	"strconv"
	"strings"
)

type MealQuantity struct {
	Value     float64
	Unit      string
	BaseValue float64
	BaseUnit  string
}

var mealQuantityPattern = regexp.MustCompile(`(?i)^\s*(\d+(?:\.\d+)?)\s*(x|g|gm|gram|grams|kg|ml|l|litre|liter|cup|cups|tbsp|tablespoon|tablespoons|tsp|teaspoon|teaspoons|piece|pieces|pc|pcs|no|nos|number|numbers|serving|servings|bowl|bowls|plate|plates)?\s*$`)
var mealQuantityInlinePattern = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(x|g|gm|gram|grams|kg|ml|l|litre|liter|cup|cups|tbsp|tablespoon|tablespoons|tsp|teaspoon|teaspoons|piece|pieces|pc|pcs|no|nos|number|numbers|serving|servings|bowl|bowls|plate|plates)\b`)
var mealQuantityInlineSpacedXPattern = regexp.MustCompile(`(?i)\b(\d+(?:\.\d+)?)\s*x\b`)

func ParseMealQuantity(servingSize string) MealQuantity {
	defaultQty := MealQuantity{
		Value:     1,
		Unit:      "pcs",
		BaseValue: 1,
		BaseUnit:  "pcs",
	}
	raw := strings.TrimSpace(strings.ToLower(servingSize))
	if raw == "" {
		return defaultQty
	}

	matches := mealQuantityPattern.FindStringSubmatch(raw)
	if len(matches) != 3 {
		return defaultQty
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil || value <= 0 {
		return defaultQty
	}

	unit := canonicalMealUnit(matches[2])
	baseValue, baseUnit := normalizeMealQuantity(value, unit)

	return MealQuantity{
		Value:     value,
		Unit:      unit,
		BaseValue: baseValue,
		BaseUnit:  baseUnit,
	}
}

// ParseMealQuantityFromText extracts the first explicit quantity mention from free text.
// Examples: "350g of poha", "2 x eggs", "1 cup oats".
func ParseMealQuantityFromText(text string) (MealQuantity, bool) {
	raw := strings.TrimSpace(strings.ToLower(text))
	if raw == "" {
		return MealQuantity{}, false
	}
	if m := mealQuantityInlineSpacedXPattern.FindStringSubmatch(raw); len(m) == 2 {
		value, err := strconv.ParseFloat(m[1], 64)
		if err == nil && value > 0 {
			baseValue, baseUnit := normalizeMealQuantity(value, "pcs")
			return MealQuantity{
				Value:     value,
				Unit:      "pcs",
				BaseValue: baseValue,
				BaseUnit:  baseUnit,
			}, true
		}
	}
	m := mealQuantityInlinePattern.FindStringSubmatch(raw)
	if len(m) != 3 {
		return MealQuantity{}, false
	}
	value, err := strconv.ParseFloat(m[1], 64)
	if err != nil || value <= 0 {
		return MealQuantity{}, false
	}
	unit := canonicalMealUnit(m[2])
	baseValue, baseUnit := normalizeMealQuantity(value, unit)
	return MealQuantity{
		Value:     value,
		Unit:      unit,
		BaseValue: baseValue,
		BaseUnit:  baseUnit,
	}, true
}

func canonicalMealUnit(unit string) string {
	switch strings.TrimSpace(strings.ToLower(unit)) {
	case "", "x", "pc", "piece", "pieces", "no", "nos", "number", "numbers":
		return "pcs"
	case "gm", "gram", "grams":
		return "g"
	case "l", "litre", "liter":
		return "ml"
	case "cup", "cups":
		return "cup"
	case "tbsp", "tablespoon", "tablespoons":
		return "tbsp"
	case "tsp", "teaspoon", "teaspoons":
		return "tsp"
	case "serving", "servings":
		return "serving"
	case "bowl", "bowls":
		return "bowl"
	case "plate", "plates":
		return "plate"
	default:
		return unit
	}
}

func normalizeMealQuantity(value float64, unit string) (float64, string) {
	switch unit {
	case "kg":
		return value * 1000, "g"
	case "l":
		return value * 1000, "ml"
	case "cup":
		return value * 240, "ml"
	case "tbsp":
		return value * 15, "ml"
	case "tsp":
		return value * 5, "ml"
	case "bowl":
		return value * 150, "g"
	case "plate":
		return value * 250, "g"
	case "serving":
		return value, "serving"
	default:
		return value, unit
	}
}
