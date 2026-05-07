package whatsapp

import "testing"

func TestResolveMealChoiceFromText_ZeroBasedIndex(t *testing.T) {
	ids := []uint{101, 202, 303}

	got, ok := resolveMealChoiceFromText("0", ids)
	if !ok || got != 101 {
		t.Fatalf("expected index 0 to map to first ID, got ok=%v id=%d", ok, got)
	}

	got, ok = resolveMealChoiceFromText("2", ids)
	if !ok || got != 303 {
		t.Fatalf("expected index 2 to map to third ID, got ok=%v id=%d", ok, got)
	}

	_, ok = resolveMealChoiceFromText("3", ids)
	if ok {
		t.Fatalf("expected out-of-range index to fail")
	}
}

