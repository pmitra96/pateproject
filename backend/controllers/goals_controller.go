package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/logger"
	"github.com/pmitra96/pateproject/models"
)

type CreateGoalRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	TargetDate  string `json:"target_date,omitempty"`
}

func GetGoals(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var goals []models.Goal
	if err := database.DB.Where("user_id = ? AND is_active = ?", userID, true).Find(&goals).Error; err != nil {
		logger.Error("Failed to fetch goals", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch goals"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(goals)
}

func CreateGoal(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateGoalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Title == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Title is required"})
		return
	}

	goal := models.Goal{
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		IsActive:    true,
	}

	if err := database.DB.Create(&goal).Error; err != nil {
		logger.Error("Failed to create goal", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create goal"})
		return
	}

	logger.Info("Goal created", "user_id", userID, "goal_id", goal.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(goal)
}

func DeleteGoal(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	goalIDStr := chi.URLParam(r, "goal_id")
	goalID, err := strconv.ParseUint(goalIDStr, 10, 32)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid goal ID"})
		return
	}

	result := database.DB.Where("id = ? AND user_id = ?", goalID, userID).Delete(&models.Goal{})
	if result.Error != nil {
		logger.Error("Failed to delete goal", "error", result.Error)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to delete goal"})
		return
	}

	if result.RowsAffected == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Goal not found"})
		return
	}

	logger.Info("Goal deleted", "user_id", userID, "goal_id", goalID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Goal deleted"})
}
