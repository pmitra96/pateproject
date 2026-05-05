package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/middleware"
	"github.com/pmitra96/pateproject/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() {
	// Use a file-based SQLite to avoid losing schema between connections
	db, _ := gorm.Open(sqlite.Open("test_pantry.db"), &gorm.Config{})
	database.DB = db
	database.DB.Exec("PRAGMA foreign_keys = OFF") // Simplify for tests
	database.DB.AutoMigrate(
		&models.User{},
		&models.UserIdentity{},
		&models.Ingredient{},
		&models.Brand{},
		&models.Item{},
		&models.PantryItem{},
		&models.Goal{},
		&models.GoalMacroProfile{},
	)
}

func TestPantryCRUD(t *testing.T) {
	setupTestDB()

	// 1. Create a Test User
	user := models.User{Name: "Test User", Email: "test@example.com"}
	database.DB.Create(&user)
	userIDStr := fmt.Sprintf("%d", user.ID)

	// Helper to add user context
	withUserContext := func(req *http.Request) *http.Request {
		ctx := context.WithValue(req.Context(), middleware.UserContextKey, userIDStr)
		return req.WithContext(ctx)
	}

	// --- STEP 1: Add Pantry Item ---
	t.Run("AddPantryItem", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":     "Rice",
			"quantity": 5.0,
			"unit":     "kg",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/pantry", bytes.NewBuffer(body))
		req = withUserContext(req)
		
		rr := httptest.NewRecorder()
		AddPantryItem(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		
		var resp models.PantryItem
		json.Unmarshal(rr.Body.Bytes(), &resp)
		assert.NotNil(t, resp.ManualQuantity)
		assert.Equal(t, 5.0, *resp.ManualQuantity)
	})

	// --- STEP 2: Get Pantry ---
	t.Run("GetPantry", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/pantry", nil)
		req = withUserContext(req)
		
		rr := httptest.NewRecorder()
		GetPantry(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		
		var items []map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &items)
		assert.GreaterOrEqual(t, len(items), 1)
	})

	// --- STEP 3: Update Pantry Item ---
	t.Run("UpdatePantryItem", func(t *testing.T) {
		var pi models.PantryItem
		database.DB.First(&pi)

		payload := map[string]interface{}{
			"manual_quantity": 10.5,
		}
		body, _ := json.Marshal(payload)
		
		// Setup router for URL param
		r := chi.NewRouter()
		r.Put("/pantry/{item_id}", UpdatePantryItem)
		
		req := httptest.NewRequest("PUT", fmt.Sprintf("/pantry/%d", pi.ID), bytes.NewBuffer(body))
		req = withUserContext(req)
		
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		
		var updatedPi models.PantryItem
		database.DB.First(&updatedPi, pi.ID)
		assert.Equal(t, 10.5, *updatedPi.ManualQuantity)
	})

	// --- STEP 4: Delete Pantry Item ---
	t.Run("DeletePantryItem", func(t *testing.T) {
		var pi models.PantryItem
		database.DB.First(&pi)

		r := chi.NewRouter()
		r.Delete("/pantry/{item_id}", DeletePantryItem)
		
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/pantry/%d", pi.ItemID), nil)
		req = withUserContext(req)
		
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
		
		var count int64
		database.DB.Model(&models.PantryItem{}).Where("id = ?", pi.ID).Count(&count)
		assert.Equal(t, int64(0), count)
	})
}
