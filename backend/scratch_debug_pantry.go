package main

import (
	"fmt"
	"github.com/pmitra96/pateproject/config"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/models"
)

func main() {
	config.LoadEnv()
	database.InitDB()
	var count int64
	database.DB.Model(&models.PantryItem{}).Count(&count)
	fmt.Printf("Total Pantry Items: %d\n", count)
	
	var items []models.PantryItem
	database.DB.Preload("Ingredient").Find(&items)
	for _, it := range items {
		fmt.Printf("- User %d: %s (Qty: %.1f)\n", it.UserID, it.Ingredient.Name, it.EffectiveQuantity())
	}
}
