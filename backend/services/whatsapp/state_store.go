package whatsapp

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/models"
)

func getConversationState(userID uint) *models.ConversationState {
	var st models.ConversationState
	if err := database.DB.Where("user_id = ?", userID).First(&st).Error; err == nil {
		return &st
	}
	st = models.ConversationState{
		UserID:    userID,
		SessionID: time.Now().Format("20060102"),
	}
	database.DB.Create(&st)
	return &st
}

func updateConversationStateAfterReply(st *models.ConversationState, intent, reply string) {
	if st == nil {
		return
	}
	st.LastIntent = intent
	st.LastTool = ""
	st.LastToolResult = reply
	st.PendingMealAction = ""
	st.PendingMealOptions = ""
	st.PendingSlots = ""
	st.TurnCount++
	st.UpdatedAt = time.Now()
	database.DB.Model(&models.ConversationState{}).Where("user_id = ?", st.UserID).Updates(map[string]interface{}{
		"last_intent":          st.LastIntent,
		"last_tool":            st.LastTool,
		"last_tool_result":     st.LastToolResult,
		"pending_meal_action":  st.PendingMealAction,
		"pending_meal_options": st.PendingMealOptions,
		"pending_slots":        st.PendingSlots,
		"turn_count":           st.TurnCount,
		"updated_at":           st.UpdatedAt,
	})
}

func updateConversationStateAfterTool(st *models.ConversationState, intent, toolName, toolResult string) {
	if st == nil {
		return
	}
	st.LastIntent = intent
	st.LastTool = toolName
	st.LastToolResult = toolResult
	st.PendingMealAction = ""
	st.PendingMealOptions = ""
	st.PendingSlots = ""
	st.TurnCount++
	st.UpdatedAt = time.Now()
	database.DB.Model(&models.ConversationState{}).Where("user_id = ?", st.UserID).Updates(map[string]interface{}{
		"last_intent":          st.LastIntent,
		"last_tool":            st.LastTool,
		"last_tool_result":     st.LastToolResult,
		"pending_meal_action":  st.PendingMealAction,
		"pending_meal_options": st.PendingMealOptions,
		"pending_slots":        st.PendingSlots,
		"turn_count":           st.TurnCount,
		"updated_at":           st.UpdatedAt,
	})
}

func setPendingMealSelection(st *models.ConversationState, action string, mealIDs []uint, meta map[string]any) {
	if st == nil {
		return
	}
	raw, _ := json.Marshal(mealIDs)
	if meta == nil {
		meta = map[string]any{}
	}
	metaRaw, _ := json.Marshal(meta)
	st.PendingMealAction = action
	st.PendingMealOptions = string(raw)
	st.PendingSlots = string(metaRaw)
	st.UpdatedAt = time.Now()
	database.DB.Model(&models.ConversationState{}).Where("user_id = ?", st.UserID).Updates(map[string]interface{}{
		"pending_meal_action":  st.PendingMealAction,
		"pending_meal_options": st.PendingMealOptions,
		"pending_slots":        st.PendingSlots,
		"updated_at":           st.UpdatedAt,
	})
}

func clearPendingMealSelection(st *models.ConversationState) {
	if st == nil {
		return
	}
	st.PendingMealAction = ""
	st.PendingMealOptions = ""
	st.PendingSlots = ""
	st.UpdatedAt = time.Now()
	database.DB.Model(&models.ConversationState{}).Where("user_id = ?", st.UserID).Updates(map[string]interface{}{
		"pending_meal_action":  "",
		"pending_meal_options": "",
		"pending_slots":        "",
		"updated_at":           st.UpdatedAt,
	})
}

func getPendingMealIDs(st *models.ConversationState) []uint {
	if st == nil || st.PendingMealOptions == "" {
		return nil
	}
	var ids []uint
	_ = json.Unmarshal([]byte(st.PendingMealOptions), &ids)
	return ids
}

func getPendingSelectionMeta(st *models.ConversationState) map[string]any {
	if st == nil || strings.TrimSpace(st.PendingSlots) == "" {
		return map[string]any{}
	}
	meta := map[string]any{}
	if err := json.Unmarshal([]byte(st.PendingSlots), &meta); err != nil {
		return map[string]any{}
	}
	return meta
}
