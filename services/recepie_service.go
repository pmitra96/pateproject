package services

import "my-gin-app/models"

func GenerateRecipe(ingredients, preferences string) models.Recipe {
	// Some logic to generate the recipe
	return models.Recipe{
		Name:            "Spaghetti Bolognese",
		Ingredients:     "Pasta, tomatoes, garlic",
		Instructions:    "Cook pasta, prepare sauce, mix together",
		NutritionalInfo: "500 kcal",
	}
}
