package model

import (
	"time"
)

// Item represents an ingredient in the system.
type Item struct {
	ID              uint    `gorm:"primaryKey" json:"id"`
	Name            string  `gorm:"size:255;not null" json:"name"`
	Category        string  `gorm:"size:255" json:"category"`
	DefaultUnitID   uint    `gorm:"not null" json:"default_unit_id"`
	CaloriesPerUnit float64 `json:"calories_per_unit"`
}

// Unit represents a measurement unit for ingredients.
type Unit struct {
	ID               uint    `gorm:"primaryKey" json:"id"`
	Name             string  `gorm:"size:255;not null" json:"name"`
	Abbreviation     string  `gorm:"size:10;not null" json:"abbreviation"`
	ConversionFactor float64 `gorm:"not null" json:"conversion_factor"`
}

// User represents an application user.
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	Email     string    `gorm:"size:255;unique;not null" json:"email"`
	Password  []byte    `gorm:"type:bytea;not null" json:"-"` // Hide password from JSON
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// UserInventory represents the inventory of a user.
type UserInventory struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"not null" json:"user_id"`
	IngredientID uint      `gorm:"not null" json:"ingredient_id"`
	Quantity     float64   `gorm:"not null" json:"quantity"`
	UnitID       uint      `gorm:"not null" json:"unit_id"`
	LastUpdated  time.Time `gorm:"autoUpdateTime" json:"last_updated_at"`
}

// Recipe represents a recipe in the system.
type Recipe struct {
	ID          uint    `gorm:"primaryKey" json:"id"`
	Name        string  `gorm:"size:255;not null" json:"name"`
	Description string  `gorm:"type:text" json:"description"`
	Calories    float64 `json:"calories"`
}

// RecipeIngredient represents the ingredients required for a recipe.
type RecipeIngredient struct {
	ID           uint    `gorm:"primaryKey" json:"id"`
	RecipeID     uint    `gorm:"not null" json:"recipe_id"`
	IngredientID uint    `gorm:"not null" json:"ingredient_id"`
	Quantity     float64 `gorm:"not null" json:"quantity"`
	UnitID       uint    `gorm:"not null" json:"unit_id"`
}

// Relationships

// Ingredient DefaultUnitID is a foreign key referencing Unit.ID.
// UserInventory IngredientID is a foreign key referencing Ingredient.ID.
// UserInventory UserID is a foreign key referencing User.ID.
// UserInventory UnitID is a foreign key referencing Unit.ID.
// RecipeIngredient RecipeID is a foreign key referencing Recipe.ID.
// RecipeIngredient IngredientID is a foreign key referencing Ingredient.ID.
// RecipeIngredient UnitID is a foreign key referencing Unit.ID.
