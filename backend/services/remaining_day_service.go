package services

import (
	"time"

	"github.com/pmitra96/pateproject/database"
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
	var consumedCalories, consumedProtein, consumedFat, consumedCarbs, consumedFiber float64
	for _, meal := range meals {
		consumedCalories += meal.Calories
		consumedProtein += meal.Protein
		consumedFat += meal.Fat
		consumedCarbs += meal.Carbs
		consumedFiber += meal.Fiber
	}

	// 5. Calculate remaining
	remainingCalories := float64(profile.DailyCalorieTarget) - consumedCalories
	remainingProtein := profile.DailyProteinTarget - consumedProtein
	remainingFat := profile.DailyFatTarget - consumedFat
	remainingCarbs := profile.DailyCarbsTarget - consumedCarbs
	remainingFiber := profile.DailyFiberTarget - consumedFiber

	// 6. Determine Control Mode
	controlMode := "NORMAL"
	damageFloor := float64(profile.DamageControlFloorCalories)
	calTarget := float64(profile.DailyCalorieTarget)

	if remainingCalories < damageFloor {
		controlMode = "DAMAGE_CONTROL"
	} else if remainingCalories < (calTarget * 0.20) {
		controlMode = "TIGHT"
	}

	// Check sticky DAMAGE_CONTROL or Log transitions
	var existingState models.RemainingDayState
	if err := database.DB.Where("user_id = ? AND date = ?", userID, startOfDay).First(&existingState).Error; err == nil {
		if existingState.ControlMode != controlMode {
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
	now := time.Now()
	mealsRemaining := 1
	if now.Hour() < 11 {
		mealsRemaining = 3
	} else if now.Hour() < 16 {
		mealsRemaining = 2
	}

	// Save/Update State
	state := models.RemainingDayState{
		UserID:            userID,
		Date:              startOfDay,
		RemainingCalories: remainingCalories,
		RemainingProtein:  remainingProtein,
		RemainingFat:      remainingFat,
		RemainingCarbs:    remainingCarbs,
		RemainingFiber:    remainingFiber,
		TargetCalories:    float64(profile.DailyCalorieTarget),
		TargetProtein:     profile.DailyProteinTarget,
		TargetFat:         profile.DailyFatTarget,
		TargetCarbs:       profile.DailyCarbsTarget,
		TargetFiber:       profile.DailyFiberTarget,
		MealsRemaining:    mealsRemaining,
		ControlMode:       controlMode,
		LastComputedAt:    time.Now(),
	}

	// Upsert State
	var existing models.RemainingDayState
	err := database.DB.Where("user_id = ? AND date = ?", userID, startOfDay).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		if err := database.DB.Create(&state).Error; err != nil {
			return nil, err
		}
	} else if err == nil {
		state.ID = existing.ID
		state.CreatedAt = existing.CreatedAt
		if err := database.DB.Save(&state).Error; err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	return &state, nil
}
