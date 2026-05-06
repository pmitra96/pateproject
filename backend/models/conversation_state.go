package models

import "time"

// ConversationState stores lightweight, structured state for deterministic routing.
type ConversationState struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	UserID             uint      `gorm:"not null;uniqueIndex" json:"user_id"`
	LastIntent         string    `gorm:"size:64" json:"last_intent"`
	PendingSlots       string    `gorm:"type:text" json:"pending_slots"`
	LastTool           string    `gorm:"size:64" json:"last_tool"`
	LastToolResult     string    `gorm:"type:text" json:"last_tool_result"`
	PendingMealAction  string    `gorm:"size:32" json:"pending_meal_action"`
	PendingMealOptions string    `gorm:"type:text" json:"pending_meal_options"` // JSON list of meal ids
	SessionID          string    `gorm:"size:128" json:"session_id"`
	TurnCount          int       `gorm:"default:0" json:"turn_count"`
	UpdatedAt          time.Time `json:"updated_at"`
	CreatedAt          time.Time `json:"created_at"`
}
