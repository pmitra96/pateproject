package services

import "testing"

func TestParseMealQuantityFromText_InlineAndXForms(t *testing.T) {
	q, ok := ParseMealQuantityFromText("i had 350g of poha")
	if !ok || q.Value != 350 || q.Unit != "g" {
		t.Fatalf("unexpected quantity: ok=%v q=%+v", ok, q)
	}

	q, ok = ParseMealQuantityFromText("2 x boiled eggs")
	if !ok || q.Value != 2 || q.Unit != "pcs" {
		t.Fatalf("unexpected x quantity: ok=%v q=%+v", ok, q)
	}

	q, ok = ParseMealQuantityFromText("1 cup oats")
	if !ok || q.Value != 1 || q.Unit != "cup" {
		t.Fatalf("unexpected cup quantity: ok=%v q=%+v", ok, q)
	}
}

func TestParseMealQuantityFromText_Malformed(t *testing.T) {
	if _, ok := ParseMealQuantityFromText("about poha only"); ok {
		t.Fatalf("expected no quantity")
	}
	if _, ok := ParseMealQuantityFromText("g350 poha"); ok {
		t.Fatalf("expected malformed quantity to fail")
	}
}
