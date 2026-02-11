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
	"gorm.io/gorm"
)

// ComputeRemainingDayState calculates the remaining nutritional budget for a user
func ComputeRemainingDayState(userID uint, date time.Time) (*models.RemainingDayState, error) {
	var goal models.Goal
	if err := database.DB.Where("user_id = ? AND is_active = ?", userID, true).Order("updated_at desc").First(&goal).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No active goal, so no state to compute
		}
		return nil, err
	}

	// 2. Get goal macro profile
	var profile models.GoalMacroProfile
	if err := database.DB.Where("goal_id = ?", goal.ID).First(&profile).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Goal exists but no targets set
		}
		return nil, err
	}

	// 3. Fetch all meals logged today
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	var meals []models.MealLog
	if err := database.DB.Where("user_id = ? AND logged_at >= ? AND logged_at < ?", userID, startOfDay, endOfDay).Find(&meals).Error; err != nil {
		return nil, err
	}

	// 4. Sum consumed macros
	var consumedCalories, consumedProtein, consumedFat, consumedCarbs float64
	for _, meal := range meals {
		consumedCalories += meal.Calories
		consumedProtein += meal.Protein
		consumedFat += meal.Fat
		consumedCarbs += meal.Carbs
	}

	// 5. Calculate remaining
	remainingCalories := float64(profile.DailyCalorieTarget) - consumedCalories
	remainingProtein := profile.DailyProteinTarget - consumedProtein
	remainingFat := profile.DailyFatTarget - consumedFat
	remainingCarbs := profile.DailyCarbsTarget - consumedCarbs

	// 6. Determine Control Mode
	controlMode := "NORMAL"
	damageFloor := float64(profile.DamageControlFloorCalories)
	calTarget := float64(profile.DailyCalorieTarget)

	// Logic:
	// If remaining < damage_floor OR < 0 (technically covered by damage_floor if floor > 0) -> DAMAGE_CONTROL
	// If remaining < 20% of target -> TIGHT
	// Else -> NORMAL

	if remainingCalories < damageFloor {
		controlMode = "DAMAGE_CONTROL"
	} else if remainingCalories < (calTarget * 0.20) {
		controlMode = "TIGHT"
	}

	// Check sticky DAMAGE_CONTROL: once in damage control, stay there until midnight?
	// The prompt said "STICKY until midnight".
	// Implementation: Check existing state for the day. If it was DAMAGE_CONTROL, keep it.
	var existingState models.RemainingDayState
	if err := database.DB.Where("user_id = ? AND date = ?", userID, startOfDay).First(&existingState).Error; err == nil {
		if existingState.ControlMode == "DAMAGE_CONTROL" {
			controlMode = "DAMAGE_CONTROL"
		}
		// Also check for audit log if mode changed (new != old)
		if existingState.ControlMode != controlMode {
			// Log transition
			transition := models.ControlModeTransition{
				UserID:                        userID,
				Date:                          startOfDay,
				FromMode:                      existingState.ControlMode,
				ToMode:                        controlMode,
				RemainingCaloriesAtTransition: remainingCalories,
				CreatedAt:                     time.Now(),
			}
			database.DB.Create(&transition)
		}
	}

	// 7. Calculate meals remaining
	// Simple logic based on time of day
	now := time.Now()
	mealsRemaining := 1
	if now.Hour() < 11 {
		mealsRemaining = 3
	} else if now.Hour() < 16 {
		mealsRemaining = 2
	} // else after 4pm -> 1

	// Save/Update State
	state := models.RemainingDayState{
		UserID:            userID,
		Date:              startOfDay,
		RemainingCalories: remainingCalories,
		RemainingProtein:  remainingProtein,
		RemainingFat:      remainingFat,
		RemainingCarbs:    remainingCarbs,
		MealsRemaining:    mealsRemaining,
		ControlMode:       controlMode,
		LastComputedAt:    time.Now(),
	}

	// Upsert
	if err := database.DB.Where("user_id = ? AND date = ?", userID, startOfDay).Assign(state).FirstOrCreate(&state).Error; err != nil {
		return nil, err
	}
	// Re-save to ensure updates if it existed
	database.DB.Save(&state)

	return &state, nil
}

// HTTP Handlers

// GetRemainingDayState returns the current state
func GetRemainingDayState(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Compute fresh state
	state, err := ComputeRemainingDayState(userID, time.Now())
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

	// Upsert
	if err := database.DB.Where("goal_id = ?", goalID).Assign(profile).FirstOrCreate(&profile).Error; err != nil {
		http.Error(w, "Failed to save targets", http.StatusInternalServerError)
		return
	}
	database.DB.Save(&profile)
	// Touch the goal to make it the most recently updated (active) one
	database.DB.Model(&models.Goal{ID: uint(goalID)}).Updates(map[string]interface{}{
		"updated_at": time.Now(),
		"is_active":  true,
	})

	// Trigger re-computation
	ComputeRemainingDayState(userID, time.Now())

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

	state, _ := ComputeRemainingDayState(userID, time.Now())
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
