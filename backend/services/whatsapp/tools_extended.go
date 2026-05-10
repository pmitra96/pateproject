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
	"gorm.io/gorm"
)

func defaultUserLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return time.FixedZone("IST", 5*60*60+30*60)
	}
	return loc
}

func userLocationForDisplay(userID uint) *time.Location {
	var prefs models.UserPreferences
	if err := database.DB.Where("user_id = ?", userID).First(&prefs).Error; err == nil {
		if strings.TrimSpace(prefs.Timezone) != "" {
			if loc, err := time.LoadLocation(strings.TrimSpace(prefs.Timezone)); err == nil {
				return loc
			}
		}
	}
	return defaultUserLocation()
}

func nowForUser(userID uint) time.Time {
	return time.Now().In(userLocationForDisplay(userID))
}

func userReadableTime(userID uint, t time.Time) string {
	loc := userLocationForDisplay(userID)
	return t.In(loc).Format("Jan 02, 2006 03:04 PM MST")
}

// HandleGetDailySummary returns a summary of all meals logged today
func HandleGetDailySummary(s *Session, args map[string]interface{}) (string, error) {
	nowLocal := nowForUser(s.User.ID)
	todayStart, todayEnd := dayWindow(nowLocal)
	var meals []models.MealLog
	database.DB.Where("user_id = ? AND logged_at >= ? AND logged_at < ?", s.User.ID, todayStart, todayEnd).
		Order("logged_at ASC").Find(&meals)

	if len(meals) == 0 {
		return `{"ok":true,"meals":[],"totals":{"calories":0,"protein":0,"carbs":0,"fat":0,"fiber":0}}`, nil
	}

	var totalCal, totalPro, totalCarb, totalFat, totalFiber float64
	var lines []string
	sections := map[string][]string{
		"Breakfast": {},
		"Lunch":     {},
		"Snack":     {},
		"Dinner":    {},
	}
	sectionTotals := map[string]map[string]float64{
		"Breakfast": {"calories": 0, "protein": 0, "carbs": 0, "fat": 0, "fiber": 0},
		"Lunch":     {"calories": 0, "protein": 0, "carbs": 0, "fat": 0, "fiber": 0},
		"Snack":     {"calories": 0, "protein": 0, "carbs": 0, "fat": 0, "fiber": 0},
		"Dinner":    {"calories": 0, "protein": 0, "carbs": 0, "fat": 0, "fiber": 0},
	}
	for _, m := range meals {
		qtyLabel := formatMealQuantityLabel(m)
		line := fmt.Sprintf("- %s [%s] (%s): %.0f kcal, P %.1fg, C %.1fg, F %.1fg, Fi %.1fg", m.Name, qtyLabel, userReadableTime(s.User.ID, m.LoggedAt), m.Calories, m.Protein, m.Carbs, m.Fat, m.Fiber)
		lines = append(lines, line)
		section := normalizeMealTypeForSummary(m.MealType)
		sections[section] = append(sections[section], line)
		sectionTotals[section]["calories"] += m.Calories
		sectionTotals[section]["protein"] += m.Protein
		sectionTotals[section]["carbs"] += m.Carbs
		sectionTotals[section]["fat"] += m.Fat
		sectionTotals[section]["fiber"] += m.Fiber
		totalCal += m.Calories
		totalPro += m.Protein
		totalCarb += m.Carbs
		totalFat += m.Fat
		totalFiber += m.Fiber
	}

	state, _ := services.ComputeRemainingDayState(s.User.ID, nowForUser(s.User.ID))
	remaining := ""
	budgetFeedback := "Good progress. Keep balancing your remaining meals."
	if state != nil {
		remaining = fmt.Sprintf("\n\n*Remaining:* %.0f kcal, %.1fg protein, %.1fg carbs, %.1fg fat, %.1fg fiber", state.RemainingCalories, state.RemainingProtein, state.RemainingCarbs, state.RemainingFat, state.RemainingFiber)
		consumed := state.TargetCalories - state.RemainingCalories
		breakfastAndLunch := sectionTotals["Breakfast"]["calories"] + sectionTotals["Lunch"]["calories"]
		if state.RemainingCalories < 0 {
			budgetFeedback = "You have exceeded your daily budget. Tomorrow, try reducing calorie-dense items earlier in the day and keep dinner lighter."
		} else if state.TargetCalories > 0 && breakfastAndLunch >= (0.85*state.TargetCalories) {
			budgetFeedback = "You used most of your budget by breakfast/lunch. Keep dinner lighter with high-protein, low-fat options."
		} else if state.TargetCalories > 0 && consumed >= (0.7*state.TargetCalories) {
			budgetFeedback = "You are close to your budget already. Keep the remaining meals lighter and protein-focused."
		} else {
			budgetFeedback = "Good pacing so far. You still have room for later meals if portions stay controlled."
		}
	}

	return jsonString(map[string]any{
		"ok": true, "count": len(meals), "lines": lines,
		"sections": map[string]any{
			"breakfast": map[string]any{"meals": sections["Breakfast"], "totals": sectionTotals["Breakfast"]},
			"lunch":     map[string]any{"meals": sections["Lunch"], "totals": sectionTotals["Lunch"]},
			"snack":     map[string]any{"meals": sections["Snack"], "totals": sectionTotals["Snack"]},
			"dinner":    map[string]any{"meals": sections["Dinner"], "totals": sectionTotals["Dinner"]},
		},
		"totals":          map[string]any{"calories": totalCal, "protein": totalPro, "carbs": totalCarb, "fat": totalFat, "fiber": totalFiber},
		"remaining_text":  strings.TrimSpace(remaining),
		"budget_feedback": budgetFeedback,
	}), nil
}

func formatMealQuantityLabel(m models.MealLog) string {
	if m.QuantityValue > 0 && strings.TrimSpace(m.QuantityUnit) != "" {
		if m.QuantityValue == float64(int64(m.QuantityValue)) {
			return fmt.Sprintf("%d %s", int64(m.QuantityValue), strings.TrimSpace(m.QuantityUnit))
		}
		return fmt.Sprintf("%.2f %s", m.QuantityValue, strings.TrimSpace(m.QuantityUnit))
	}
	if strings.TrimSpace(m.ServingSize) != "" {
		return strings.TrimSpace(m.ServingSize)
	}
	return "1 serving"
}

func normalizeMealTypeForSummary(mealType string) string {
	switch strings.ToLower(strings.TrimSpace(mealType)) {
	case "breakfast":
		return "Breakfast"
	case "lunch":
		return "Lunch"
	case "snack":
		return "Snack"
	case "dinner":
		return "Dinner"
	default:
		return "Snack"
	}
}

// HandleModifyMeal handles corrections to previously logged meals
func HandleModifyMeal(s *Session, args map[string]interface{}) (string, error) {
	ns := services.NewNutritionService()
	mealType, _ := args["meal_type"].(string)
	action, _ := args["action"].(string)
	targetDish, _ := args["target_dish_name"].(string)
	newIngredients, _ := args["new_ingredients"].(string)

	nowLocal := nowForUser(s.User.ID)
	todayStart, todayEnd := dayWindow(nowLocal)
	candidates, err := findMealsForDay(s.User.ID, mealType, todayStart, todayEnd)
	if err != nil {
		return `{"ok":false,"error":"meal_lookup_failed"}`, nil
	}

	switch action {
	case "delete":
		return deleteMealsTransactional(s, candidates, mealType, targetDish, args)

	case "update":
		return updateMealTransactional(s, ns, candidates, mealType, targetDish, newIngredients, args)

	case "add":
		if newIngredients == "" {
			return `{"ok":false,"error":"new_ingredients_required"}`, nil
		}
		dishName := targetDish
		dishName = canonicalDishName(dishName, mealType, newIngredients)
		estimated, err := ns.EstimateNutritionFromQuery(s.User.ID, newIngredients)
		if err != nil || estimated == nil {
			return `{"ok":false,"error":"nutrition_estimation_failed"}`, nil
		}
		newMeal := models.MealLog{
			UserID:      s.User.ID,
			Name:        dishName,
			MealType:    mealType,
			Ingredients: newIngredients,
			Calories:    estimated.Calories,
			Protein:     estimated.Protein,
			Carbs:       estimated.Carbs,
			Fat:         estimated.Fat,
			Fiber:       estimated.Fiber,
			ServingSize: estimated.ServingSize,
			LoggedAt:    nowForUser(s.User.ID),
		}
		qty := services.ParseMealQuantity(estimated.ServingSize)
		newMeal.QuantityValue = qty.Value
		newMeal.QuantityUnit = qty.Unit
		newMeal.QuantityBaseValue = qty.BaseValue
		newMeal.QuantityBaseUnit = qty.BaseUnit
		database.DB.Create(&newMeal)
		syncMealComponents(s.User.ID, newMeal.ID, newIngredients, estimated)
		return jsonString(map[string]any{"ok": true, "action": "add", "meal_id": newMeal.ID, "dish_name": dishName, "meal_type": mealType, "calories": estimated.Calories, "protein": estimated.Protein, "carbs": estimated.Carbs, "fat": estimated.Fat, "fiber": estimated.Fiber, "serving_size": estimated.ServingSize}), nil
	}

	return `{"ok":false,"error":"unknown_action"}`, nil
}

// HandleGetPastDaySummary shows meals for a specific past date
func HandleGetPastDaySummary(s *Session, args map[string]interface{}) (string, error) {
	dateStr, _ := args["date"].(string)
	loc := userLocationForDisplay(s.User.ID)
	date, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return `{"ok":false,"error":"invalid_date_format"}`, nil
	}

	dayStart, nextDay := dayWindow(date)
	var meals []models.MealLog
	database.DB.Where("user_id = ? AND logged_at >= ? AND logged_at < ?", s.User.ID, dayStart, nextDay).
		Order("logged_at ASC").Find(&meals)

	if len(meals) == 0 {
		return jsonString(map[string]any{
			"ok": true, "date": dayStart.Format("2006-01-02"), "meals": []string{},
			"totals": map[string]any{"calories": 0, "protein": 0, "carbs": 0, "fat": 0, "fiber": 0},
		}), nil
	}

	var totalCal, totalPro, totalCarb, totalFat, totalFiber float64
	var lines []string
	for _, m := range meals {
		lines = append(lines, fmt.Sprintf("- %s (%s, %s): %.0f kcal, P %.1fg, C %.1fg, F %.1fg, Fi %.1fg", m.Name, m.MealType, userReadableTime(s.User.ID, m.LoggedAt), m.Calories, m.Protein, m.Carbs, m.Fat, m.Fiber))
		totalCal += m.Calories
		totalPro += m.Protein
		totalCarb += m.Carbs
		totalFat += m.Fat
		totalFiber += m.Fiber
	}

	return jsonString(map[string]any{
		"ok": true, "date": dateStr, "count": len(meals), "lines": lines,
		"totals": map[string]any{"calories": totalCal, "protein": totalPro, "carbs": totalCarb, "fat": totalFat, "fiber": totalFiber},
	}), nil
}

// HandleClearAllMealsToday deletes all meals logged today
func HandleClearAllMealsToday(s *Session, args map[string]interface{}) (string, error) {
	nowLocal := nowForUser(s.User.ID)
	todayStart, todayEnd := dayWindow(nowLocal)
	result := database.DB.Where("user_id = ? AND logged_at >= ? AND logged_at < ?", s.User.ID, todayStart, todayEnd).Delete(&models.MealLog{})
	return jsonString(map[string]any{"ok": true, "deleted_count": result.RowsAffected}), nil
}

// HandleCreateRecipe saves a new recipe
func HandleCreateRecipe(s *Session, args map[string]interface{}) (string, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return `{"ok":false,"error":"recipe_name_required"}`, nil
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
	return jsonString(map[string]any{"ok": true, "recipe_id": recipe.ID, "name": name, "estimated": map[string]any{"calories": recipe.TotalCalories, "protein": recipe.TotalProtein, "fat": recipe.TotalFat, "carbs": recipe.TotalCarbs}}), nil
}

// HandleGetPantry lists all pantry items
func HandleGetPantry(s *Session, args map[string]interface{}) (string, error) {
	var items []models.PantryItem
	database.DB.Preload("Ingredient").Where("user_id = ?", s.User.ID).Find(&items)

	if len(items) == 0 {
		return `{"ok":true,"items":[]}`, nil
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
		return `{"ok":true,"items":[]}`, nil
	}

	return jsonString(map[string]any{"ok": true, "count": len(lines), "items": lines}), nil
}

// HandleGetRecipes lists all saved recipes
func HandleGetRecipes(s *Session, args map[string]interface{}) (string, error) {
	var recipes []models.Recipe
	database.DB.Where("user_id = ?", s.User.ID).Find(&recipes)

	if len(recipes) == 0 {
		return `{"ok":true,"recipes":[]}`, nil
	}

	var lines []string
	for _, r := range recipes {
		lines = append(lines, fmt.Sprintf("- *%s* (%.0f kcal)", r.Name, r.TotalCalories))
	}

	return jsonString(map[string]any{"ok": true, "count": len(recipes), "recipes": lines}), nil
}

// HandleDeleteRecipe deletes a recipe by name
func HandleDeleteRecipe(s *Session, args map[string]interface{}) (string, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return `{"ok":false,"error":"recipe_name_required"}`, nil
	}

	result := database.DB.Where("user_id = ? AND LOWER(name) = ?", s.User.ID, strings.ToLower(name)).Delete(&models.Recipe{})
	if result.RowsAffected == 0 {
		return jsonString(map[string]any{"ok": false, "error": "recipe_not_found", "name": name}), nil
	}

	return jsonString(map[string]any{"ok": true, "deleted_recipe": name}), nil
}

func deleteMealsTransactional(s *Session, candidates []models.MealLog, mealType, targetDish string, args map[string]interface{}) (string, error) {
	// Bulk delete all meals for a meal type when confirmation flow sets target as "*".
	if strings.TrimSpace(targetDish) == "*" {
		var ids []uint
		for _, c := range candidates {
			ids = append(ids, c.ID)
		}
		if len(ids) == 0 {
			return jsonString(map[string]any{"ok": false, "error": "meal_not_found", "meal_type": mealType}), nil
		}
		err := database.DB.Transaction(func(tx *gorm.DB) error {
			return tx.Where("user_id = ? AND id IN ?", s.User.ID, ids).Delete(&models.MealLog{}).Error
		})
		if err != nil {
			return jsonString(map[string]any{"ok": false, "error": "delete_transaction_failed"}), nil
		}
		return jsonString(map[string]any{"ok": true, "action": "delete", "scope": "meal_type_all", "meal_type": mealType, "deleted_count": len(ids), "meal_ids": ids}), nil
	}

	if idNum, ok := args["meal_id"].(float64); ok && idNum > 0 {
		var meal models.MealLog
		if err := database.DB.Where("user_id = ? AND id = ?", s.User.ID, uint(idNum)).First(&meal).Error; err == nil {
			err = database.DB.Transaction(func(tx *gorm.DB) error {
				return tx.Delete(&models.MealLog{}, meal.ID).Error
			})
			if err != nil {
				return jsonString(map[string]any{"ok": false, "error": "delete_transaction_failed"}), nil
			}
			return jsonString(map[string]any{"ok": true, "action": "delete", "meal_id": meal.ID, "meal_type": meal.MealType, "dish_name": meal.Name}), nil
		}
	}
	meal, reason, ok := selectMealForCorrection(candidates, targetDish)
	if !ok {
		if reason == "ambiguous_target" {
			ids := make([]uint, 0, len(candidates))
			for _, c := range candidates {
				ids = append(ids, c.ID)
			}
			setPendingMealSelection(getConversationState(s.User.ID), "delete", ids, nil)
			return jsonString(ambiguousMealsPayload(candidates, reason)), nil
		}
		return jsonString(map[string]any{"ok": false, "error": reason, "meal_type": mealType, "dish_name": targetDish}), nil
	}
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		return tx.Delete(&models.MealLog{}, meal.ID).Error
	})
	if err != nil {
		return jsonString(map[string]any{"ok": false, "error": "delete_transaction_failed"}), nil
	}
	return jsonString(map[string]any{"ok": true, "action": "delete", "meal_id": meal.ID, "meal_type": meal.MealType, "dish_name": meal.Name}), nil
}

func updateMealTransactional(s *Session, ns *services.NutritionService, candidates []models.MealLog, mealType, targetDish, newIngredients string, args map[string]interface{}) (string, error) {
	var meal models.MealLog
	var estimatedServingSize string
	if idNum, ok := args["meal_id"].(float64); ok && idNum > 0 {
		if err := database.DB.Where("user_id = ? AND id = ?", s.User.ID, uint(idNum)).First(&meal).Error; err != nil {
			return jsonString(map[string]any{"ok": false, "error": "meal_not_found"}), nil
		}
	} else {
		selected, reason, ok := selectMealForCorrection(candidates, targetDish)
		if !ok {
			if reason == "ambiguous_target" {
				ids := make([]uint, 0, len(candidates))
				for _, c := range candidates {
					ids = append(ids, c.ID)
				}
				meta := map[string]any{"pending_update_ingredients": newIngredients}
				setPendingMealSelection(getConversationState(s.User.ID), "update", ids, meta)
				return jsonString(ambiguousMealsPayload(candidates, reason)), nil
			}
			return jsonString(map[string]any{"ok": false, "error": reason, "meal_type": mealType, "dish_name": targetDish}), nil
		}
		meal = selected
	}

	updates := map[string]interface{}{}
	updatedFields := []string{}

	if rename, ok := args["new_dish_name"].(string); ok && strings.TrimSpace(rename) != "" {
		updates["name"] = strings.TrimSpace(rename)
		updatedFields = append(updatedFields, "name")
	}
	if mt, ok := args["new_meal_type"].(string); ok && strings.TrimSpace(mt) != "" {
		updates["meal_type"] = strings.TrimSpace(mt)
		updatedFields = append(updatedFields, "meal_type")
	}
	manualMacro := false
	for _, key := range []string{"calories", "protein", "carbs", "fat", "fiber"} {
		if v, ok := args[key].(float64); ok {
			updates[key] = v
			updatedFields = append(updatedFields, key)
			manualMacro = true
		}
	}

	if strings.TrimSpace(newIngredients) != "" {
		updates["ingredients"] = newIngredients
		updatedFields = append(updatedFields, "ingredients")
		if !manualMacro {
			estimated, estErr := ns.EstimateNutritionFromQuery(s.User.ID, newIngredients)
			if estErr != nil || estimated == nil {
				return `{"ok":false,"error":"nutrition_estimation_failed"}`, nil
			}
			updates["calories"] = estimated.Calories
			updates["protein"] = estimated.Protein
			updates["carbs"] = estimated.Carbs
			updates["fat"] = estimated.Fat
			updates["fiber"] = estimated.Fiber
			estimatedServingSize = estimated.ServingSize
			updates["serving_size"] = estimatedServingSize
			qty := services.ParseMealQuantity(estimatedServingSize)
			updates["quantity_value"] = qty.Value
			updates["quantity_unit"] = qty.Unit
			updates["quantity_base_value"] = qty.BaseValue
			updates["quantity_base_unit"] = qty.BaseUnit
			updatedFields = append(updatedFields, "quantity_value", "quantity_unit", "quantity_base_value", "quantity_base_unit")
		}
	}

	if len(updates) == 0 {
		return `{"ok":false,"error":"target_dish_and_ingredients_required"}`, nil
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&meal).Updates(updates).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return jsonString(map[string]any{"ok": false, "error": "update_transaction_failed"}), nil
	}

	if strings.TrimSpace(newIngredients) != "" {
		estimated, _ := ns.EstimateNutritionFromQuery(s.User.ID, newIngredients)
		if estimated != nil && !manualMacro {
			syncMealComponents(s.User.ID, meal.ID, newIngredients, estimated)
		}
	}
	var refreshed models.MealLog
	_ = database.DB.Where("id = ?", meal.ID).First(&refreshed).Error
	return jsonString(map[string]any{
		"ok":             true,
		"action":         "update",
		"meal_id":        refreshed.ID,
		"dish_name":      refreshed.Name,
		"meal_type":      refreshed.MealType,
		"calories":       refreshed.Calories,
		"protein":        refreshed.Protein,
		"carbs":          refreshed.Carbs,
		"fat":            refreshed.Fat,
		"fiber":          refreshed.Fiber,
		"serving_size":   refreshed.ServingSize,
		"quantity_value": refreshed.QuantityValue,
		"quantity_unit":  refreshed.QuantityUnit,
		"updated_fields": updatedFields,
	}), nil
}

// Ensure llm import is used
var _ = llm.NewClient

func HandleGetMealLogTime(s *Session, args map[string]interface{}) (string, error) {
	mealName, _ := args["meal_name"].(string)
	dateStr, _ := args["date"].(string)
	if strings.TrimSpace(mealName) == "" {
		return `{"found":false,"reason":"meal_name is required"}`, nil
	}

	var dayStart time.Time
	if dateStr != "" {
		loc := userLocationForDisplay(s.User.ID)
		parsed, err := time.ParseInLocation("2006-01-02", dateStr, loc)
		if err != nil {
			return `{"found":false,"reason":"invalid date format, use YYYY-MM-DD"}`, nil
		}
		dayStart = parsed
	} else {
		nowLocal := nowForUser(s.User.ID)
		dayStart, _ = dayWindow(nowLocal)
		dateStr = dayStart.Format("2006-01-02")
	}
	dayEnd := dayStart.Add(24 * time.Hour)

	var meal models.MealLog
	err := database.DB.
		Where("user_id = ? AND logged_at >= ? AND logged_at < ? AND LOWER(name) LIKE ?",
			s.User.ID, dayStart, dayEnd, "%"+strings.ToLower(strings.TrimSpace(mealName))+"%").
		Order("logged_at DESC").
		First(&meal).Error
	if err != nil {
		return fmt.Sprintf(`{"found":false,"meal_name":"%s","date":"%s"}`, mealName, dateStr), nil
	}

	return fmt.Sprintf(
		`{"found":true,"meal":{"id":%d,"name":%q,"meal_type":%q,"logged_at":%q,"calories":%.0f,"protein":%.1f,"carbs":%.1f,"fat":%.1f,"fiber":%.1f}}`,
		meal.ID, meal.Name, meal.MealType, userReadableTime(s.User.ID, meal.LoggedAt), meal.Calories, meal.Protein, meal.Carbs, meal.Fat, meal.Fiber,
	), nil
}

func HandleGetRecentMeals(s *Session, args map[string]interface{}) (string, error) {
	limit := 5
	if v, ok := args["limit"].(float64); ok && int(v) > 0 && int(v) <= 20 {
		limit = int(v)
	}

	var meals []models.MealLog
	database.DB.Where("user_id = ?", s.User.ID).Order("logged_at DESC").Limit(limit).Find(&meals)
	if len(meals) == 0 {
		return `{"meals":[]}`, nil
	}

	type mealJSON struct {
		ID       uint    `json:"id"`
		Name     string  `json:"name"`
		MealType string  `json:"meal_type"`
		LoggedAt string  `json:"logged_at"`
		Calories float64 `json:"calories"`
		Protein  float64 `json:"protein"`
		Carbs    float64 `json:"carbs"`
		Fat      float64 `json:"fat"`
		Fiber    float64 `json:"fiber"`
	}
	out := struct {
		Meals []mealJSON `json:"meals"`
	}{Meals: make([]mealJSON, 0, len(meals))}
	for _, m := range meals {
		out.Meals = append(out.Meals, mealJSON{
			ID:       m.ID,
			Name:     m.Name,
			MealType: m.MealType,
			LoggedAt: userReadableTime(s.User.ID, m.LoggedAt),
			Calories: m.Calories,
			Protein:  m.Protein,
			Carbs:    m.Carbs,
			Fat:      m.Fat,
			Fiber:    m.Fiber,
		})
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

func HandleGetActiveGoal(s *Session, args map[string]interface{}) (string, error) {
	var goal models.Goal
	if err := database.DB.Where("user_id = ? AND is_active = ?", s.User.ID, true).First(&goal).Error; err != nil {
		return `{"has_active_goal":false}`, nil
	}

	var profile models.GoalMacroProfile
	_ = database.DB.Where("goal_id = ?", goal.ID).First(&profile).Error
	return fmt.Sprintf(
		`{"has_active_goal":true,"goal":{"id":%d,"title":%q,"daily_calorie_target":%d,"daily_protein_target":%.1f,"daily_fat_target":%.1f,"daily_carbs_target":%.1f,"daily_fiber_target":%.1f}}`,
		goal.ID, goal.Title, profile.DailyCalorieTarget, profile.DailyProteinTarget, profile.DailyFatTarget, profile.DailyCarbsTarget, profile.DailyFiberTarget,
	), nil
}

func HandleGetUserProfile(s *Session, args map[string]interface{}) (string, error) {
	var prefs models.UserPreferences
	if err := database.DB.Where("user_id = ?", s.User.ID).First(&prefs).Error; err != nil {
		return `{"has_profile":false}`, nil
	}
	return fmt.Sprintf(
		`{"has_profile":true,"profile":{"height":%.1f,"weight":%.1f,"age":%d,"gender":%q,"country":%q,"state":%q,"city":%q}}`,
		prefs.Height, prefs.Weight, prefs.Age, prefs.Gender, prefs.Country, prefs.State, prefs.City,
	), nil
}

func HandleGetRecentOrders(s *Session, args map[string]interface{}) (string, error) {
	limit := 5
	if v, ok := args["limit"].(float64); ok && int(v) > 0 && int(v) <= 20 {
		limit = int(v)
	}

	var orders []models.Order
	database.DB.Preload("OrderItems").Where("user_id = ?", s.User.ID).Order("order_date DESC").Limit(limit).Find(&orders)
	if len(orders) == 0 {
		return `{"orders":[]}`, nil
	}

	type orderJSON struct {
		ID         uint   `json:"id"`
		Provider   string `json:"provider"`
		OrderDate  string `json:"order_date"`
		ExternalID string `json:"external_order_id"`
		ItemsCount int    `json:"items_count"`
		Status     string `json:"status"`
	}
	out := struct {
		Orders []orderJSON `json:"orders"`
	}{Orders: make([]orderJSON, 0, len(orders))}
	for _, o := range orders {
		out.Orders = append(out.Orders, orderJSON{
			ID:         o.ID,
			Provider:   o.Provider,
			OrderDate:  o.OrderDate.Format(time.RFC3339),
			ExternalID: o.ExternalOrderID,
			ItemsCount: len(o.OrderItems),
			Status:     o.Status,
		})
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}
