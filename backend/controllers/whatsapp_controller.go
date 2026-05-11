package controllers

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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
		if !persistWebhookIdempotencyKey(msgID) {
			return
		}
	}

	user, err := GetOrCreateWhatsAppUser(fromPhone)
	if err != nil {
		logger.Error("User Provisioning Error", "error", err)
		markTurnFailed(msgID, "user_provisioning_error")
		_ = client.SendTextMessage(fromPhone, "Sorry, I had trouble identifying your account.")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			logger.Error("WhatsApp Panic", "error", r, "msgID", msgID)
			markTurnFailed(msgID, "panic")
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
		if !persistWebhookIdempotencyKey(fallback) {
			return
		}
		contentKey := fallbackWebhookContentBucketKey(fromPhone, msgType, textBody, mediaID, ts)
		if !persistWebhookIdempotencyKey(contentKey) {
			return
		}
	}

	session := whatsapp.NewSession(user, msgID, logger.L().With("user_id", user.ID, "message_id", msgID))
	if msgID != "" {
		turnID := "turn-" + msgID
		session.TurnID = turnID
		_ = database.DB.Create(&models.ConversationTurn{
			UserID:    user.ID,
			MessageID: msgID,
			TurnID:    turnID,
			UserText:  textBody,
			Status:    "received",
		}).Error
		if !claimConversationTurnForProcessing(msgID) {
			logger.Info("Skipping message: turn claim failed", "message_id", msgID, "user_id", user.ID)
			return
		}
	}
	whatsapp.WithUserTurnLock(user.ID, func() {
		processWhatsAppIntent(session, textBody, imageBase64)
	})
}

func claimConversationTurnForProcessing(messageID string) bool {
	if strings.TrimSpace(messageID) == "" {
		return true
	}
	res := database.DB.Model(&models.ConversationTurn{}).
		Where("message_id = ? AND status IN ?", strings.TrimSpace(messageID), []string{"received", "retryable"}).
		Updates(map[string]any{
			"status":     "processing",
			"updated_at": time.Now(),
		})
	return res.Error == nil && res.RowsAffected == 1
}

func persistWebhookIdempotencyKey(key string) bool {
	if strings.TrimSpace(key) == "" {
		return true
	}
	if err := database.DB.Create(&models.ProcessedWebhook{MessageID: key}).Error; err != nil {
		errText := strings.ToLower(err.Error())
		if strings.Contains(errText, "duplicate") || strings.Contains(errText, "unique") {
			return false
		}
		logger.Error("Failed to persist webhook dedup key; continuing", "key", key, "error", err)
	}
	return true
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

func fallbackWebhookContentBucketKey(fromPhone, msgType, textBody, mediaID, timestamp string) string {
	bucket := time.Now().Unix() / 120
	if ts, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64); err == nil && ts > 0 {
		bucket = ts / 120
	}
	payload := strings.ToLower(strings.TrimSpace(fromPhone)) + "|" +
		strings.ToLower(strings.TrimSpace(msgType)) + "|" +
		strconv.FormatInt(bucket, 10) + "|" +
		strings.ToLower(strings.TrimSpace(textBody)) + "|" +
		strings.TrimSpace(mediaID)
	sum := sha256.Sum256([]byte(payload))
	return "content_bucket:" + hex.EncodeToString(sum[:16])
}

func markTurnFailed(messageID string, reason string) {
	if strings.TrimSpace(messageID) == "" {
		return
	}
	_ = database.DB.Model(&models.ConversationTurn{}).
		Where("message_id = ? AND status IN ?", messageID, []string{"received", "processing", "retryable"}).
		Updates(map[string]any{
			"status":         "failed",
			"assistant_text": fmt.Sprintf("failed: %s", strings.TrimSpace(reason)),
			"updated_at":     time.Now(),
		}).Error
}

func GetOrCreateWhatsAppUser(phone string) (*models.User, error) {
	return whatsapp.GetOrCreateWhatsAppUser(phone)
}
