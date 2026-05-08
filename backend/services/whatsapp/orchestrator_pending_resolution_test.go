package whatsapp

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/llm"
	"github.com/pmitra96/pateproject/models"
)

func setupWhatsAppOrchestratorTestDB(t *testing.T) {
	t.Helper()
	setupWhatsAppTestDB(t)
	if err := database.DB.AutoMigrate(
		&models.ConversationState{},
		&models.Conversation{},
		&models.LLMUsageLog{},
		&models.Goal{},
		&models.GoalMacroProfile{},
		&models.RemainingDayState{},
	); err != nil {
		t.Fatalf("migrate orchestrator tables: %v", err)
	}
}

func newTestSession(userID uint, waID string) (*Session, *MockClient) {
	mock := &MockClient{}
	return &Session{
		User: &models.User{
			ID: userID,
			Identities: []models.UserIdentity{
				{ExternalID: waID},
			},
		},
		Logger: slog.Default(),
		Client: mock,
	}, mock
}

func TestTryResolvePendingMealSelection_UsesStoredUpdatePayload(t *testing.T) {
	setupWhatsAppOrchestratorTestDB(t)

	userID := uint(77)
	meal := models.MealLog{
		UserID:      userID,
		Name:        "Curd Rice",
		MealType:    "Breakfast",
		Ingredients: "100gm curd rice",
		LoggedAt:    time.Now(),
	}
	if err := database.DB.Create(&meal).Error; err != nil {
		t.Fatalf("create meal: %v", err)
	}

	st := getConversationState(userID)
	setPendingMealSelection(st, "update", []uint{meal.ID}, map[string]any{
		"pending_update_ingredients": "50gm of curd rice",
	})

	var captured map[string]interface{}
	registry := NewToolRegistry()
	registry.Register("modify_logged_meal", func(s *Session, args map[string]interface{}) (string, error) {
		captured = args
		return `{"ok":true,"action":"update","dish_name":"Curd Rice","meal_type":"Breakfast","calories":95,"protein":4,"carbs":12,"fat":2,"fiber":1}`, nil
	})
	o := &Orchestrator{Registry: registry}
	session, client := newTestSession(userID, "919999999999")

	handled := o.tryResolvePendingMealSelection(session, st, "0")
	if !handled {
		t.Fatalf("expected pending selection to be handled")
	}
	if captured["new_ingredients"] != "50gm of curd rice" {
		t.Fatalf("expected stored correction payload, got %#v", captured["new_ingredients"])
	}

	reply := client.LastMessage
	if strings.Contains(reply, "{") || strings.Contains(strings.ToLower(reply), "\"action\"") {
		t.Fatalf("reply should not leak JSON: %q", reply)
	}
	if !strings.Contains(reply, "Updated Curd Rice") {
		t.Fatalf("unexpected reply: %q", reply)
	}

	var conv models.Conversation
	if err := database.DB.Where("user_id = ?", userID).First(&conv).Error; err != nil {
		t.Fatalf("conversation missing: %v", err)
	}
	var history []llm.Message
	if err := json.Unmarshal([]byte(conv.Messages), &history); err != nil {
		t.Fatalf("invalid history json: %v", err)
	}
	if len(history) < 2 {
		t.Fatalf("expected full user+assistant turn in history, got %d messages", len(history))
	}
	last := history[len(history)-1]
	if last.Role != "assistant" || last.Content != reply {
		t.Fatalf("expected final assistant history to match reply, got role=%q content=%#v", last.Role, last.Content)
	}
	if strings.TrimSpace(reply) == "Updated." {
		t.Fatalf("placeholder reply should not be used")
	}

	var state models.ConversationState
	if err := database.DB.Where("user_id = ?", userID).First(&state).Error; err != nil {
		t.Fatalf("state missing: %v", err)
	}
	if state.LastTool != "modify_logged_meal" {
		t.Fatalf("expected last_tool modify_logged_meal, got %q", state.LastTool)
	}
	if state.LastIntent != "pending_meal_resolution" {
		t.Fatalf("expected last_intent pending_meal_resolution, got %q", state.LastIntent)
	}
	if !strings.Contains(state.LastToolResult, `"action":"update"`) {
		t.Fatalf("expected tool result to be persisted, got %q", state.LastToolResult)
	}
	if state.PendingMealAction != "" || state.PendingMealOptions != "" || state.PendingSlots != "" {
		t.Fatalf("pending state should be cleared after resolution: %+v", state)
	}
}

func TestTryResolvePendingMealSelection_MissingMetaPromptsAndKeepsPending(t *testing.T) {
	setupWhatsAppOrchestratorTestDB(t)

	userID := uint(78)
	meal := models.MealLog{
		UserID:      userID,
		Name:        "Curd Rice",
		MealType:    "Breakfast",
		Ingredients: "100gm curd rice",
		LoggedAt:    time.Now(),
	}
	if err := database.DB.Create(&meal).Error; err != nil {
		t.Fatalf("create meal: %v", err)
	}

	st := getConversationState(userID)
	setPendingMealSelection(st, "update", []uint{meal.ID}, nil)

	called := false
	registry := NewToolRegistry()
	registry.Register("modify_logged_meal", func(s *Session, args map[string]interface{}) (string, error) {
		called = true
		return `{"ok":true}`, nil
	})
	o := &Orchestrator{Registry: registry}
	session, client := newTestSession(userID, "918888888888")

	handled := o.tryResolvePendingMealSelection(session, st, "0")
	if !handled {
		t.Fatalf("expected pending flow to handle numeric choice with prompt")
	}
	if called {
		t.Fatalf("tool should not execute when pending metadata is missing")
	}
	if !strings.Contains(strings.ToLower(client.LastMessage), "corrected ingredients") {
		t.Fatalf("expected corrective prompt, got %q", client.LastMessage)
	}

	var state models.ConversationState
	if err := database.DB.Where("user_id = ?", userID).First(&state).Error; err != nil {
		t.Fatalf("state missing: %v", err)
	}
	if state.PendingMealAction != "update" || strings.TrimSpace(state.PendingMealOptions) == "" {
		t.Fatalf("pending state should remain until correction is provided: %+v", state)
	}
}

func TestTryResolvePendingMealSelection_InvalidChoicePromptsForNumberAndKeepsPending(t *testing.T) {
	setupWhatsAppOrchestratorTestDB(t)

	userID := uint(80)
	meal := models.MealLog{
		UserID:      userID,
		Name:        "Curd Rice",
		MealType:    "Breakfast",
		Ingredients: "100gm curd rice",
		LoggedAt:    time.Now(),
	}
	if err := database.DB.Create(&meal).Error; err != nil {
		t.Fatalf("create meal: %v", err)
	}

	st := getConversationState(userID)
	setPendingMealSelection(st, "delete", []uint{meal.ID}, nil)

	registry := NewToolRegistry()
	called := false
	registry.Register("modify_logged_meal", func(s *Session, args map[string]interface{}) (string, error) {
		called = true
		return `{"ok":true}`, nil
	})
	o := &Orchestrator{Registry: registry}
	session, client := newTestSession(userID, "917666666666")

	handled := o.tryResolvePendingMealSelection(session, st, "option one")
	if !handled {
		t.Fatalf("expected pending flow to handle invalid choice with explicit prompt")
	}
	if called {
		t.Fatalf("tool should not execute for invalid numeric selection")
	}
	if !strings.Contains(strings.ToLower(client.LastMessage), "reply with a number") {
		t.Fatalf("expected numeric-format prompt, got %q", client.LastMessage)
	}

	var state models.ConversationState
	if err := database.DB.Where("user_id = ?", userID).First(&state).Error; err != nil {
		t.Fatalf("state missing: %v", err)
	}
	if state.PendingMealAction != "delete" || strings.TrimSpace(state.PendingMealOptions) == "" {
		t.Fatalf("pending state should remain after invalid choice: %+v", state)
	}
}

func TestProcessMessage_DeterministicRouteFallbackNeverLeaksToolJSON(t *testing.T) {
	setupWhatsAppOrchestratorTestDB(t)

	userID := uint(79)
	session, client := newTestSession(userID, "917777777777")

	registry := NewToolRegistry()
	registry.Register("modify_logged_meal", func(s *Session, args map[string]interface{}) (string, error) {
		return `{"ok":false}`, nil
	})
	o := &Orchestrator{Registry: registry}

	o.ProcessMessage(session, "delete curd from breakfast", "")

	reply := strings.TrimSpace(client.LastMessage)
	if reply == "" {
		t.Fatalf("expected fallback reply")
	}
	if strings.Contains(reply, "{") || strings.Contains(strings.ToLower(reply), "\"ok\"") {
		t.Fatalf("fallback should not leak raw tool json: %q", reply)
	}
}

func TestUserLocationForDisplay_DefaultsToAsiaKolkata(t *testing.T) {
	setupWhatsAppOrchestratorTestDB(t)

	loc := userLocationForDisplay(999999)
	_, offset := time.Now().In(loc).Zone()
	if offset != (5*60*60 + 30*60) {
		t.Fatalf("expected IST offset, got %d from %q", offset, loc.String())
	}
}
