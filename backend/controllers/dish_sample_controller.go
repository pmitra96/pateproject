package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/logger"
	"github.com/pmitra96/pateproject/models"
)

type DishSampleRequest struct {
	Cuisine                  string   `json:"cuisine"`
	Region                   string   `json:"region"`
	Dish                     string   `json:"dish"`
	Details                  string   `json:"details"`
	Ingredients              []string `json:"ingredients"`
	Process                  []string `json:"process"`
	CalorificValuePerServing string   `json:"calorific_value_per_serving"`
	Benefits                 []string `json:"benefits"`
}

type DishSampleResponse struct {
	ID                       uint     `json:"id"`
	Cuisine                  string   `json:"cuisine"`
	Region                   string   `json:"region"`
	Dish                     string   `json:"dish"`
	Details                  string   `json:"details"`
	Ingredients              []string `json:"ingredients"`
	Process                  []string `json:"process"`
	CalorificValuePerServing string   `json:"calorific_value_per_serving"`
	Benefits                 []string `json:"benefits"`
}

func dishToResponse(d models.DishSample) DishSampleResponse {
	var ingredients, process, benefits []string
	json.Unmarshal([]byte(d.Ingredients), &ingredients)
	json.Unmarshal([]byte(d.Process), &process)
	json.Unmarshal([]byte(d.Benefits), &benefits)

	return DishSampleResponse{
		ID:                       d.ID,
		Cuisine:                  d.Cuisine,
		Region:                   d.Region,
		Dish:                     d.Dish,
		Details:                  d.Details,
		Ingredients:              ingredients,
		Process:                  process,
		CalorificValuePerServing: d.CalorificValuePerServing,
		Benefits:                 benefits,
	}
}

// GetDishSamples fetches dish samples, optionally filtered by cuisine or region
func GetDishSamples(w http.ResponseWriter, r *http.Request) {
	logger.Info("Received get dish samples request")

	cuisine := r.URL.Query().Get("cuisine")
	region := r.URL.Query().Get("region")

	query := database.DB.Model(&models.DishSample{})
	if cuisine != "" {
		query = query.Where("cuisine ILIKE ?", "%"+cuisine+"%")
	}
	if region != "" {
		query = query.Where("region ILIKE ?", "%"+region+"%")
	}

	var dishes []models.DishSample
	if err := query.Find(&dishes).Error; err != nil {
		logger.Error("Failed to fetch dish samples", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch dish samples"})
		return
	}

	response := make([]DishSampleResponse, len(dishes))
	for i, d := range dishes {
		response[i] = dishToResponse(d)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateDishSample adds a new dish sample
func CreateDishSample(w http.ResponseWriter, r *http.Request) {
	logger.Info("Received create dish sample request")

	var req DishSampleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Dish == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Dish name is required"})
		return
	}

	ingredientsJSON, _ := json.Marshal(req.Ingredients)
	processJSON, _ := json.Marshal(req.Process)
	benefitsJSON, _ := json.Marshal(req.Benefits)

	dish := models.DishSample{
		Cuisine:                  req.Cuisine,
		Region:                   req.Region,
		Dish:                     req.Dish,
		Details:                  req.Details,
		Ingredients:              string(ingredientsJSON),
		Process:                  string(processJSON),
		CalorificValuePerServing: req.CalorificValuePerServing,
		Benefits:                 string(benefitsJSON),
	}

	if err := database.DB.Create(&dish).Error; err != nil {
		logger.Error("Failed to create dish sample", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create dish sample"})
		return
	}

	logger.Info("Dish sample created", "dish_id", dish.ID, "dish", dish.Dish)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(dishToResponse(dish))
}

// BulkCreateDishSamples adds multiple dish samples at once
func BulkCreateDishSamples(w http.ResponseWriter, r *http.Request) {
	logger.Info("Received bulk create dish samples request")

	var requests []DishSampleRequest
	if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	var dishes []models.DishSample
	for _, req := range requests {
		if req.Dish == "" {
			continue
		}
		ingredientsJSON, _ := json.Marshal(req.Ingredients)
		processJSON, _ := json.Marshal(req.Process)
		benefitsJSON, _ := json.Marshal(req.Benefits)

		dishes = append(dishes, models.DishSample{
			Cuisine:                  req.Cuisine,
			Region:                   req.Region,
			Dish:                     req.Dish,
			Details:                  req.Details,
			Ingredients:              string(ingredientsJSON),
			Process:                  string(processJSON),
			CalorificValuePerServing: req.CalorificValuePerServing,
			Benefits:                 string(benefitsJSON),
		})
	}

	if err := database.DB.Create(&dishes).Error; err != nil {
		logger.Error("Failed to bulk create dish samples", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create dish samples"})
		return
	}

	logger.Info("Bulk dish samples created", "count", len(dishes))

	response := make([]DishSampleResponse, len(dishes))
	for i, d := range dishes {
		response[i] = dishToResponse(d)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// DeleteDishSample removes a dish sample
func DeleteDishSample(w http.ResponseWriter, r *http.Request) {
	dishID := chi.URLParam(r, "dish_id")

	if err := database.DB.Delete(&models.DishSample{}, dishID).Error; err != nil {
		logger.Error("Failed to delete dish sample", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to delete dish sample"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
