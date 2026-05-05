package whatsapp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/llm"
	"github.com/pmitra96/pateproject/models"
	"github.com/pmitra96/pateproject/services"
)

// HandleGetDailySummary returns a summary of all meals logged today
func HandleGetDailySummary(s *Session, args map[string]interface{}) (string, error) {
	todayStart := time.Now().Truncate(24 * time.Hour)
	var meals []models.MealLog
	database.DB.Where("user_id = ? AND logged_at >= ? AND logged_at < ?", s.User.ID, todayStart, todayStart.Add(24*time.Hour)).
		Order("logged_at ASC").Find(&meals)

	if len(meals) == 0 {
		return "📋 *Today's Summary*\n\nNo meals logged yet today.", nil
	}

	var totalCal, totalPro, totalCarb, totalFat float64
	var lines []string
	for _, m := range meals {
		lines = append(lines, fmt.Sprintf("- %s (%s): %.0f kcal, %.1fg protein", m.Name, m.MealType, m.Calories, m.Protein))
		totalCal += m.Calories
		totalPro += m.Protein
		totalCarb += m.Carbs
		totalFat += m.Fat
	}

	state, _ := services.ComputeRemainingDayState(s.User.ID, time.Now())
	remaining := ""
	if state != nil {
		remaining = fmt.Sprintf("\n\n*Remaining:* %.0f kcal, %.1fg protein", state.RemainingCalories, state.RemainingProtein)
	}

	return fmt.Sprintf("📋 *Today's Summary (%d meals):*\n\n%s\n\n*Totals:* %.0f kcal | %.1fg protein | %.1fg carbs | %.1fg fat%s",
		len(meals), strings.Join(lines, "\n"), totalCal, totalPro, totalCarb, totalFat, remaining), nil
}

// HandleModifyMeal handles corrections to previously logged meals
func HandleModifyMeal(s *Session, args map[string]interface{}) (string, error) {
	ns := services.NewNutritionService()
	mealType, _ := args["meal_type"].(string)
	action, _ := args["action"].(string)
	targetDish, _ := args["target_dish_name"].(string)
	newIngredients, _ := args["new_ingredients"].(string)

	todayStart := time.Now().Truncate(24 * time.Hour)

	switch action {
	case "delete":
		if targetDish == "" {
			return "Please specify which dish to remove.", nil
		}
		result := database.DB.Where("user_id = ? AND meal_type = ? AND LOWER(name) = ? AND logged_at >= ?",
			s.User.ID, mealType, strings.ToLower(targetDish), todayStart).Delete(&models.MealLog{})
		if result.RowsAffected == 0 {
			return fmt.Sprintf("I couldn't find '%s' in your %s to remove.", targetDish, mealType), nil
		}
		return fmt.Sprintf("🗑️ Removed *%s* from %s.", targetDish, mealType), nil

	case "update":
		if targetDish == "" || newIngredients == "" {
			return "I need both the dish name and the new description to update.", nil
		}
		var meal models.MealLog
		err := database.DB.Where("user_id = ? AND meal_type = ? AND LOWER(name) LIKE ? AND logged_at >= ?",
			s.User.ID, mealType, "%"+strings.ToLower(targetDish)+"%", todayStart).First(&meal).Error
		if err != nil {
			return fmt.Sprintf("I couldn't find '%s' in your %s to update.", targetDish, mealType), nil
		}
		estimated, err := ns.EstimateNutritionFromQuery(s.User.ID, newIngredients)
		if err != nil || estimated == nil {
			return "I had trouble re-estimating the nutrition for the updated item.", nil
		}
		database.DB.Model(&meal).Updates(map[string]interface{}{
			"ingredients": newIngredients,
			"calories":    estimated.Calories,
			"protein":     estimated.Protein,
			"carbs":       estimated.Carbs,
			"fat":         estimated.Fat,
		})
		return fmt.Sprintf("✏️ Updated *%s* → %.0f kcal, %.1fg protein.", targetDish, estimated.Calories, estimated.Protein), nil

	case "add":
		if newIngredients == "" {
			return "Please describe what you'd like to add.", nil
		}
		dishName := targetDish
		if dishName == "" {
			dishName = newIngredients
		}
		estimated, err := ns.EstimateNutritionFromQuery(s.User.ID, newIngredients)
		if err != nil || estimated == nil {
			return "I had trouble estimating the nutrition for that item.", nil
		}
		database.DB.Create(&models.MealLog{
			UserID:      s.User.ID,
			Name:        dishName,
			MealType:    mealType,
			Ingredients: newIngredients,
			Calories:    estimated.Calories,
			Protein:     estimated.Protein,
			Carbs:       estimated.Carbs,
			Fat:         estimated.Fat,
			LoggedAt:    time.Now(),
		})
		return fmt.Sprintf("➕ Added *%s* to %s (%.0f kcal).", dishName, mealType, estimated.Calories), nil
	}

	return "I didn't understand the modification action.", nil
}

// HandleGetPastDaySummary shows meals for a specific past date
func HandleGetPastDaySummary(s *Session, args map[string]interface{}) (string, error) {
	dateStr, _ := args["date"].(string)
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return "I couldn't understand that date. Please use YYYY-MM-DD format.", nil
	}

	nextDay := date.Add(24 * time.Hour)
	var meals []models.MealLog
	database.DB.Where("user_id = ? AND logged_at >= ? AND logged_at < ?", s.User.ID, date, nextDay).
		Order("logged_at ASC").Find(&meals)

	if len(meals) == 0 {
		return fmt.Sprintf("📋 No meals logged on %s.", dateStr), nil
	}

	var totalCal, totalPro float64
	var lines []string
	for _, m := range meals {
		lines = append(lines, fmt.Sprintf("- %s (%s): %.0f kcal", m.Name, m.MealType, m.Calories))
		totalCal += m.Calories
		totalPro += m.Protein
	}

	return fmt.Sprintf("📋 *Summary for %s (%d meals):*\n\n%s\n\n*Totals:* %.0f kcal, %.1fg protein.",
		dateStr, len(meals), strings.Join(lines, "\n"), totalCal, totalPro), nil
}

// HandleClearAllMealsToday deletes all meals logged today
func HandleClearAllMealsToday(s *Session, args map[string]interface{}) (string, error) {
	todayStart := time.Now().Truncate(24 * time.Hour)
	result := database.DB.Where("user_id = ? AND logged_at >= ?", s.User.ID, todayStart).Delete(&models.MealLog{})
	return fmt.Sprintf("🗑️ Cleared %d meals from today. Your daily log is now empty.", result.RowsAffected), nil
}

// HandleCreateRecipe saves a new recipe
func HandleCreateRecipe(s *Session, args map[string]interface{}) (string, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return "I need a name for the recipe.", nil
	}

	ingredientsJSON, _ := json.Marshal(args["ingredients"])
	instructionsJSON, _ := json.Marshal(args["instructions"])
	sourceURL, _ := args["source_url"].(string)

	recipe := models.Recipe{
		UserID:       s.User.ID,
		Name:         name,
		Ingredients:  string(ingredientsJSON),
		Instructions: string(instructionsJSON),
		SourceURL:    sourceURL,
	}

	// Estimate nutrition
	ns := services.NewNutritionService()
	var ingredientDesc []string
	if ingList, ok := args["ingredients"].([]interface{}); ok {
		for _, ing := range ingList {
			if ingMap, ok := ing.(map[string]interface{}); ok {
				ingName, _ := ingMap["name"].(string)
				qty, _ := ingMap["quantity"].(float64)
				unit, _ := ingMap["unit"].(string)
				ingredientDesc = append(ingredientDesc, fmt.Sprintf("%.0f %s %s", qty, unit, ingName))
			}
		}
	}
	if len(ingredientDesc) > 0 {
		estimated, err := ns.EstimateNutritionFromQuery(s.User.ID, strings.Join(ingredientDesc, ", "))
		if err == nil && estimated != nil {
			recipe.TotalCalories = estimated.Calories
			recipe.TotalProtein = estimated.Protein
			recipe.TotalFat = estimated.Fat
			recipe.TotalCarbs = estimated.Carbs
		}
	}

	database.DB.Create(&recipe)
	return fmt.Sprintf("📖 *Recipe Saved:* %s\nEstimated: %.0f kcal, %.1fg protein per serving.", name, recipe.TotalCalories, recipe.TotalProtein), nil
}

// HandleGetPantry lists all pantry items
func HandleGetPantry(s *Session, args map[string]interface{}) (string, error) {
	var items []models.PantryItem
	database.DB.Preload("Ingredient").Where("user_id = ?", s.User.ID).Find(&items)

	if len(items) == 0 {
		return "📦 Your pantry is empty. Send me a grocery list to get started.", nil
	}

	var lines []string
	for _, pi := range items {
		qty := pi.EffectiveQuantity()
		if qty <= 0 {
			continue
		}
		name := "Unknown"
		if pi.Ingredient.Name != "" {
			name = pi.Ingredient.Name
		}
		lines = append(lines, fmt.Sprintf("- %s: %.1f", name, qty))
	}

	if len(lines) == 0 {
		return "📦 Your pantry is empty. Send me a grocery list to get started.", nil
	}

	return fmt.Sprintf("📦 *Your Pantry (%d items):*\n\n%s", len(lines), strings.Join(lines, "\n")), nil
}

// HandleGetRecipes lists all saved recipes
func HandleGetRecipes(s *Session, args map[string]interface{}) (string, error) {
	var recipes []models.Recipe
	database.DB.Where("user_id = ?", s.User.ID).Find(&recipes)

	if len(recipes) == 0 {
		return "📖 No saved recipes yet. Share a recipe and I'll save it for you.", nil
	}

	var lines []string
	for _, r := range recipes {
		lines = append(lines, fmt.Sprintf("- *%s* (%.0f kcal)", r.Name, r.TotalCalories))
	}

	return fmt.Sprintf("📖 *Your Recipes (%d):*\n\n%s", len(recipes), strings.Join(lines, "\n")), nil
}

// HandleDeleteRecipe deletes a recipe by name
func HandleDeleteRecipe(s *Session, args map[string]interface{}) (string, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return "Please specify which recipe to delete.", nil
	}

	result := database.DB.Where("user_id = ? AND LOWER(name) = ?", s.User.ID, strings.ToLower(name)).Delete(&models.Recipe{})
	if result.RowsAffected == 0 {
		return fmt.Sprintf("I couldn't find a recipe called '%s'.", name), nil
	}

	return fmt.Sprintf("🗑️ Deleted recipe: *%s*.", name), nil
}

// Ensure llm import is used
var _ = llm.NewClient
