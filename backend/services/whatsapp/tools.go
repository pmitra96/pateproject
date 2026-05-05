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
		return "I couldn't identify any specific meals in your message. Could you please specify what you ate?", nil
	}

	var summary []string
	for _, m := range meals {
		mealData, _ := m.(map[string]interface{})
		dishName, _ := mealData["dish_name"].(string)
		ingredients, _ := mealData["ingredients"].(string)
		mealType, _ := mealData["meal_type"].(string)

		estimated, err := ns.EstimateNutritionFromQuery(s.User.ID, ingredients)
		if err != nil || estimated == nil {
			summary = append(summary, fmt.Sprintf("⚠️ *Could not estimate %s*: I had trouble calculating the calories for this item.", dishName))
			continue
		}

		preState, _ := services.ComputeRemainingDayState(s.User.ID, time.Now())
		controlMode := "NORMAL"
		if preState != nil {
			controlMode = preState.ControlMode
		}

		mealLog := models.MealLog{
			UserID:           s.User.ID,
			Name:             dishName,
			MealType:         mealType,
			Ingredients:      ingredients,
			Calories:         estimated.Calories,
			Protein:          estimated.Protein,
			Carbs:            estimated.Carbs,
			Fat:              estimated.Fat,
			LoggedAt:         time.Now(),
			ControlModeAtLog: controlMode,
		}
		database.DB.Create(&mealLog)
		summary = append(summary, fmt.Sprintf("- %s (%.0f kcal)", dishName, estimated.Calories))
	}

	if len(summary) == 0 {
		return "I had some trouble estimating the nutrition for those items.", nil
	}

	newState, _ := services.ComputeRemainingDayState(s.User.ID, time.Now())
	modeStr := ""
	if newState != nil && newState.ControlMode == "DAMAGE_CONTROL" {
		modeStr = "\n⚠️ WARNING: You are now in DAMAGE CONTROL mode!"
	}

	resp := MsgMealLogged + "\n" + strings.Join(summary, "\n")
	if newState != nil {
		resp += fmt.Sprintf("\n\n*Remaining today:* %.0f kcal, %.1fg protein.%s",
			newState.RemainingCalories, newState.RemainingProtein, modeStr)
	}
	return resp, nil
}

func HandleSetGoal(s *Session, args map[string]interface{}) (string, error) {
	calories := int(args["calories"].(float64))
	if calories <= 0 {
		return "Please specify a valid calorie number above zero.", nil
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
		}
		database.DB.Create(&profile)
	} else {
		database.DB.Model(&profile).Updates(map[string]interface{}{
			"daily_calorie_target": calories,
			"updated_at":           time.Now(),
		})
	}
	return fmt.Sprintf(MsgGoalUpdated, calories), nil
}

func HandleAskAdvice(s *Session, args map[string]interface{}) (string, error) {
	ns := services.NewNutritionService()
	foodName, _ := args["food_description"].(string)

	state, _ := services.ComputeRemainingDayState(s.User.ID, time.Now())
	if state == nil {
		return "I couldn't find your daily goals. Please set them up first!", nil
	}

	estimated, err := ns.EstimateNutritionFromQuery(s.User.ID, foodName)
	if err != nil || estimated == nil {
		return "I'm having trouble estimating the nutrition for that right now.", nil
	}

	result := services.CheckFoodPermission(state, *estimated)
	decision := "❌ No, you shouldn't eat this."
	if result.Status == services.StatusAllow || result.Status == services.StatusAllowWithConstraint {
		decision = "✅ Yes, you can eat this!"
	}

	return fmt.Sprintf("%s\n\n%s\n\nNutrition Est: %.0f kcal, %.1fg protein.", decision, result.Reason, estimated.Calories, estimated.Protein), nil
}

func HandleGetBudget(s *Session, args map[string]interface{}) (string, error) {
	state, _ := services.ComputeRemainingDayState(s.User.ID, time.Now())
	if state == nil {
		return "You haven't set up your goals yet!", nil
	}
	return fmt.Sprintf("📊 *Your Daily Budget:*\n\nRemaining Calories: %.0f kcal\nRemaining Protein: %.1f g\nRemaining Carbs: %.1f g\nRemaining Fat: %.1f g\n\nCurrent Mode: %s",
		state.RemainingCalories, state.RemainingProtein, state.RemainingCarbs, state.RemainingFat, state.ControlMode), nil
}

func HandleUpdateProfile(s *Session, args map[string]interface{}) (string, error) {
	var prefs models.UserPreferences
	if err := database.DB.Where("user_id = ?", s.User.ID).First(&prefs).Error; err != nil {
		prefs = models.UserPreferences{UserID: s.User.ID}
		database.DB.Create(&prefs)
	}

	updates := make(map[string]interface{})
	if h, ok := args["height"].(float64); ok { updates["height"] = h }
	if w, ok := args["weight"].(float64); ok { updates["weight"] = w }
	if a, ok := args["age"].(float64); ok { updates["age"] = int(a) }
	if g, ok := args["gender"].(string); ok { updates["gender"] = g }

	if len(updates) == 0 {
		return "I didn't see any profile details to update.", nil
	}

	database.DB.Model(&models.UserPreferences{}).Where("user_id = ?", s.User.ID).Updates(updates)
	return MsgProfileUpdated, nil
}

func HandleUpdatePantry(s *Session, args map[string]interface{}) (string, error) {
	llmClient := llm.NewClient()
	items, ok := args["items"].([]interface{})
	if !ok {
		return "I couldn't understand the pantry items.", nil
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
		case "add": newQty = current + change.quantity
		case "set": newQty = change.quantity
		case "remove": newQty = 0
		}

		database.DB.Model(&pi).Updates(map[string]interface{}{
			"manual_quantity": &newQty,
			"last_updated":    time.Now(),
		})
		updatedNames = append(updatedNames, fmt.Sprintf("%s (%.1f %s)", ingredientNames[ingID], newQty, change.unit))
	}

	return fmt.Sprintf("📦 *Pantry Updated!*\nUpdated: %s", strings.Join(updatedNames, ", ")), nil
}

type pantryChange struct {
	quantity float64
	unit     string
	action   string
	brand    *string
	rawName  string
}
