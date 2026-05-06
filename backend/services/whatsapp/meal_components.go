package whatsapp

import (
	"strings"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/models"
	"github.com/pmitra96/pateproject/services"
)

func syncMealComponents(userID uint, mealLogID uint, ingredients string, estimate *services.FoodEstimate) {
	_ = database.DB.Where("meal_log_id = ?", mealLogID).Delete(&models.MealComponent{}).Error

	component := models.MealComponent{
		UserID:     userID,
		MealLogID:  mealLogID,
		Name:       componentNameFromIngredients(ingredients),
		Quantity:   1,
		Unit:       "serving",
		SourceType: "estimate",
		Confidence: 0.6,
	}
	if estimate != nil {
		component.Calories = estimate.Calories
		component.Protein = estimate.Protein
		component.Carbs = estimate.Carbs
		component.Fat = estimate.Fat
		component.Fiber = estimate.Fiber
		if strings.TrimSpace(estimate.ServingSize) != "" {
			component.Unit = estimate.ServingSize
		}
	}
	_ = database.DB.Create(&component).Error
}

func componentNameFromIngredients(ingredients string) string {
	parts := strings.Split(strings.TrimSpace(ingredients), ",")
	if len(parts) == 0 {
		return "meal_component"
	}
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return "meal_component"
	}
	return name
}
