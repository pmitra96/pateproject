package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/pmitra96/pateproject/llm"
	"github.com/pmitra96/pateproject/logger"
)

type StoryRequest struct {
	Topic string `json:"topic"`
}

type StoryResponse struct {
	Story string `json:"story"`
	Topic string `json:"topic,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type MealSuggestionRequest struct {
	Inventory []llm.InventoryItem `json:"inventory"`
}

type MealSuggestionResponse struct {
	Suggestions string `json:"suggestions"`
}

type PersonalizedMealRequest struct {
	Inventory []llm.InventoryItem `json:"inventory"`
	Goals     []llm.GoalInfo      `json:"goals"`
	TimeOfDay string              `json:"time_of_day"`
}

func GenerateStory(w http.ResponseWriter, r *http.Request) {
	logger.Info("Received story generation request")

	var req StoryRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	client := llm.NewClient()
	story, err := client.GenerateStory(req.Topic)

	if err != nil {
		logger.Error("Failed to generate story", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}

	logger.Info("Story generated successfully", "topic", req.Topic)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StoryResponse{
		Story: story,
		Topic: req.Topic,
	})
}

func SuggestMeal(w http.ResponseWriter, r *http.Request) {
	logger.Info("Received meal suggestion request")

	var req MealSuggestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Invalid request body"})
		return
	}

	if len(req.Inventory) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "No inventory items provided"})
		return
	}

	client := llm.NewClient()
	suggestions, err := client.SuggestMeals(req.Inventory)

	if err != nil {
		logger.Error("Failed to generate meal suggestions", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}

	logger.Info("Meal suggestions generated successfully", "items_count", len(req.Inventory))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MealSuggestionResponse{
		Suggestions: suggestions,
	})
}

func SuggestMealPersonalized(w http.ResponseWriter, r *http.Request) {
	logger.Info("Received personalized meal suggestion request")

	var req PersonalizedMealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Invalid request body"})
		return
	}

	if len(req.Inventory) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "No inventory items provided"})
		return
	}

	client := llm.NewClient()
	suggestions, err := client.SuggestMealsPersonalized(req.Inventory, req.Goals, req.TimeOfDay)

	if err != nil {
		logger.Error("Failed to generate personalized meal suggestions", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}

	logger.Info("Personalized meal suggestions generated", "items_count", len(req.Inventory), "goals_count", len(req.Goals), "time", req.TimeOfDay)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MealSuggestionResponse{
		Suggestions: suggestions,
	})
}
