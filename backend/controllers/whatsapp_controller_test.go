package controllers

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/models"
	"github.com/pmitra96/pateproject/services/whatsapp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type testWhatsAppClient struct {
	markReadIDs []string
}

func (c *testWhatsAppClient) SendTextMessage(to string, text string) error { return nil }
func (c *testWhatsAppClient) MarkAsRead(msgID string) error {
	c.markReadIDs = append(c.markReadIDs, msgID)
	return nil
}
func (c *testWhatsAppClient) DownloadMedia(mediaID string) ([]byte, error) { return nil, nil }

func setupControllerTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	database.DB = db
	if err := database.DB.AutoMigrate(
		&models.User{},
		&models.UserIdentity{},
		&models.Goal{},
		&models.GoalMacroProfile{},
		&models.ProcessedWebhook{},
		&models.LLMUsageLog{},
		&models.Conversation{},
		&models.UserPreferences{},
		&models.RemainingDayState{},
	); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
}

func TestHandleWhatsAppMessage_AcksAndDispatchesAsync(t *testing.T) {
	setupControllerTestDB(t)

	oldAsync := runWhatsAppPayloadAsync
	defer func() { runWhatsAppPayloadAsync = oldAsync }()

	dispatched := false
	runWhatsAppPayloadAsync = func(payload map[string]interface{}) {
		dispatched = true
	}

	body := map[string]interface{}{"entry": []interface{}{}}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/whatsapp/webhook", bytes.NewReader(raw))
	rr := httptest.NewRecorder()

	HandleWhatsAppMessage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "EVENT_RECEIVED" {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
	if !dispatched {
		t.Fatal("expected async dispatcher to be called")
	}
}

func TestProcessWhatsAppPayload_ProcessesAllMessages(t *testing.T) {
	setupControllerTestDB(t)

	oldClient := newWhatsAppClient
	oldIntent := processWhatsAppIntent
	defer func() {
		newWhatsAppClient = oldClient
		processWhatsAppIntent = oldIntent
	}()

	client := &testWhatsAppClient{}
	newWhatsAppClient = func() whatsapp.WhatsAppClient { return client }

	var mu sync.Mutex
	intentCalls := 0
	processWhatsAppIntent = func(session *whatsapp.Session, textBody string, imageBase64 string) {
		mu.Lock()
		defer mu.Unlock()
		intentCalls++
		if textBody == "" {
			t.Fatalf("expected textBody to be set")
		}
		session.Logger = slog.Default()
	}

	payload := map[string]interface{}{
		"entry": []interface{}{
			map[string]interface{}{
				"changes": []interface{}{
					map[string]interface{}{
						"value": map[string]interface{}{
							"messages": []interface{}{
								map[string]interface{}{"from": "111", "id": "m1", "type": "text", "text": map[string]interface{}{"body": "first"}},
								map[string]interface{}{"from": "111", "id": "m2", "type": "text", "text": map[string]interface{}{"body": "second"}},
							},
						},
					},
				},
			},
		},
	}

	processWhatsAppPayload(payload)

	var dedupCount int64
	if err := database.DB.Model(&models.ProcessedWebhook{}).Count(&dedupCount).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if dedupCount != 2 {
		t.Fatalf("expected 2 dedup records, got %d", dedupCount)
	}
	if len(client.markReadIDs) != 2 {
		t.Fatalf("expected 2 read receipts, got %d", len(client.markReadIDs))
	}
	if intentCalls != 2 {
		t.Fatalf("expected 2 intent calls, got %d", intentCalls)
	}
}

func TestProcessWhatsAppPayload_DuplicateMessageIsSkipped(t *testing.T) {
	setupControllerTestDB(t)

	oldClient := newWhatsAppClient
	oldIntent := processWhatsAppIntent
	defer func() {
		newWhatsAppClient = oldClient
		processWhatsAppIntent = oldIntent
	}()

	newWhatsAppClient = func() whatsapp.WhatsAppClient { return &testWhatsAppClient{} }
	intentCalls := 0
	processWhatsAppIntent = func(session *whatsapp.Session, textBody string, imageBase64 string) {
		intentCalls++
	}

	payload := map[string]interface{}{
		"entry": []interface{}{
			map[string]interface{}{
				"changes": []interface{}{
					map[string]interface{}{
						"value": map[string]interface{}{
							"messages": []interface{}{
								map[string]interface{}{"from": "222", "id": "dup-msg", "type": "text", "text": map[string]interface{}{"body": "hello"}},
								map[string]interface{}{"from": "222", "id": "dup-msg", "type": "text", "text": map[string]interface{}{"body": "hello"}},
							},
						},
					},
				},
			},
		},
	}

	processWhatsAppPayload(payload)

	if intentCalls != 1 {
		t.Fatalf("expected duplicate to be skipped and only one intent call, got %d", intentCalls)
	}
}

