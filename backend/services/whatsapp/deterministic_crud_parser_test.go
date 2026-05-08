package whatsapp

import "testing"

func TestParseDeterministicCRUD_LogMealsFromList(t *testing.T) {
	msg := "for breakfast i had\n1. 100g curd\n2. 50g oats"
	decision, ok := parseDeterministicCRUD(msg)
	if !ok {
		t.Fatalf("expected deterministic parse")
	}
	if decision.ToolName != "log_meals" {
		t.Fatalf("expected log_meals, got %q", decision.ToolName)
	}
	meals, ok := decision.Args["meals"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected meals payload")
	}
	if len(meals) != 2 {
		t.Fatalf("expected 2 meals, got %d", len(meals))
	}
	if meals[0]["meal_type"] != "Breakfast" {
		t.Fatalf("expected Breakfast meal_type, got %#v", meals[0]["meal_type"])
	}
}

func TestParseDeterministicCRUD_DeleteMeal(t *testing.T) {
	msg := "delete curd rice from lunch"
	decision, ok := parseDeterministicCRUD(msg)
	if !ok {
		t.Fatalf("expected deterministic parse")
	}
	if decision.ToolName != "modify_logged_meal" {
		t.Fatalf("expected modify_logged_meal, got %q", decision.ToolName)
	}
	if decision.Args["action"] != "delete" {
		t.Fatalf("expected delete action, got %#v", decision.Args["action"])
	}
	if decision.Args["meal_type"] != "Lunch" {
		t.Fatalf("expected lunch meal_type, got %#v", decision.Args["meal_type"])
	}
	if decision.Args["target_dish_name"] != "curd rice" {
		t.Fatalf("unexpected target dish %#v", decision.Args["target_dish_name"])
	}
}

func TestParseDeterministicCRUD_UpdateMealQuantityPhrase(t *testing.T) {
	msg := "curd rice is actually 50gm"
	decision, ok := parseDeterministicCRUD(msg)
	if !ok {
		t.Fatalf("expected deterministic parse")
	}
	if decision.ToolName != "modify_logged_meal" {
		t.Fatalf("expected modify_logged_meal, got %q", decision.ToolName)
	}
	if decision.Args["action"] != "update" {
		t.Fatalf("expected update action, got %#v", decision.Args["action"])
	}
	if decision.Args["target_dish_name"] != "curd rice" {
		t.Fatalf("unexpected target dish %#v", decision.Args["target_dish_name"])
	}
	newIngredients, _ := decision.Args["new_ingredients"].(string)
	if newIngredients != "50gm of curd rice" {
		t.Fatalf("unexpected new_ingredients %q", newIngredients)
	}
}

func TestParseDeterministicCRUD_UpdateMealQuantityPhraseCaseInsensitive(t *testing.T) {
	msg := "Curd Rice IS ACTUALLY 50gm"
	decision, ok := parseDeterministicCRUD(msg)
	if !ok {
		t.Fatalf("expected deterministic parse")
	}
	if decision.ToolName != "modify_logged_meal" {
		t.Fatalf("expected modify_logged_meal, got %q", decision.ToolName)
	}
	if decision.Args["action"] != "update" {
		t.Fatalf("expected update action, got %#v", decision.Args["action"])
	}
	if decision.Args["target_dish_name"] != "Curd Rice" {
		t.Fatalf("unexpected target dish %#v", decision.Args["target_dish_name"])
	}
	newIngredients, _ := decision.Args["new_ingredients"].(string)
	if newIngredients != "50gm of Curd Rice" {
		t.Fatalf("unexpected new_ingredients %q", newIngredients)
	}
}

func TestParseDeterministicCRUD_UpdateMealNoItIsPhrase(t *testing.T) {
	msg := "no it is 30g of whey protein"
	decision, ok := parseDeterministicCRUD(msg)
	if !ok {
		t.Fatalf("expected deterministic parse")
	}
	if decision.ToolName != "modify_logged_meal" {
		t.Fatalf("expected modify_logged_meal, got %q", decision.ToolName)
	}
	if decision.Args["action"] != "update" {
		t.Fatalf("expected update action, got %#v", decision.Args["action"])
	}
	if decision.Args["target_dish_name"] != "whey protein" {
		t.Fatalf("unexpected target dish %#v", decision.Args["target_dish_name"])
	}
	newIngredients, _ := decision.Args["new_ingredients"].(string)
	if newIngredients != "30g of whey protein" {
		t.Fatalf("unexpected new_ingredients %q", newIngredients)
	}
}

func TestParseDeterministicCRUD_UpdateMealShouldBePhrase(t *testing.T) {
	msg := "it should be 30g of whey protein"
	decision, ok := parseDeterministicCRUD(msg)
	if !ok {
		t.Fatalf("expected deterministic parse")
	}
	if decision.ToolName != "modify_logged_meal" {
		t.Fatalf("expected modify_logged_meal, got %q", decision.ToolName)
	}
	if decision.Args["action"] != "update" {
		t.Fatalf("expected update action, got %#v", decision.Args["action"])
	}
	if decision.Args["target_dish_name"] != "whey protein" {
		t.Fatalf("unexpected target dish %#v", decision.Args["target_dish_name"])
	}
	newIngredients, _ := decision.Args["new_ingredients"].(string)
	if newIngredients != "30g of whey protein" {
		t.Fatalf("unexpected new_ingredients %q", newIngredients)
	}
}

func TestParseDeterministicCRUD_SkipsPantryMessage(t *testing.T) {
	msg := "add 1kg rice to pantry"
	_, ok := parseDeterministicCRUD(msg)
	if ok {
		t.Fatalf("expected pantry message to skip deterministic meal CRUD parser")
	}
}
