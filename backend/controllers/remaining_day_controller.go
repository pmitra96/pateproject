package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/logger"
	"github.com/pmitra96/pateproject/models"
	"github.com/pmitra96/pateproject/services"
	"gorm.io/gorm"
)

// (Function moved to services/remaining_day_service.go)

// HTTP Handlers

// GetRemainingDayState returns the current state
func GetRemainingDayState(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Compute fresh state
	state, err := services.ComputeRemainingDayState(userID, time.Now())
	if err != nil {
		logger.Error("Failed to compute state", "error", err)
		http.Error(w, "Failed to compute state", http.StatusInternalServerError)
		return
	}

	if state == nil {
		// No goal or targets set
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "no_targets",
			"message": "Please set a goal and macro targets to enable this feature.",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

// SetGoalMacroTargets sets the macro targets for a goal
func SetGoalMacroTargets(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	goalIDStr := chi.URLParam(r, "goal_id")
	goalID, _ := strconv.ParseUint(goalIDStr, 10, 32)

	// Verify goal belongs to user
	var goal models.Goal
	if err := database.DB.Where("id = ? AND user_id = ?", goalID, userID).First(&goal).Error; err != nil {
		http.Error(w, "Goal not found", http.StatusNotFound)
		return
	}

	var req struct {
		DailyCalorieTarget         int      `json:"daily_calorie_target"`
		DailyProteinTarget         float64  `json:"daily_protein_target"`
		DailyFatTarget             float64  `json:"daily_fat_target"`
		DailyCarbsTarget           float64  `json:"daily_carbs_target"`
		MacroPriority              []string `json:"macro_priority"`
		DamageControlFloorCalories int      `json:"damage_control_floor_calories"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	priorityJSON, _ := json.Marshal(req.MacroPriority)

	profile := models.GoalMacroProfile{
		GoalID:                     uint(goalID),
		DailyCalorieTarget:         req.DailyCalorieTarget,
		DailyProteinTarget:         req.DailyProteinTarget,
		DailyFatTarget:             req.DailyFatTarget,
		DailyCarbsTarget:           req.DailyCarbsTarget,
		MacroPriorityOrder:         string(priorityJSON),
		DamageControlFloorCalories: req.DamageControlFloorCalories,
	}

	// Upsert Profile
	var existingProfile models.GoalMacroProfile
	err = database.DB.Where("goal_id = ?", goalID).First(&existingProfile).Error
	if err == gorm.ErrRecordNotFound {
		if err := database.DB.Create(&profile).Error; err != nil {
			http.Error(w, "Failed to save targets", http.StatusInternalServerError)
			return
		}
	} else if err == nil {
		profile.ID = existingProfile.ID
		profile.CreatedAt = existingProfile.CreatedAt // Preserve creation time
		if err := database.DB.Save(&profile).Error; err != nil {
			http.Error(w, "Failed to save targets", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Failed to query targets", http.StatusInternalServerError)
		return
	}
	// Touch the goal to make it the most recently updated (active) one
	database.DB.Model(&models.Goal{ID: uint(goalID)}).Updates(map[string]interface{}{
		"updated_at": time.Now(),
		"is_active":  true,
	})

	// Trigger re-computation
	services.ComputeRemainingDayState(userID, time.Now())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success"})
}

// ValidateMeal checks if a meal is allowed
func ValidateMeal(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse query params: calories, protein, etc.
	calories, _ := strconv.ParseFloat(r.URL.Query().Get("calories"), 64)
	// ... parse others ...

	state, _ := services.ComputeRemainingDayState(userID, time.Now())
	if state == nil {
		// No restrictions if no state
		json.NewEncoder(w).Encode(map[string]interface{}{"allowed": true})
		return
	}

	allowed := true
	reason := ""

	if calories > state.RemainingCalories+50 { // tolerance
		allowed = false
		reason = "Exceeds remaining calories"
	}

	// Check damage control
	if state.ControlMode == "DAMAGE_CONTROL" && calories > 200 {
		allowed = false
		reason = "In Damage Control: Only small snacks allowed"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"allowed":       allowed,
		"reason":        reason,
		"current_state": state,
	})
}

// PermissionCheckResponse includes the decision and the estimated food data
type PermissionCheckResponse struct {
	services.PermissionResult
	Food services.FoodEstimate `json:"food"`
}

// CheckFoodPermissionHandler handles the API request to check food permission
func CheckFoodPermissionHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Query    string  `json:"query"` // User text input
		Calories float64 `json:"calories"`
		Protein  float64 `json:"protein"`
		Fat      float64 `json:"fat"`
		Carbs    float64 `json:"carbs"`
		Name     string  `json:"name"` // Fallback name
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 1. Get current state
	state, err := services.ComputeRemainingDayState(userID, time.Now())
	if err != nil {
		logger.Error("Failed to compute state for permission check", "error", err)
		http.Error(w, "Failed to compute state", http.StatusInternalServerError)
		return
	}

	// 2. Determine FoodEstimate
	var food services.FoodEstimate

	if req.Query != "" {
		// Use automatic estimation
		ns := services.NewNutritionService()
		estimated, err := ns.EstimateNutritionFromQuery(userID, req.Query)
		if err != nil {
			logger.Error("Failed to estimate nutrition", "query", req.Query, "error", err)
			http.Error(w, "Failed to estimate nutrition for: "+req.Query, http.StatusInternalServerError)
			return
		}
		food = *estimated
	} else {
		// Manual input fallback to support old/direct usage
		food = services.FoodEstimate{
			Name:     req.Name,
			Calories: req.Calories,
			Protein:  req.Protein,
			Fat:      req.Fat,
			Carbs:    req.Carbs,
		}
	}

	// 3. Check permission
	result := services.CheckFoodPermission(state, food)

	// 4. Compute Simulated State
	// Clone current state to simulate impact
	simulatedState := *state
	simulatedState.RemainingCalories -= food.Calories
	simulatedState.RemainingProtein -= food.Protein
	simulatedState.RemainingFat -= food.Fat
	simulatedState.RemainingCarbs -= food.Carbs

	// Create a new struct for response to include SimulatedState
	type PermissionCheckResponseWithSimulation struct {
		PermissionResult services.PermissionResult `json:"permission_result"`
		Food             services.FoodEstimate     `json:"food"`
		CurrentState     models.RemainingDayState  `json:"current_state"`
		SimulatedState   models.RemainingDayState  `json:"simulated_state"`
	}

	// Fetch active profile to get damage floor for accurate simulation
	var goal models.Goal
	var profile models.GoalMacroProfile
	damageFloor := 0.0
	if err := database.DB.Where("user_id = ? AND is_active = ?", userID, true).Order("updated_at desc").First(&goal).Error; err == nil {
		if err := database.DB.Where("goal_id = ?", goal.ID).First(&profile).Error; err == nil {
			damageFloor = float64(profile.DamageControlFloorCalories)
		}
	}

	simCalTarget := float64(simulatedState.TargetCalories)
	if simulatedState.RemainingCalories < damageFloor {
		simulatedState.ControlMode = "DAMAGE_CONTROL"
	} else if simulatedState.RemainingCalories < (simCalTarget * 0.20) {
		simulatedState.ControlMode = "TIGHT"
	} else {
		simulatedState.ControlMode = "NORMAL"
	}

	// 5. Log the check
	logger.Info("Food permission check",
		"user_id", userID,
		"food", food.Name,
		"calories", food.Calories,
		"decision", result.Status,
		"reason", result.Reason,
		"control_mode", state.ControlMode,
	)

	// 6. Return full response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PermissionCheckResponseWithSimulation{
		PermissionResult: result,
		Food:             food,
		CurrentState:     *state,
		SimulatedState:   simulatedState,
	})
}
