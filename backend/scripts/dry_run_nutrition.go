package main

import (
	"fmt"

	"github.com/pmitra96/pateproject/models"
	"github.com/pmitra96/pateproject/services"
)

func main() {
	svc := services.NewNutritionService()

	testItems := []struct {
		Name       string
		Ingredient string
		Brand      string
		Unit       string
	}{
		{"Mooz Organic Soya Tofu", "Soya Tofu", "Mooz", "g"},
		{"Amul Taaza Toned Milk", "Milk", "Amul", "ml"},
		{"Akshayakalpa Organic Set Curd", "Curd", "Akshayakalpa", "g"},
		{"Urban Platter Tofu", "Tofu", "Urban Platter", "g"},
		{"Tata Sampann Kala Chana", "Kala Chana", "Tata Sampann", "g"},
		{"DeHaat Honest Farms Peanuts Whole", "Peanuts", "DeHaat Honest Farms", "g"},
		{"White Eggs", "Eggs", "Local", "pc"},
	}

	fmt.Println("=== Nutrition Dry Run ===")
	for _, ti := range testItems {
		item := &models.Item{
			ProductName: ti.Name,
			Ingredient:  models.Ingredient{Name: ti.Ingredient},
			Brand:       &models.Brand{Name: ti.Brand},
			Unit:        ti.Unit,
		}

		fmt.Printf("\nTesting Item: %s\n", ti.Name)
		err := svc.FetchItemNutrition(item)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("Calories: %.2f kcal\n", item.Calories)
		fmt.Printf("Protein:  %.2f g\n", item.Protein)
		fmt.Printf("Carbs:    %.2f g\n", item.Carbs)
		fmt.Printf("Fats:     %.2f g\n", item.Fat)
		fmt.Printf("Fiber:    %.2f g\n", item.Fiber)
		fmt.Printf("Verified: %v\n", item.NutritionVerified)
	}
}
