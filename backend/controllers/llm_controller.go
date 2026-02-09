package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/llm"
	"github.com/pmitra96/pateproject/logger"
	"github.com/pmitra96/pateproject/models"
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

	userID, err := getUserID(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Unauthorized"})
		return
	}

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

	// Fetch user preferences
	var userPrefs models.UserPreferences
	var preferencesInfo *llm.UserPreferencesInfo
	if err := database.DB.Where("user_id = ?", userID).First(&userPrefs).Error; err == nil {
		var cuisines []string
		json.Unmarshal([]byte(userPrefs.PreferredCuisines), &cuisines)
		preferencesInfo = &llm.UserPreferencesInfo{
			Country:           userPrefs.Country,
			State:             userPrefs.State,
			City:              userPrefs.City,
			PreferredCuisines: cuisines,
		}
	}

	// Fetch dish samples based on preferred cuisines
	var dishSamples []llm.DishSampleInfo
	if preferencesInfo != nil && len(preferencesInfo.PreferredCuisines) > 0 {
		var dbDishes []models.DishSample
		query := database.DB.Model(&models.DishSample{})
		for i, cuisine := range preferencesInfo.PreferredCuisines {
			if i == 0 {
				query = query.Where("cuisine ILIKE ?", "%"+cuisine+"%")
			} else {
				query = query.Or("cuisine ILIKE ?", "%"+cuisine+"%")
			}
		}
		query.Limit(8).Find(&dbDishes)

		for _, d := range dbDishes {
			var ingredients []string
			json.Unmarshal([]byte(d.Ingredients), &ingredients)
			dishSamples = append(dishSamples, llm.DishSampleInfo{
				Dish:        d.Dish,
				Cuisine:     d.Cuisine,
				Details:     d.Details,
				Ingredients: ingredients,
				Calories:    d.CalorificValuePerServing,
			})
		}
	}

	client := llm.NewClient()
	suggestions, err := client.SuggestMealsPersonalized(req.Inventory, req.Goals, req.TimeOfDay, preferencesInfo, dishSamples)

	if err != nil {
		logger.Error("Failed to generate personalized meal suggestions", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}

	logger.Info("Personalized meal suggestions generated", "items_count", len(req.Inventory), "goals_count", len(req.Goals), "time", req.TimeOfDay, "dish_samples", len(dishSamples))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MealSuggestionResponse{
		Suggestions: suggestions,
	})
}

type ChatRequest struct {
	Message   string            `json:"message"`
	History   []llm.ChatMessage `json:"history"`
	Inventory []llm.InventoryItem `json:"inventory"`
	Goals     []llm.GoalInfo      `json:"goals"`
}

type ChatResponse struct {
	Response string `json:"response"`
}

func ChatBot(w http.ResponseWriter, r *http.Request) {
	logger.Info("Received chatbot request")

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Invalid request body"})
		return
	}

	if req.Message == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Message is required"})
		return
	}

	client := llm.NewClient()
	response, err := client.ChatWithContext(req.Message, req.History, req.Inventory, req.Goals)

	if err != nil {
		logger.Error("Failed to get chatbot response", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}

	logger.Info("Chatbot response generated", "message_length", len(req.Message))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ChatResponse{
		Response: response,
	})
}
