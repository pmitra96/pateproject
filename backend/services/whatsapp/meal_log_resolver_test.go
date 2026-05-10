package whatsapp

import (
	"testing"
	"time"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupWhatsAppTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	database.DB = db
	database.DB.Exec("PRAGMA foreign_keys = OFF")
	if err := database.DB.AutoMigrate(
		&models.User{},
		&models.UserIdentity{},
		&models.MealLog{},
		&models.UserPreferences{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

func TestRecentDuplicateMeal_FindsMatchWithinWindow(t *testing.T) {
	setupWhatsAppTestDB(t)

	now := time.Now()
	meal := models.MealLog{
		UserID:      1,
		Name:        "Papaya",
		MealType:    "Breakfast",
		Ingredients: "100g papaya",
		LoggedAt:    now.Add(-5 * time.Minute),
	}
	if err := database.DB.Create(&meal).Error; err != nil {
		t.Fatalf("create meal: %v", err)
	}

	found, err := recentDuplicateMeal(1, "Breakfast", "papaya", "100g   papaya", now)
	if err != nil {
		t.Fatalf("expected duplicate match, got error: %v", err)
	}
	if found == nil || found.ID != meal.ID {
		t.Fatalf("expected meal id %d, got %+v", meal.ID, found)
	}
}

func TestSelectMealForCorrection_AmbiguousTarget(t *testing.T) {
	candidates := []models.MealLog{
		{ID: 1, Name: "Egg White Omelette", MealType: "Breakfast", LoggedAt: time.Now().Add(-10 * time.Minute)},
		{ID: 2, Name: "Egg White Omelette", MealType: "Breakfast", LoggedAt: time.Now().Add(-5 * time.Minute)},
	}

	_, reason, ok := selectMealForCorrection(candidates, "egg white omelette")
	if ok {
		t.Fatalf("expected ambiguity for duplicate names")
	}
	if reason != "ambiguous_target" {
		t.Fatalf("expected ambiguous_target, got %q", reason)
	}
}
