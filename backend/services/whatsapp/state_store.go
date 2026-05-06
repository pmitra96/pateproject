package whatsapp

import (
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
	st.TurnCount++
	st.UpdatedAt = time.Now()
	database.DB.Model(&models.ConversationState{}).Where("user_id = ?", st.UserID).Updates(map[string]interface{}{
		"last_intent":      st.LastIntent,
		"last_tool":        st.LastTool,
		"last_tool_result": st.LastToolResult,
		"turn_count":       st.TurnCount,
		"updated_at":       st.UpdatedAt,
	})
}

func updateConversationStateAfterTool(st *models.ConversationState, intent, toolName, toolResult string) {
	if st == nil {
		return
	}
	st.LastIntent = intent
	st.LastTool = toolName
	st.LastToolResult = toolResult
	st.TurnCount++
	st.UpdatedAt = time.Now()
	database.DB.Model(&models.ConversationState{}).Where("user_id = ?", st.UserID).Updates(map[string]interface{}{
		"last_intent":      st.LastIntent,
		"last_tool":        st.LastTool,
		"last_tool_result": st.LastToolResult,
		"turn_count":       st.TurnCount,
		"updated_at":       st.UpdatedAt,
	})
}
