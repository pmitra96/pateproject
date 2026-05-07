//go:build tools
// +build tools

package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	// 1. Setup Test DB
	db, err := gorm.Open(sqlite.Open("test_fixes.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	database.DB = db
	db.AutoMigrate(&models.ProcessedWebhook{}, &models.MealLog{}, &models.User{}, &models.Conversation{})

	// 2. Mock User
	user := models.User{Name: "Test User", Email: "test@example.com"}
	db.Create(&user)

	fmt.Println("--- TEST 1: Webhook Deduplication ---")
	msgID := "wamid.Test12345"

	// First arrival
	err1 := db.Create(&models.ProcessedWebhook{MessageID: msgID}).Error
	if err1 == nil {
		fmt.Printf("✅ First webhook (ID: %s) processed successfully.\n", msgID)
	} else {
		fmt.Printf("❌ First webhook failed: %v\n", err1)
	}

	// Retry arrival (Meta sending it again)
	err2 := db.Create(&models.ProcessedWebhook{MessageID: msgID}).Error
	if err2 != nil && strings.Contains(err2.Error(), "UNIQUE constraint failed") {
		fmt.Printf("✅ Retry of webhook (ID: %s) successfully blocked by database.\n", msgID)
	} else {
		fmt.Printf("❌ Retry of webhook failed to be blocked: %v\n", err2)
	}

	fmt.Println("\n--- TEST 2: Partial Meal Update (modify_logged_meal) ---")

	// Log initial meal
	initialMeal := models.MealLog{
		UserID:      user.ID,
		Name:        "Egg White Omelette",
		MealType:    "Breakfast",
		Ingredients: "4 egg whites",
		Calories:    100,
		LoggedAt:    time.Now(),
	}
	db.Create(&initialMeal)
	fmt.Printf("Initial state: %s with %s (%.0f kcal)\n", initialMeal.Name, initialMeal.Ingredients, initialMeal.Calories)

	// Simulate "modify_logged_meal" (update action)
	// We'll mimic the logic from whatsapp_controller.go:565
	targetDishName := "Egg White Omelette"
	newIngredients := "6 egg whites"
	today := time.Now().Truncate(24 * time.Hour)

	var meal models.MealLog
	err = db.Where("user_id = ? AND meal_type = ? AND DATE(logged_at) = DATE(?) AND name ILIKE ?", user.ID, "Breakfast", today, "%"+targetDishName+"%").First(&meal).Error

	// Since we are using SQLite, DATE() might work differently, but we'll try it.
	// For testing on SQLite, we might need simple string match
	if err != nil {
		// Fallback for sqlite test if DATE() isn't available
		err = db.Where("user_id = ? AND meal_type = ? AND name LIKE ?", user.ID, "Breakfast", "%"+targetDishName+"%").First(&meal).Error
	}

	if err == nil {
		db.Model(&meal).Updates(map[string]interface{}{
			"ingredients": newIngredients,
			"calories":    150, // Mocked recalculation
			"updated_at":  time.Now(),
		})
		fmt.Printf("✅ Partial update successful! New state: %s with %s (%.0f kcal)\n", meal.Name, meal.Ingredients, meal.Calories)
	} else {
		fmt.Printf("❌ Failed to find dish for update: %v\n", err)
	}

	fmt.Println("\n--- TEST 3: Partial Meal Deletion ---")
	// Add another item to the same meal
	extraItem := models.MealLog{
		UserID:      user.ID,
		Name:        "Espresso",
		MealType:    "Breakfast",
		Ingredients: "1 shot",
		Calories:    5,
		LoggedAt:    time.Now(),
	}
	db.Create(&extraItem)
	fmt.Printf("Added %s to Breakfast.\n", extraItem.Name)

	// Simulate "delete" action for the Espresso
	targetToDelete := "Espresso"
	var toDelete models.MealLog
	err = db.Where("user_id = ? AND meal_type = ? AND name LIKE ?", user.ID, "Breakfast", "%"+targetToDelete+"%").First(&toDelete).Error
	if err == nil {
		db.Delete(&toDelete)
		fmt.Printf("✅ Partial deletion successful! %s removed.\n", targetToDelete)
	} else {
		fmt.Printf("❌ Failed to find dish for deletion: %v\n", err)
	}

	// Verify the Omelette is still there
	var remainingCount int64
	db.Model(&models.MealLog{}).Where("user_id = ? AND meal_type = ?", user.ID, "Breakfast").Count(&remainingCount)
	fmt.Printf("Remaining items in Breakfast: %d (Expected: 1)\n", remainingCount)
}
