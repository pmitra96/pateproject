package services

import "testing"

func TestNormalizeServingQuery_DefaultsToServingForBareFruit(t *testing.T) {
	n := NormalizeServingQuery("papaya")
	if n.HasExplicitQuantity {
		t.Fatalf("expected bare papaya to be treated as implicit serving")
	}
	if n.AssumedUnit != "g" {
		t.Fatalf("expected grams, got %q", n.AssumedUnit)
	}
	if n.AssumedAmount != 100 {
		t.Fatalf("expected 100g default, got %.0f", n.AssumedAmount)
	}
	if n.AssumedServing == "" {
		t.Fatalf("expected assumed serving to be set")
	}
}

func TestNormalizeServingQuery_RespectsExplicitQuantity(t *testing.T) {
	n := NormalizeServingQuery("2 cups papaya")
	if !n.HasExplicitQuantity {
		t.Fatalf("expected explicit quantity to be detected")
	}
	if n.AssumedServing != "2 cups papaya" {
		t.Fatalf("expected explicit serving to be preserved, got %q", n.AssumedServing)
	}
}

func TestNormalizeServingQuery_DefaultsLiquidsToMl(t *testing.T) {
	n := NormalizeServingQuery("milk")
	if n.AssumedUnit != "ml" {
		t.Fatalf("expected ml for liquid default, got %q", n.AssumedUnit)
	}
	if n.AssumedAmount != 200 {
		t.Fatalf("expected 200ml default, got %.0f", n.AssumedAmount)
	}
}
