package whatsapp

import (
	"fmt"
	"math"
	"strings"
)

const maxServingSizeLen = 64

func sanitizeServingSize(estimated string, canonical MealInputV1) string {
	candidate := strings.TrimSpace(estimated)
	if isSafeServingSize(candidate) {
		return candidate
	}
	fallback := canonicalQuantityLabel(canonical.QuantityValue, canonical.QuantityUnit)
	if isSafeServingSize(fallback) {
		return fallback
	}
	return "1 serving"
}

func isSafeServingSize(v string) bool {
	if strings.TrimSpace(v) == "" {
		return false
	}
	if len(v) > maxServingSizeLen {
		return false
	}
	if strings.ContainsAny(v, "\n\r\t") {
		return false
	}
	// Ingredient-like strings must never be persisted as serving size.
	if strings.Contains(v, ",") {
		return false
	}
	return true
}

func canonicalQuantityLabel(value float64, unit string) string {
	u := strings.TrimSpace(unit)
	if value <= 0 || u == "" {
		return "1 serving"
	}
	if math.Abs(value-math.Round(value)) < 1e-6 {
		return fmt.Sprintf("%.0f %s", value, u)
	}
	return fmt.Sprintf("%.2f %s", value, u)
}

