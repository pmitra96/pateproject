package controllers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/pmitra96/pateproject/config"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/logger"
	"github.com/pmitra96/pateproject/models"
	"github.com/pmitra96/pateproject/services/whatsapp"
)

// VerifyWhatsAppWebhook handles the GET request from Meta to verify the webhook URL.
func VerifyWhatsAppWebhook(w http.ResponseWriter, r *http.Request) {
	verifyToken := config.GetEnv("WHATSAPP_VERIFY_TOKEN", "")
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode == "subscribe" && token == verifyToken {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(challenge))
		return
	}
	w.WriteHeader(http.StatusForbidden)
}

// HandleWhatsAppMessage handles the POST request from Meta containing new messages.
func HandleWhatsAppMessage(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 200 OK to Meta quickly
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("EVENT_RECEIVED"))

	go processWhatsAppPayload(payload)
}

func processWhatsAppPayload(payload map[string]interface{}) {
	// 1. Parse Meta Payload
	entries, _ := payload["entry"].([]interface{})
	if len(entries) == 0 { return }
	entry, _ := entries[0].(map[string]interface{})
	changes, _ := entry["changes"].([]interface{})
	if len(changes) == 0 { return }
	change, _ := changes[0].(map[string]interface{})
	value, _ := change["value"].(map[string]interface{})
	messages, _ := value["messages"].([]interface{})
	if len(messages) == 0 { return }
	message, _ := messages[0].(map[string]interface{})

	fromPhone, _ := message["from"].(string)
	msgID, _ := message["id"].(string)

	// 2. Immediate Read Receipt & Deduplication
	client := whatsapp.NewClient()
	if msgID != "" {
		client.MarkAsRead(msgID)
		if err := database.DB.Create(&models.ProcessedWebhook{MessageID: msgID}).Error; err != nil {
			return // Duplicate message
		}
	}

	// 3. User Identification
	user, err := GetOrCreateWhatsAppUser(fromPhone)
	if err != nil {
		logger.Error("User Provisioning Error", "error", err)
		client.SendTextMessage(fromPhone, "Sorry, I had trouble identifying your account.")
		return
	}

	// 4. Panic Recovery
	defer func() {
		if r := recover(); r != nil {
			logger.Error("WhatsApp Panic", "error", r, "msgID", msgID)
			client.SendTextMessage(fromPhone, "⚠️ I encountered a technical error. Please try again in a few minutes!")
		}
	}()

	// 5. Resolve Content
	var textBody string
	var imageBase64 string
	msgType, _ := message["type"].(string)

	if msgType == "text" {
		textObj, _ := message["text"].(map[string]interface{})
		textBody, _ = textObj["body"].(string)
	} else if msgType == "image" {
		imageObj, _ := message["image"].(map[string]interface{})
		mediaID, _ := imageObj["id"].(string)
		textBody, _ = imageObj["caption"].(string)
		
		imgBytes, err := client.DownloadMedia(mediaID)
		if err == nil {
			imageBase64 = base64.StdEncoding.EncodeToString(imgBytes)
		}
	}

	if textBody == "" && imageBase64 == "" {
		return
	}
	
	// 6. Orchestrate Intent
	session := whatsapp.NewSession(user, msgID, logger.L().With("user_id", user.ID, "message_id", msgID))
	orch := whatsapp.NewOrchestrator()
	orch.ProcessMessage(session, textBody, imageBase64)
}

func GetOrCreateWhatsAppUser(phone string) (*models.User, error) {
	return whatsapp.GetOrCreateWhatsAppUser(phone)
}
