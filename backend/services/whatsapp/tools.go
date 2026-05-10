package whatsapp

import (
	"fmt"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/llm"
	"github.com/pmitra96/pateproject/models"
	"github.com/pmitra96/pateproject/services"
	"gorm.io/gorm"
)

// --- Tool Handlers ---

func HandleLogMeals(s *Session, args map[string]interface{}) (string, error) {
	ns := services.NewNutritionService()
	meals, ok := args["meals"].([]interface{})
	if !ok || len(meals) == 0 {
		return jsonString(map[string]any{"ok": false, "error": "no_meals_detected"}), nil
	}

	type pendingMealLog struct {
		DishName     string
		Ingredients  string
		MealType     string
		Estimated    *services.FoodEstimate
		ControlMode  string
		LoggedAt     time.Time
		QuantityInfo services.MealQuantity
	}
	pending := make([]pendingMealLog, 0, len(meals))

	now := nowForUser(s.User.ID)
	preState, _ := services.ComputeRemainingDayState(s.User.ID, now)
	controlMode := "NORMAL"
	if preState != nil {
		controlMode = preState.ControlMode
	}

	for _, m := range meals {
		mealData, _ := m.(map[string]interface{})
		dishName, _ := mealData["dish_name"].(string)
		ingredients, _ := mealData["ingredients"].(string)
		mealType, _ := mealData["meal_type"].(string)
		dishName = canonicalDishName(dishName, mealType, ingredients)

		estimated, err := ns.EstimateNutritionFromQuery(s.User.ID, ingredients)
		if err != nil || estimated == nil {
			return jsonString(map[string]any{"ok": false, "error": ErrCodeNutritionEstimateFailed, "dish_name": dishName}), nil
		}
		qty := services.ParseMealQuantity(estimated.ServingSize)
		pending = append(pending, pendingMealLog{
			DishName:     dishName,
			Ingredients:  ingredients,
			MealType:     mealType,
			Estimated:    estimated,
			ControlMode:  controlMode,
			LoggedAt:     now,
			QuantityInfo: qty,
		})
	}

	var entries []map[string]any
	createdIDs := make([]uint, 0, len(pending))
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		for _, pm := range pending {
			estimated := pm.Estimated
			if estimated == nil {
				return fmt.Errorf(ErrCodeNutritionEstimateFailed)
			}
			mealLog := models.MealLog{
				UserID:           s.User.ID,
				Name:             pm.DishName,
				MealType:         pm.MealType,
				Ingredients:      pm.Ingredients,
				Calories:         estimated.Calories,
				Protein:          estimated.Protein,
				Carbs:            estimated.Carbs,
				Fat:              estimated.Fat,
				Fiber:            estimated.Fiber,
				ServingSize:      estimated.ServingSize,
				LoggedAt:         pm.LoggedAt,
				ControlModeAtLog: pm.ControlMode,
			}
			if err := tx.Create(&mealLog).Error; err != nil {
				return err
			}
			mealLog.QuantityValue = pm.QuantityInfo.Value
			mealLog.QuantityUnit = pm.QuantityInfo.Unit
			mealLog.QuantityBaseValue = pm.QuantityInfo.BaseValue
			mealLog.QuantityBaseUnit = pm.QuantityInfo.BaseUnit
			if err := tx.Model(&mealLog).Updates(map[string]any{
				"quantity_value":      mealLog.QuantityValue,
				"quantity_unit":       mealLog.QuantityUnit,
				"quantity_base_value": mealLog.QuantityBaseValue,
				"quantity_base_unit":  mealLog.QuantityBaseUnit,
			}).Error; err != nil {
				return err
			}
			var check models.MealLog
			if err := tx.Where("id = ? AND user_id = ?", mealLog.ID, s.User.ID).First(&check).Error; err != nil {
				return err
			}
			syncMealComponents(s.User.ID, mealLog.ID, pm.Ingredients, estimated)
			createdIDs = append(createdIDs, mealLog.ID)
			entries = append(entries, map[string]any{
				"meal_id": mealLog.ID, "dish_name": pm.DishName, "meal_type": pm.MealType, "ok": true,
				"calories": estimated.Calories, "protein": estimated.Protein, "carbs": estimated.Carbs, "fat": estimated.Fat, "fiber": estimated.Fiber,
				"serving_size": estimated.ServingSize,
				"logged_at":    mealLog.LoggedAt.Format(time.RFC3339),
				"display_time": userReadableTime(s.User.ID, mealLog.LoggedAt),
			})
		}
		return nil
	}); err != nil {
		return jsonString(map[string]any{"ok": false, "error": ErrCodeWriteFailed}), nil
	}
	if len(createdIDs) != len(pending) {
		return jsonString(map[string]any{"ok": false, "error": ErrCodeReadbackFailed}), nil
	}

	if len(entries) == 0 {
		return jsonString(map[string]any{"ok": false, "error": ErrCodeNoMealsLogged}), nil
	}

	newState, _ := services.ComputeRemainingDayState(s.User.ID, nowForUser(s.User.ID))
	modeStr := ""
	if newState != nil && newState.ControlMode == "DAMAGE_CONTROL" {
		modeStr = "\n⚠️ WARNING: You are now in DAMAGE CONTROL mode!"
	}

	res := map[string]any{"ok": true, "logged_meals": entries}
	if newState != nil {
		res["remaining"] = map[string]any{
			"calories": newState.RemainingCalories, "protein": newState.RemainingProtein, "carbs": newState.RemainingCarbs, "fat": newState.RemainingFat, "fiber": newState.RemainingFiber, "mode": newState.ControlMode,
		}
	}
	if modeStr != "" {
		res["warning"] = "damage_control_mode"
	}
	return jsonString(res), nil
}

func HandleSetGoal(s *Session, args map[string]interface{}) (string, error) {
	calories := int(args["calories"].(float64))
	if calories <= 0 {
		return jsonString(map[string]any{"ok": false, "error": "invalid_calories"}), nil
	}

	var goal models.Goal
	if err := database.DB.Where("user_id = ? AND is_active = ?", s.User.ID, true).First(&goal).Error; err != nil {
		goal = models.Goal{UserID: s.User.ID, Title: "My Health Goal", IsActive: true}
		database.DB.Create(&goal)
	}

	var profile models.GoalMacroProfile
	if err := database.DB.Where("goal_id = ?", goal.ID).First(&profile).Error; err != nil {
		profile = models.GoalMacroProfile{
			GoalID:             goal.ID,
			DailyCalorieTarget: calories,
			DailyProteinTarget: float64(calories) * 0.30 / 4,
			DailyFatTarget:     float64(calories) * 0.30 / 9,
			DailyCarbsTarget:   float64(calories) * 0.40 / 4,
			DailyFiberTarget:   services.DefaultFiberTargetFromCalories(calories),
		}
		database.DB.Create(&profile)
	} else {
		database.DB.Model(&profile).Updates(map[string]interface{}{
			"daily_calorie_target": calories,
			"daily_fiber_target":   services.DefaultFiberTargetFromCalories(calories),
			"updated_at":           time.Now(),
		})
	}
	return jsonString(map[string]any{"ok": true, "daily_calorie_target": calories}), nil
}

func HandleAskAdvice(s *Session, args map[string]interface{}) (string, error) {
	ns := services.NewNutritionService()
	foodName, _ := args["food_description"].(string)

	state, _ := services.ComputeRemainingDayState(s.User.ID, nowForUser(s.User.ID))
	if state == nil {
		return jsonString(map[string]any{"ok": false, "error": "goal_not_found"}), nil
	}

	estimated, err := ns.EstimateNutritionFromQuery(s.User.ID, foodName)
	if err != nil || estimated == nil {
		return jsonString(map[string]any{"ok": false, "error": "nutrition_estimation_failed"}), nil
	}

	result := services.CheckFoodPermission(state, *estimated)
	decision := "❌ No, you shouldn't eat this."
	if result.Status == services.StatusAllow || result.Status == services.StatusAllowWithConstraint {
		decision = "✅ Yes, you can eat this!"
	}

	return jsonString(map[string]any{
		"ok": true, "decision": decision, "reason": result.Reason,
		"estimated": map[string]any{"calories": estimated.Calories, "protein": estimated.Protein, "carbs": estimated.Carbs, "fat": estimated.Fat, "fiber": estimated.Fiber, "serving_size": estimated.ServingSize},
		"status":    result.Status,
	}), nil
}

func HandleGetFoodNutrition(s *Session, args map[string]interface{}) (string, error) {
	ns := services.NewNutritionService()
	foodName, _ := args["food_description"].(string)
	foodName = strings.TrimSpace(foodName)
	if foodName == "" {
		return jsonString(map[string]any{"ok": false, "error": "food_description_required"}), nil
	}

	estimated, err := ns.EstimateNutritionFromQuery(s.User.ID, foodName)
	if err != nil || estimated == nil {
		return jsonString(map[string]any{"ok": false, "error": "nutrition_estimation_failed"}), nil
	}

	return jsonString(map[string]any{
		"ok":               true,
		"food_description": foodName,
		"estimated": map[string]any{
			"calories":     estimated.Calories,
			"protein":      estimated.Protein,
			"carbs":        estimated.Carbs,
			"fat":          estimated.Fat,
			"fiber":        estimated.Fiber,
			"serving_size": estimated.ServingSize,
		},
	}), nil
}

func HandleGetBudget(s *Session, args map[string]interface{}) (string, error) {
	state, _ := services.ComputeRemainingDayState(s.User.ID, nowForUser(s.User.ID))
	if state == nil {
		return jsonString(map[string]any{"ok": false, "error": "goal_not_found"}), nil
	}
	return jsonString(map[string]any{
		"ok": true,
		"remaining": map[string]any{
			"calories": state.RemainingCalories, "protein": state.RemainingProtein,
			"carbs": state.RemainingCarbs, "fat": state.RemainingFat, "fiber": state.RemainingFiber, "mode": state.ControlMode,
		},
	}), nil
}

func HandleUpdateProfile(s *Session, args map[string]interface{}) (string, error) {
	var prefs models.UserPreferences
	if err := database.DB.Where("user_id = ?", s.User.ID).First(&prefs).Error; err != nil {
		prefs = models.UserPreferences{UserID: s.User.ID}
		database.DB.Create(&prefs)
	}

	updates := make(map[string]interface{})
	if h, ok := args["height"].(float64); ok {
		updates["height"] = h
	}
	if w, ok := args["weight"].(float64); ok {
		updates["weight"] = w
	}
	if a, ok := args["age"].(float64); ok {
		updates["age"] = int(a)
	}
	if g, ok := args["gender"].(string); ok {
		updates["gender"] = g
	}
	if al, ok := args["activity_level"].(string); ok && strings.TrimSpace(al) != "" {
		updates["activity_level"] = strings.ToLower(strings.TrimSpace(al))
	}
	if tz, ok := args["timezone"].(string); ok && strings.TrimSpace(tz) != "" {
		updates["timezone"] = strings.TrimSpace(tz)
	}

	if len(updates) == 0 {
		return jsonString(map[string]any{"ok": false, "error": "no_profile_updates"}), nil
	}

	database.DB.Model(&models.UserPreferences{}).Where("user_id = ?", s.User.ID).Updates(updates)
	return jsonString(map[string]any{"ok": true, "updated_fields": updates}), nil
}

func HandleUpdatePantry(s *Session, args map[string]interface{}) (string, error) {
	llmClient := llm.NewClient()
	items, ok := args["items"].([]interface{})
	if !ok {
		return jsonString(map[string]any{"ok": false, "error": "invalid_items_payload"}), nil
	}

	var updatedNames []string
	ingredientChanges := make(map[uint]*pantryChange)
	ingredientNames := make(map[uint]string)

	var rawNames []string
	var results = make([]llm.PantryItemExtraction, len(items))
	var itemsToExtract []int

	for i, it := range items {
		itemMap, _ := it.(map[string]interface{})
		name, _ := itemMap["name"].(string)
		rawNames = append(rawNames, name)

		var mapping models.IngredientMapping
		if err := database.DB.Where("LOWER(raw_name) = ?", strings.ToLower(name)).First(&mapping).Error; err == nil {
			results[i] = llm.PantryItemExtraction{
				Ingredient: mapping.IngredientName,
				Brand:      mapping.Brand,
				Product:    mapping.Product,
			}
		} else {
			itemsToExtract = append(itemsToExtract, i)
		}
	}

	if len(itemsToExtract) > 0 {
		var namesToCall []string
		for _, idx := range itemsToExtract {
			namesToCall = append(namesToCall, rawNames[idx])
		}

		extractions, err := llmClient.ExtractPantryItemsBatch(namesToCall)
		if err == nil {
			for i, ext := range extractions {
				originalIdx := itemsToExtract[i]
				results[originalIdx] = ext
				if ext.Ingredient != "" {
					var ing models.Ingredient
					database.DB.Where("LOWER(name) = ?", strings.ToLower(ext.Ingredient)).FirstOrCreate(&ing, models.Ingredient{Name: ext.Ingredient})
					database.DB.Create(&models.IngredientMapping{
						RawName:        rawNames[originalIdx],
						IngredientID:   ing.ID,
						IngredientName: ing.Name,
						Brand:          ext.Brand,
						Product:        ext.Product,
					})
				}
			}
		}
	}

	for i, it := range items {
		itemMap, _ := it.(map[string]interface{})
		quantity, _ := itemMap["quantity"].(float64)
		unit, _ := itemMap["unit"].(string)
		action, _ := itemMap["action"].(string)

		ingredientName := results[i].Ingredient
		if ingredientName == "" {
			// Simpler normalization since this is a service now
			ingredientName = strings.TrimSpace(strings.ToLower(rawNames[i]))
		}

		var ingredient models.Ingredient
		database.DB.Where("LOWER(name) = ?", strings.ToLower(ingredientName)).FirstOrCreate(&ingredient, models.Ingredient{Name: ingredientName})

		if _, exists := ingredientChanges[ingredient.ID]; !exists {
			ingredientChanges[ingredient.ID] = &pantryChange{
				quantity: quantity,
				unit:     unit,
				action:   action,
				brand:    results[i].Brand,
				rawName:  rawNames[i],
			}
			ingredientNames[ingredient.ID] = ingredientName
		} else {
			if action == "add" {
				ingredientChanges[ingredient.ID].quantity += quantity
			} else {
				ingredientChanges[ingredient.ID].quantity = quantity
				ingredientChanges[ingredient.ID].action = action
			}
		}
	}

	for ingID, change := range ingredientChanges {
		var pi models.PantryItem
		err := database.DB.Where("user_id = ? AND ingredient_id = ?", s.User.ID, ingID).First(&pi).Error

		if err != nil && err == gorm.ErrRecordNotFound {
			var sampleItem models.Item
			database.DB.Where("ingredient_id = ?", ingID).First(&sampleItem)
			if sampleItem.ID == 0 {
				sampleItem = models.Item{Name: change.rawName, IngredientID: ingID, Unit: change.unit}
				database.DB.Create(&sampleItem)
			}
			pi = models.PantryItem{UserID: s.User.ID, IngredientID: ingID, ItemID: sampleItem.ID}
			database.DB.Create(&pi)
		}

		current := pi.EffectiveQuantity()
		var newQty float64
		switch change.action {
		case "add":
			newQty = current + change.quantity
		case "set":
			newQty = change.quantity
		case "remove":
			newQty = 0
		}

		database.DB.Model(&pi).Updates(map[string]interface{}{
			"manual_quantity": &newQty,
			"last_updated":    time.Now(),
		})
		updatedNames = append(updatedNames, fmt.Sprintf("%s (%.1f %s)", ingredientNames[ingID], newQty, change.unit))
	}

	return jsonString(map[string]any{"ok": true, "updated_items": updatedNames}), nil
}

type pantryChange struct {
	quantity float64
	unit     string
	action   string
	brand    *string
	rawName  string
}
