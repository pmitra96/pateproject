package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/llm"
	"github.com/pmitra96/pateproject/logger"
	"github.com/pmitra96/pateproject/models"
)

type SaveConversationRequest struct {
	Messages []llm.ChatMessage `json:"messages"`
}

type ConversationResponse struct {
	ID        uint   `json:"id"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"created_at"`
}

// SaveConversation saves a chat conversation with auto-generated summary
func SaveConversation(w http.ResponseWriter, r *http.Request) {
	logger.Info("Received save conversation request")

	userID, err := getUserID(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	var req SaveConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if len(req.Messages) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "No messages to save"})
		return
	}

	// Generate summary using LLM
	client := llm.NewClient()
	summary, err := client.SummarizeConversation(req.Messages)
	if err != nil {
		logger.Error("Failed to summarize conversation", "error", err)
		summary = "Conversation about pantry and meals"
	}

	// Serialize messages to JSON
	messagesJSON, err := json.Marshal(req.Messages)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to serialize messages"})
		return
	}

	// Save to database
	conversation := models.Conversation{
		UserID:   userID,
		Summary:  summary,
		Messages: string(messagesJSON),
	}

	if err := database.DB.Create(&conversation).Error; err != nil {
		logger.Error("Failed to save conversation", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save conversation"})
		return
	}

	logger.Info("Conversation saved", "user_id", userID, "conversation_id", conversation.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ConversationResponse{
		ID:        conversation.ID,
		Summary:   conversation.Summary,
		CreatedAt: conversation.CreatedAt.Format("2006-01-02 15:04"),
	})
}

// GetConversations fetches all conversation summaries for a user
func GetConversations(w http.ResponseWriter, r *http.Request) {
	logger.Info("Received get conversations request")

	userID, err := getUserID(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	var conversations []models.Conversation
	if err := database.DB.Where("user_id = ?", userID).Order("created_at desc").Limit(20).Find(&conversations).Error; err != nil {
		logger.Error("Failed to fetch conversations", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch conversations"})
		return
	}

	// Convert to response format
	response := make([]ConversationResponse, len(conversations))
	for i, conv := range conversations {
		response[i] = ConversationResponse{
			ID:        conv.ID,
			Summary:   conv.Summary,
			CreatedAt: conv.CreatedAt.Format("2006-01-02 15:04"),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
