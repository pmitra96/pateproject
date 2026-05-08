package controllers

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/pmitra96/pateproject/config"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/logger"
	"github.com/pmitra96/pateproject/models"
	"github.com/pmitra96/pateproject/services/whatsapp"
)

var runWhatsAppPayloadAsync = func(payload map[string]interface{}) {
	go processWhatsAppPayload(payload)
}

var newWhatsAppClient = func() whatsapp.WhatsAppClient {
	return whatsapp.NewClient()
}

var processWhatsAppIntent = func(session *whatsapp.Session, textBody string, imageBase64 string) {
	orch := whatsapp.NewOrchestrator()
	orch.ProcessMessage(session, textBody, imageBase64)
}

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

	runWhatsAppPayloadAsync(payload)
}

func processWhatsAppPayload(payload map[string]interface{}) {
	entries, _ := payload["entry"].([]interface{})
	for _, entryRaw := range entries {
		entry, _ := entryRaw.(map[string]interface{})
		changes, _ := entry["changes"].([]interface{})
		for _, changeRaw := range changes {
			change, _ := changeRaw.(map[string]interface{})
			value, _ := change["value"].(map[string]interface{})
			messages, _ := value["messages"].([]interface{})
			for _, messageRaw := range messages {
				message, _ := messageRaw.(map[string]interface{})
				processSingleWhatsAppMessage(message)
			}
		}
	}
}

func processSingleWhatsAppMessage(message map[string]interface{}) {
	fromPhone, _ := message["from"].(string)
	msgID, _ := message["id"].(string)

	client := newWhatsAppClient()
	if msgID != "" {
		_ = client.MarkAsRead(msgID)
		if err := database.DB.Create(&models.ProcessedWebhook{MessageID: msgID}).Error; err != nil {
			errText := strings.ToLower(err.Error())
			if strings.Contains(errText, "duplicate") || strings.Contains(errText, "unique") {
				return
			}
			logger.Error("Failed to persist webhook dedup key; continuing", "msgID", msgID, "error", err)
		}
	}

	user, err := GetOrCreateWhatsAppUser(fromPhone)
	if err != nil {
		logger.Error("User Provisioning Error", "error", err)
		_ = client.SendTextMessage(fromPhone, "Sorry, I had trouble identifying your account.")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			logger.Error("WhatsApp Panic", "error", r, "msgID", msgID)
			_ = client.SendTextMessage(fromPhone, "⚠️ I encountered a technical error. Please try again in a few minutes!")
		}
	}()

	var textBody string
	var imageBase64 string
	var mediaID string
	msgType, _ := message["type"].(string)

	if msgType == "text" {
		textObj, _ := message["text"].(map[string]interface{})
		textBody, _ = textObj["body"].(string)
	} else if msgType == "image" {
		imageObj, _ := message["image"].(map[string]interface{})
		mediaID, _ = imageObj["id"].(string)
		textBody, _ = imageObj["caption"].(string)

		imgBytes, err := client.DownloadMedia(mediaID)
		if err == nil {
			imageBase64 = base64.StdEncoding.EncodeToString(imgBytes)
		}
	}

	if textBody == "" && imageBase64 == "" {
		return
	}

	if msgID == "" {
		ts, _ := message["timestamp"].(string)
		fallback := fallbackWebhookMessageKey(fromPhone, msgType, textBody, mediaID, ts)
		if err := database.DB.Create(&models.ProcessedWebhook{MessageID: fallback}).Error; err != nil {
			errText := strings.ToLower(err.Error())
			if strings.Contains(errText, "duplicate") || strings.Contains(errText, "unique") {
				return
			}
			logger.Error("Failed to persist fallback webhook dedup key; continuing", "fallback_key", fallback, "error", err)
		}
	}

	session := whatsapp.NewSession(user, msgID, logger.L().With("user_id", user.ID, "message_id", msgID))
	processWhatsAppIntent(session, textBody, imageBase64)
}

func fallbackWebhookMessageKey(fromPhone, msgType, textBody, mediaID, timestamp string) string {
	payload := strings.ToLower(strings.TrimSpace(fromPhone)) + "|" +
		strings.ToLower(strings.TrimSpace(msgType)) + "|" +
		strings.TrimSpace(timestamp) + "|" +
		strings.ToLower(strings.TrimSpace(textBody)) + "|" +
		strings.TrimSpace(mediaID)
	sum := sha256.Sum256([]byte(payload))
	return "fallback:" + hex.EncodeToString(sum[:16])
}

func GetOrCreateWhatsAppUser(phone string) (*models.User, error) {
	return whatsapp.GetOrCreateWhatsAppUser(phone)
}
