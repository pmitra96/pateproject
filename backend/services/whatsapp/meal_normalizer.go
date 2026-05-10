package whatsapp

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/pmitra96/pateproject/services"
)

var quantityPrefixStripper = regexp.MustCompile(`(?i)^\s*(?:about|around)?\s*(?:half|quarter|\d+(?:\.\d+)?)\s*(?:x|g|gm|gram|grams|kg|ml|l|litre|liter|cup|cups|tbsp|tablespoon|tablespoons|tsp|teaspoon|teaspoons|piece|pieces|pc|pcs|no|nos|number|numbers|serving|servings|bowl|bowls|plate|plates)?\s*(?:of\s+)?`)

func normalizeMealInputV1(traceID, source, rawText, dishName, ingredients, mealType, fallbackServing string) MealInputV1 {
	raw := strings.TrimSpace(rawText)
	if raw == "" {
		raw = strings.TrimSpace(ingredients)
	}
	qty, ok := services.ParseMealQuantityFromText(ingredients)
	if !ok {
		qty, ok = services.ParseMealQuantityFromText(raw)
	}
	if !ok {
		qty, ok = services.ParseMealQuantityFromText(dishName)
	}
	assumptions := ""
	if !ok && strings.TrimSpace(fallbackServing) != "" {
		qty = services.ParseMealQuantity(fallbackServing)
		ok = true
		assumptions = "quantity inferred from estimator serving size"
	}
	if !ok {
		qty = services.MealQuantity{Value: 1, Unit: "serving", BaseValue: 1, BaseUnit: "serving"}
		assumptions = "quantity defaulted to 1 serving"
	}

	item := strings.TrimSpace(dishName)
	if item == "" {
		item = strings.TrimSpace(ingredients)
	}
	item = quantityPrefixStripper.ReplaceAllString(item, "")
	item = strings.TrimSpace(item)
	if item == "" {
		item = quantityPrefixStripper.ReplaceAllString(strings.TrimSpace(ingredients), "")
		item = strings.TrimSpace(item)
	}
	if item == "" {
		item = "Mixed Meal"
	}
	item = strings.Join(strings.Fields(item), " ")
	item = strings.Title(strings.ToLower(item))

	return MealInputV1{
		TraceID:           strings.TrimSpace(traceID),
		Source:            strings.TrimSpace(source),
		RawText:           raw,
		MealType:          strings.TrimSpace(mealType),
		ItemName:          item,
		QuantityValue:     qty.Value,
		QuantityUnit:      qty.Unit,
		QuantityBaseValue: qty.BaseValue,
		QuantityBaseUnit:  qty.BaseUnit,
		Confidence:        0.9,
		Assumptions:       assumptions,
	}
}

func mutationIdempotencyKey(messageID string, m MealMutationV1) string {
	seed := strings.TrimSpace(messageID) + "|" + strings.TrimSpace(m.TraceID) + "|" + string(m.Action) + "|" + strings.TrimSpace(m.MealType) + "|" + strings.TrimSpace(m.TargetDishName)
	if m.MealID > 0 {
		seed += "|id"
	}
	if m.Input != nil {
		seed += "|" + strings.TrimSpace(m.Input.ItemName) + "|" + strings.TrimSpace(m.Input.RawText)
	}
	h := sha256.Sum256([]byte(seed))
	return "mut:" + hex.EncodeToString(h[:12])
}
