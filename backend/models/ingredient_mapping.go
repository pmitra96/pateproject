package models

import (
	"time"
)

type IngredientMapping struct {
	ID             uint      `gorm:"primaryKey"`
	RawName        string    `gorm:"index:idx_raw_name,unique;not null"`
	IngredientID   uint      `gorm:"not null"`
	IngredientName string    `gorm:"not null"`
	Brand          *string
	Product        *string
	CreatedAt      time.Time
}
