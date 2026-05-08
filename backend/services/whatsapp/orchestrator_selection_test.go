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

func TestResolveMealChoiceFromText_DoesNotSubstringMatch(t *testing.T) {
	ids := []uint{11, 22}

	if got, ok := resolveMealChoiceFromText("10", ids); ok {
		t.Fatalf("expected no match for \"10\" with 2 options, got id=%d", got)
	}
}

func TestResolveMealChoiceFromText_StrictWordsOnly(t *testing.T) {
	ids := []uint{11, 22, 33}

	got, ok := resolveMealChoiceFromText("first", ids)
	if !ok || got != 11 {
		t.Fatalf("expected first -> 11, got ok=%v id=%d", ok, got)
	}

	if _, ok := resolveMealChoiceFromText("first one", ids); ok {
		t.Fatalf("expected strict parser to reject \"first one\"")
	}
}
