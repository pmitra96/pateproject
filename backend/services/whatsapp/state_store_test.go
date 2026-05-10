package whatsapp

import (
	"testing"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupStateStoreDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	database.DB = db
	if err := database.DB.AutoMigrate(&models.ConversationState{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

func TestUpdateConversationStateAfterTool_PreservesPendingSelection(t *testing.T) {
	setupStateStoreDB(t)

	st := &models.ConversationState{
		UserID:             42,
		PendingMealAction:  "delete",
		PendingMealOptions: "[1,2]",
		PendingSlots:       `{"pending_update_ingredients":"50g curd"}`,
	}
	if err := database.DB.Create(st).Error; err != nil {
		t.Fatalf("create state: %v", err)
	}

	updateConversationStateAfterTool(st, "llm", "modify_logged_meal", `{"ok":false,"error":"ambiguous_target"}`)

	var got models.ConversationState
	if err := database.DB.Where("user_id = ?", st.UserID).First(&got).Error; err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if got.PendingMealAction != "delete" {
		t.Fatalf("pending action was cleared unexpectedly: %q", got.PendingMealAction)
	}
	if got.PendingMealOptions != "[1,2]" {
		t.Fatalf("pending options changed unexpectedly: %q", got.PendingMealOptions)
	}
	if got.PendingSlots == "" {
		t.Fatalf("pending metadata should be preserved")
	}
}

func TestUpdateConversationStateAfterReply_PreservesPendingSelection(t *testing.T) {
	setupStateStoreDB(t)

	st := &models.ConversationState{
		UserID:             43,
		PendingMealAction:  "update",
		PendingMealOptions: "[10,11]",
		PendingSlots:       `{"pending_update_ingredients":"30g whey protein"}`,
	}
	if err := database.DB.Create(st).Error; err != nil {
		t.Fatalf("create state: %v", err)
	}

	updateConversationStateAfterReply(st, "llm", "Please send a valid choice")

	var got models.ConversationState
	if err := database.DB.Where("user_id = ?", st.UserID).First(&got).Error; err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if got.PendingMealAction != "update" {
		t.Fatalf("pending action was cleared unexpectedly: %q", got.PendingMealAction)
	}
	if got.PendingMealOptions != "[10,11]" {
		t.Fatalf("pending options changed unexpectedly: %q", got.PendingMealOptions)
	}
	if got.PendingSlots == "" {
		t.Fatalf("pending metadata should be preserved")
	}
}
