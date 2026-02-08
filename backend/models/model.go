package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents an authenticated user in the system.
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Email     string         `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Name      string         `gorm:"size:255" json:"name"`
	Password  string         `gorm:"size:255;default:''" json:"-"` // Added to match existing DB constraint
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Identities  []UserIdentity `json:"identities,omitempty"`
	PantryItems []PantryItem   `json:"pantry_items,omitempty"`
	Orders      []Order        `json:"orders,omitempty"`
}

// UserIdentity maps an external provider's user ID to a local user.
type UserIdentity struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"not null;index" json:"user_id"`
	Provider   string    `gorm:"size:50;not null" json:"provider"`           // google, zepto, blinkit, etc.
	ExternalID string    `gorm:"size:255;not null;index" json:"external_id"` // Provider's user ID (e.g. Google sub)
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Ingredient represents a canonical, brand-agnostic ingredient name.
type Ingredient struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:255;uniqueIndex;not null" json:"name"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Brand represents a manufacturer or brand name.
type Brand struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:255;uniqueIndex;not null" json:"name"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Item represents a specific product linked to an ingredient and optionally a brand.
type Item struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	Name          string         `gorm:"size:255;uniqueIndex;not null" json:"name"` // Full display name
	IngredientID  uint           `gorm:"not null;index" json:"ingredient_id"`
	BrandID       *uint          `gorm:"index" json:"brand_id"`
	ProductName   string         `gorm:"size:255" json:"product_name"` // e.g., "Taaza Toned Milk"
	DefaultUnitID uint           `gorm:"default:1" json:"-"`
	Unit          string         `gorm:"size:50;not null" json:"unit"` // Normalized unit (g, ml, pcs)
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	Ingredient Ingredient `gorm:"foreignKey:IngredientID" json:"ingredient,omitempty"`
	Brand      *Brand     `gorm:"foreignKey:BrandID" json:"brand,omitempty"`

	// Nutritional Info (per 100g or 100ml, or per unit if non-divisible)
	Calories          float64 `gorm:"default:0" json:"calories"`
	Protein           float64 `gorm:"default:0" json:"protein"`
	Carbs             float64 `gorm:"default:0" json:"carbs"`
	Fat               float64 `gorm:"default:0" json:"fat"`
	Fiber             float64 `gorm:"default:0" json:"fiber"`
	NutritionVerified bool    `gorm:"default:false" json:"nutrition_verified"`
}

// Order represents an ingested grocery order.
// Uniqueness constraint on (ExternalOrderID, Provider) or similar logic needed.
// Prompt says: Deduplicate using external_order_id OR email_message_id.
// So we will store both and ensure uniqueness if present.
type Order struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	UserID          uint           `gorm:"not null;index" json:"user_id"`
	ExternalOrderID string         `gorm:"size:255;index" json:"external_order_id"` // Provider's ID
	EmailMessageID  string         `gorm:"size:255;index" json:"email_message_id"`  // For email parsers
	Provider        string         `gorm:"size:50;not null" json:"provider"`        // zepto, blinkit, instamart
	OrderDate       time.Time      `json:"order_date"`
	Status          string         `gorm:"size:50;default:'processed'" json:"status"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	OrderItems []OrderItem `json:"items,omitempty"`
}

// OrderItem links an order to a canonical item.
// It preserves the Raw Name from the provider.
type OrderItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OrderID   uint      `gorm:"not null;index" json:"order_id"`
	ItemID    uint      `gorm:"not null;index" json:"item_id"`
	RawName   string    `gorm:"size:255;not null" json:"raw_name"` // What the receipt said
	Quantity  float64   `gorm:"not null" json:"quantity"`
	CreatedAt time.Time `json:"created_at"`

	Item Item `gorm:"foreignKey:ItemID" json:"item,omitempty"`
}

// PantryItem represents the current state of an item in a user's pantry.
// We aggregate by IngredientID to handle different brands/products of the same ingredient.
type PantryItem struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `gorm:"not null;uniqueIndex:idx_user_ingredient" json:"user_id"`
	IngredientID    uint      `gorm:"not null;uniqueIndex:idx_user_ingredient" json:"ingredient_id"`
	ItemID          uint      `gorm:"not null" json:"item_id"` // Representative item (most recently purchased)
	DerivedQuantity float64   `gorm:"default:0" json:"derived_quantity"`
	ManualQuantity  *float64  `json:"manual_quantity"`
	LastUpdated     time.Time `gorm:"autoUpdateTime" json:"last_updated"`

	Ingredient Ingredient `gorm:"foreignKey:IngredientID" json:"ingredient"`
	Item       Item       `gorm:"foreignKey:ItemID" json:"item"`
}

// EffectiveQuantity returns the quantity the user sees.
func (p *PantryItem) EffectiveQuantity() float64 {
	if p.ManualQuantity != nil {
		return *p.ManualQuantity
	}
	return p.DerivedQuantity
}

// Goal represents a health/fitness goal set by a user.
type Goal struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      uint           `gorm:"not null;index" json:"user_id"`
	Title       string         `gorm:"size:255;not null" json:"title"`
	Description string         `gorm:"type:text" json:"description"`
	TargetDate  *time.Time     `json:"target_date,omitempty"`
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}
