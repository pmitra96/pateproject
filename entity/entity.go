package entity

import (
	"encoding/json"
	"time"
)

// Item represents an ingredient or item.
type Item struct {
	ID              uint    `json:"id"`
	Name            string  `json:"name"`
	Category        string  `json:"category"`
	DefaultUnitID   uint    `json:"default_unit_id"`
	CaloriesPerUnit float64 `json:"calories_per_unit"`
}

// Unit represents a measurement unit for ingredients.
type Unit struct {
	ID               uint    `json:"id"`
	Name             string  `json:"name"`
	Abbreviation     string  `json:"abbreviation"`
	ConversionFactor float64 `json:"conversion_factor"`
}

// User represents an application user.
type User struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  string    `json:"password"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserInventory represents the inventory of a user.
type UserInventory struct {
	ID           uint      `json:"id"`
	UserID       uint      `json:"user_id"`
	IngredientID uint      `json:"ingredient_id"`
	Quantity     float64   `json:"quantity"`
	UnitID       uint      `json:"unit_id"`
	LastUpdated  time.Time `json:"last_updated_at"`
}

// Recipe represents a recipe in the system.
type Recipe struct {
	ID          uint    `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Calories    float64 `json:"calories"`
}

// RecipeIngredient represents the ingredients required for a recipe.
type RecipeIngredient struct {
	ID           uint    `json:"id"`
	RecipeID     uint    `json:"recipe_id"`
	IngredientID uint    `json:"ingredient_id"`
	Quantity     float64 `json:"quantity"`
	UnitID       uint    `json:"unit_id"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// MarshalJSON implements the custom JSON serialization for User
func (u User) MarshalJSON() ([]byte, error) {
	type Alias User // Create an alias to avoid infinite recursion
	return json.Marshal(&struct {
		*Alias
		Password string `json:"-"` // Exclude password field
	}{
		Alias:    (*Alias)(&u),
		Password: "",
	})
}
