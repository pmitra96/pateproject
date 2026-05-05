package main

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/models"
)

func main() {
	godotenv.Load("../.env")
	database.InitDB()

	var identity models.UserIdentity
	database.DB.Where("external_id = ?", "915555555555").First(&identity)
	
	fmt.Printf("Test User ID: %d\n", identity.UserID)

	var meals []models.MealLog
	database.DB.Where("user_id = ?", identity.UserID).Order("logged_at asc").Find(&meals)
	fmt.Println("Meal Logs:")
	for _, m := range meals {
		fmt.Printf("- [%s] [%s] %s: %.0f kcal (%s)\n", m.LoggedAt.Format("15:04:05"), m.MealType, m.Name, m.Calories, m.Ingredients)
	}
}
