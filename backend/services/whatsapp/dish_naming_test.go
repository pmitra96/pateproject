package whatsapp

import "testing"

func TestCanonicalDishName_ReplacesGenericLabel(t *testing.T) {
	got := canonicalDishName("Dinner", "Dinner", "1 carrot, half capsicum, 100 gm of hello tempay tempeh")
	want := "Carrot + Capsicum + Hello Tempay Tempeh"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestCanonicalDishName_KeepsSpecificName(t *testing.T) {
	got := canonicalDishName("Masala Dosa", "Breakfast", "dosa, aloo")
	if got != "Masala Dosa" {
		t.Fatalf("expected specific dish name to be preserved, got %q", got)
	}
}
