package whatsapp

import "testing"

func TestSanitizeServingSize_RejectsIngredientBlob(t *testing.T) {
	canonical := MealInputV1{
		QuantityValue: 5,
		QuantityUnit:  "pcs",
	}
	got := sanitizeServingSize("2 whole eggs, 3 egg whites, little onion, little tomato, 1 tsp oil", canonical)
	if got != "5 pcs" {
		t.Fatalf("expected fallback quantity label, got %q", got)
	}
}

func TestSanitizeServingSize_RejectsOverlongValue(t *testing.T) {
	canonical := MealInputV1{
		QuantityValue: 350,
		QuantityUnit:  "g",
	}
	got := sanitizeServingSize("this is a very long serving size that should never be persisted as serving size because it exceeds the schema limit", canonical)
	if got != "350 g" {
		t.Fatalf("expected canonical fallback, got %q", got)
	}
}

func TestSanitizeServingSize_PreservesValidServing(t *testing.T) {
	canonical := MealInputV1{
		QuantityValue: 1,
		QuantityUnit:  "serving",
	}
	got := sanitizeServingSize("100 g", canonical)
	if got != "100 g" {
		t.Fatalf("expected original serving size, got %q", got)
	}
}

