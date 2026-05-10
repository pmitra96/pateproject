package whatsapp

import "testing"

func TestMealMutationV1Validate_AddRequiresQuantity(t *testing.T) {
	m := &MealMutationV1{
		Action: MutationAdd,
		Input: &MealInputV1{
			MealType:      "Breakfast",
			ItemName:      "Poha",
			QuantityValue: 0,
			QuantityUnit:  "",
		},
	}
	if err := m.Validate(); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestMealMutationV1Validate_DeleteSelectorRequired(t *testing.T) {
	m := &MealMutationV1{Action: MutationDelete}
	if err := m.Validate(); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestMealMutationV1Validate_UpdateOK(t *testing.T) {
	m := &MealMutationV1{
		Action: MutationUpdate,
		Input: &MealInputV1{
			MealType:      "Breakfast",
			ItemName:      "Poha",
			QuantityValue: 350,
			QuantityUnit:  "g",
		},
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
