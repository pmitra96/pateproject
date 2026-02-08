package controllers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/logger"
	"github.com/pmitra96/pateproject/models"
)

type LogMealRequest struct {
	Name        string   `json:"name"`
	Ingredients []string `json:"ingredients"`
	Calories    string   `json:"calories"`
	Protein     string   `json:"protein"`
}

type LogMealResponse struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Updated []string `json:"updated_items"`
}

func LogMeal(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil || userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req LogMealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	logger.Info("Logging meal", "user_id", userID, "meal", req.Name, "ingredients", len(req.Ingredients))

	updatedItems := []string{}

	// Track macros for the meal
	var totalCalories, totalProtein, totalCarbs, totalFat, totalFiber float64

	// Parse each ingredient and reduce pantry quantity
	// Ingredients are in format like "100g Paneer", "2 Eggs", "1 cup Rice"
	for _, ingredient := range req.Ingredients {
		ingredientName, quantity, unit := parseIngredient(ingredient)
		if ingredientName == "" {
			continue
		}

		// Find matching pantry item by ingredient name (fuzzy match)
		var pantryItems []models.PantryItem
		database.DB.Preload("Ingredient").Preload("Item").Where("user_id = ?", userID).Find(&pantryItems)

		for _, pi := range pantryItems {
			if matchesIngredient(pi.Ingredient.Name, ingredientName) {
				// Convert quantity to base units if needed
				reduction := convertToBaseUnit(quantity, unit, pi.Ingredient.Name)

				// Calculate macros based on the item's nutrition per 100g/ml
				if pi.Item.ID > 0 {
					factor := reduction / 100.0 // Nutrition is per 100g
					totalCalories += pi.Item.Calories * factor
					totalProtein += pi.Item.Protein * factor
					totalCarbs += pi.Item.Carbs * factor
					totalFat += pi.Item.Fat * factor
					totalFiber += pi.Item.Fiber * factor
				}

				// Reduce the quantity
				newQty := pi.DerivedQuantity - reduction
				if newQty < 0 {
					newQty = 0
				}

				database.DB.Model(&pi).Update("derived_quantity", newQty)
				updatedItems = append(updatedItems, pi.Ingredient.Name)
				logger.Info("Reduced pantry item", "ingredient", pi.Ingredient.Name, "reduction", reduction, "new_qty", newQty)
				break
			}
		}
	}

	// Save ingredients as JSON
	ingredientsJSON, _ := json.Marshal(req.Ingredients)

	// Create meal log entry
	mealLog := models.MealLog{
		UserID:      userID,
		Name:        req.Name,
		Calories:    totalCalories,
		Protein:     totalProtein,
		Carbs:       totalCarbs,
		Fat:         totalFat,
		Fiber:       totalFiber,
		Ingredients: string(ingredientsJSON),
		LoggedAt:    time.Now(),
	}
	database.DB.Create(&mealLog)

	logger.Info("Meal logged to history", "meal_log_id", mealLog.ID, "calories", totalCalories, "protein", totalProtein)

	resp := struct {
		Status    string   `json:"status"`
		Message   string   `json:"message"`
		Updated   []string `json:"updated_items"`
		MealLogID uint     `json:"meal_log_id"`
		Macros    struct {
			Calories float64 `json:"calories"`
			Protein  float64 `json:"protein"`
			Carbs    float64 `json:"carbs"`
			Fat      float64 `json:"fat"`
			Fiber    float64 `json:"fiber"`
		} `json:"macros"`
	}{
		Status:    "success",
		Message:   "Meal logged successfully",
		Updated:   updatedItems,
		MealLogID: mealLog.ID,
	}
	resp.Macros.Calories = totalCalories
	resp.Macros.Protein = totalProtein
	resp.Macros.Carbs = totalCarbs
	resp.Macros.Fat = totalFat
	resp.Macros.Fiber = totalFiber

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// parseIngredient extracts quantity, unit, and name from ingredient strings
// Examples: "100g Paneer" -> ("Paneer", 100, "g"), "2 Eggs" -> ("Eggs", 2, "pcs")
func parseIngredient(ingredient string) (name string, quantity float64, unit string) {
	ingredient = strings.TrimSpace(ingredient)

	// Pattern: number + optional unit + name
	// Examples: "100g Paneer", "2 Eggs", "1 cup Rice", "200ml Milk"
	pattern := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(g|kg|ml|l|cup|cups|tbsp|tsp|pcs|pc|pieces?)?\s*(.+)$`)
	matches := pattern.FindStringSubmatch(strings.ToLower(ingredient))

	if len(matches) >= 4 {
		qty, _ := strconv.ParseFloat(matches[1], 64)
		unit = matches[2]
		name = strings.TrimSpace(matches[3])

		// Default unit to pcs if not specified
		if unit == "" {
			unit = "pcs"
		}

		return name, qty, unit
	}

	// Fallback: just return the whole string as name with qty=1
	return ingredient, 1, "pcs"
}

// matchesIngredient checks if pantry ingredient matches the parsed ingredient name
func matchesIngredient(pantryName, ingredientName string) bool {
	pantryLower := strings.ToLower(pantryName)
	ingredientLower := strings.ToLower(ingredientName)

	// Exact match
	if pantryLower == ingredientLower {
		return true
	}

	// Full substring match
	if strings.Contains(pantryLower, ingredientLower) {
		return true
	}
	if strings.Contains(ingredientLower, pantryLower) {
		return true
	}

	// Keyword-based matching: extract key food words and match
	// Common ingredient keywords to extract (last word is usually the ingredient)
	ingredientWords := strings.Fields(ingredientLower)
	pantryWords := strings.Fields(pantryLower)

	// Check if any significant word from pantry matches ingredient words
	for _, pw := range pantryWords {
		if len(pw) < 3 { // Skip short words like "of", "in", etc.
			continue
		}
		// Skip common modifiers
		if isModifierWord(pw) {
			continue
		}
		for _, iw := range ingredientWords {
			if len(iw) < 3 {
				continue
			}
			if isModifierWord(iw) {
				continue
			}
			// Match if words are similar (handles typos like "broccoli" vs "broccoll")
			if pw == iw || strings.HasPrefix(pw, iw) || strings.HasPrefix(iw, pw) {
				return true
			}
			// Levenshtein-like: if differ by 1-2 chars, still match (for typos)
			if len(pw) > 4 && len(iw) > 4 && similarWord(pw, iw) {
				return true
			}
		}
	}

	return false
}

// isModifierWord returns true for words that describe but don't identify ingredients
func isModifierWord(word string) bool {
	modifiers := map[string]bool{
		"organic": true, "fresh": true, "raw": true, "cooked": true,
		"firm": true, "soft": true, "whole": true, "sliced": true,
		"diced": true, "minced": true, "chopped": true, "natural": true,
		"premium": true, "artisanal": true, "homemade": true,
	}
	return modifiers[word]
}

// similarWord checks if two words differ by at most 2 characters (for typos)
func similarWord(a, b string) bool {
	if len(a) != len(b) && (len(a)-len(b) > 2 || len(b)-len(a) > 2) {
		return false
	}

	// Check if they share a common prefix of at least 5 chars
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	if minLen < 5 {
		return a == b
	}

	commonPrefix := 0
	for i := 0; i < minLen; i++ {
		if a[i] == b[i] {
			commonPrefix++
		} else {
			break
		}
	}

	return commonPrefix >= minLen-2
}

// convertToBaseUnit converts quantity to grams or ml based on units
func convertToBaseUnit(quantity float64, unit string, ingredientName string) float64 {
	switch strings.ToLower(unit) {
	case "kg":
		return quantity * 1000
	case "l":
		return quantity * 1000
	case "cup", "cups":
		// Approximate: 1 cup = 240ml for liquids, 150g for dry
		if strings.Contains(strings.ToLower(ingredientName), "milk") ||
			strings.Contains(strings.ToLower(ingredientName), "water") ||
			strings.Contains(strings.ToLower(ingredientName), "juice") {
			return quantity * 240
		}
		return quantity * 150
	case "tbsp":
		return quantity * 15
	case "tsp":
		return quantity * 5
	case "pcs", "pc", "piece", "pieces", "":
		return quantity
	default:
		return quantity
	}
}

// GetMealHistory returns all logged meals for the user
func GetMealHistory(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil || userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var mealLogs []models.MealLog
	database.DB.Where("user_id = ?", userID).Order("logged_at DESC").Find(&mealLogs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mealLogs)
}

// DeleteMealLog deletes a meal log and restores pantry quantities
func DeleteMealLog(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil || userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get meal log ID from URL
	path := r.URL.Path
	parts := strings.Split(path, "/")
	mealLogIDStr := parts[len(parts)-1]
	mealLogID, err := strconv.ParseUint(mealLogIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid meal log ID", http.StatusBadRequest)
		return
	}

	// Find the meal log
	var mealLog models.MealLog
	if err := database.DB.Where("id = ? AND user_id = ?", mealLogID, userID).First(&mealLog).Error; err != nil {
		http.Error(w, "Meal log not found", http.StatusNotFound)
		return
	}

	// Parse ingredients from the meal log
	var ingredients []string
	json.Unmarshal([]byte(mealLog.Ingredients), &ingredients)

	// Restore pantry quantities
	restoredItems := []string{}
	for _, ingredient := range ingredients {
		ingredientName, quantity, unit := parseIngredient(ingredient)
		if ingredientName == "" {
			continue
		}

		var pantryItems []models.PantryItem
		database.DB.Preload("Ingredient").Where("user_id = ?", userID).Find(&pantryItems)

		for _, pi := range pantryItems {
			if matchesIngredient(pi.Ingredient.Name, ingredientName) {
				restoration := convertToBaseUnit(quantity, unit, pi.Ingredient.Name)
				newQty := pi.DerivedQuantity + restoration
				database.DB.Model(&pi).Update("derived_quantity", newQty)
				restoredItems = append(restoredItems, pi.Ingredient.Name)
				logger.Info("Restored pantry item", "ingredient", pi.Ingredient.Name, "restoration", restoration, "new_qty", newQty)
				break
			}
		}
	}

	// Delete the meal log
	database.DB.Delete(&mealLog)

	logger.Info("Meal log deleted and pantry restored", "meal_log_id", mealLogID, "restored_items", len(restoredItems))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "success",
		"message":        "Meal deleted and pantry restored",
		"restored_items": restoredItems,
	})
}
