package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/logger"
	"github.com/pmitra96/pateproject/models"
)

type UserPreferencesRequest struct {
	Country           string   `json:"country"`
	State             string   `json:"state"`
	City              string   `json:"city"`
	PreferredCuisines []string `json:"preferred_cuisines"`
}

type UserPreferencesResponse struct {
	ID                uint     `json:"id"`
	Country           string   `json:"country"`
	State             string   `json:"state"`
	City              string   `json:"city"`
	PreferredCuisines []string `json:"preferred_cuisines"`
}

// GetUserPreferences fetches user preferences
func GetUserPreferences(w http.ResponseWriter, r *http.Request) {
	logger.Info("Received get user preferences request")

	userID, err := getUserID(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	var prefs models.UserPreferences
	result := database.DB.Where("user_id = ?", userID).First(&prefs)

	if result.Error != nil {
		// Return empty preferences if not found
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(UserPreferencesResponse{
			PreferredCuisines: []string{},
		})
		return
	}

	// Parse cuisines from comma-separated string
	var cuisines []string
	if prefs.PreferredCuisines != "" {
		if err := json.Unmarshal([]byte(prefs.PreferredCuisines), &cuisines); err != nil {
			cuisines = []string{}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserPreferencesResponse{
		ID:                prefs.ID,
		Country:           prefs.Country,
		State:             prefs.State,
		City:              prefs.City,
		PreferredCuisines: cuisines,
	})
}

// UpdateUserPreferences creates or updates user preferences
func UpdateUserPreferences(w http.ResponseWriter, r *http.Request) {
	logger.Info("Received update user preferences request")

	userID, err := getUserID(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	var req UserPreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	// Serialize cuisines to JSON
	cuisinesJSON, _ := json.Marshal(req.PreferredCuisines)

	var prefs models.UserPreferences
	result := database.DB.Where("user_id = ?", userID).First(&prefs)

	if result.Error != nil {
		// Create new preferences
		prefs = models.UserPreferences{
			UserID:            userID,
			Country:           req.Country,
			State:             req.State,
			City:              req.City,
			PreferredCuisines: string(cuisinesJSON),
		}
		if err := database.DB.Create(&prefs).Error; err != nil {
			logger.Error("Failed to create user preferences", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save preferences"})
			return
		}
	} else {
		// Update existing preferences
		prefs.Country = req.Country
		prefs.State = req.State
		prefs.City = req.City
		prefs.PreferredCuisines = string(cuisinesJSON)
		if err := database.DB.Save(&prefs).Error; err != nil {
			logger.Error("Failed to update user preferences", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save preferences"})
			return
		}
	}

	logger.Info("User preferences saved", "user_id", userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserPreferencesResponse{
		ID:                prefs.ID,
		Country:           prefs.Country,
		State:             prefs.State,
		City:              prefs.City,
		PreferredCuisines: req.PreferredCuisines,
	})
}
