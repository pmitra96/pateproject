package services

import (
	"fmt"

	"github.com/pmitra96/pateproject/models"
)

// FoodEstimate represents the nutritional estimate of a food item
type FoodEstimate struct {
	Name     string  `json:"name,omitempty"`
	Calories float64 `json:"calories"`
	Protein  float64 `json:"protein"`
	Fat      float64 `json:"fat"`
	Carbs    float64 `json:"carbs"`
}

// PermissionResult represents the authoritative decision
type PermissionResult struct {
	Status string `json:"status"` // ALLOW, ALLOW_WITH_CONSTRAINT, BLOCK
	Reason string `json:"reason,omitempty"`
}

// Permission Status Constants
const (
	StatusAllow               = "ALLOW"
	StatusAllowWithConstraint = "ALLOW_WITH_CONSTRAINT"
	StatusBlock               = "BLOCK"
)

// CheckFoodPermission evaluates if a food is allowed based on the remaining day state
// This function implements the Rules 0-5 from the spec "Can I Eat This?"
func CheckFoodPermission(state *models.RemainingDayState, food FoodEstimate) PermissionResult {
	if state == nil {
		// Fallback if no state exists (shouldn't happen if properly gated, but safety first)
		return PermissionResult{Status: StatusAllow, Reason: "No limits set"}
	}

	// Rule 0 — Damage Control Override
	if state.ControlMode == "DAMAGE_CONTROL" {
		// ALLOW only if:
		// - food is protein-dominant (Protein > Fat + Carbs OR Protein is significant portion)
		// Spec says "food is protein-dominant". Let's define this as Protein >= (Fat + Carbs)
		// AND food.calories <= remainingCalories (Wait, in damage control remaining is likely ~0 or negative.
		// The spec says "food.calories <= remainingCalories".
		// IF remainingCalories is negative, this condition is impossible unless food has negative calories.
		// However, "Damage Control" might mean we are just below the floor, not necessarily negative.
		// But in many damage control scenarios, remaining is very low.
		// Spec text: "food.calories <= remainingCalories". If remaining is 50, and food is 100 -> Block.
		//
		// Spec also says "food.fat <= minimalFatThreshold". Let's pick a threshold, say 5g for a snack.

		isProteinDominant := food.Protein >= (food.Fat + food.Carbs)
		fitsCalories := food.Calories <= state.RemainingCalories
		lowFat := food.Fat <= 5.0 // Hardcoded minimal threshold for now

		if isProteinDominant && fitsCalories && lowFat {
			return PermissionResult{Status: StatusAllow, Reason: "Protein prioritized in damage control"}
		}

		return PermissionResult{Status: StatusBlock, Reason: "Day is in damage control."}
	}

	// Rule 1 — Hard Calorie Block
	if food.Calories > state.RemainingCalories {
		return PermissionResult{Status: StatusBlock, Reason: "Exceeds remaining calories today."}
	}

	// Rule 2 — Hard Fat Block
	if food.Fat > state.RemainingFat {
		return PermissionResult{Status: StatusBlock, Reason: "Exceeds remaining fat limit today."}
	}

	// Rule 3 — Conditional Allow (Tight Margins)
	// If food fits remaining calories and fat (already checked above implicitly by passing Rule 1 & 2)
	// AND (food.calories > 50% of remaining OR food.fat > 60% of remaining)
	fitsCalories := food.Calories <= state.RemainingCalories
	fitsFat := food.Fat <= state.RemainingFat

	isTightCalories := state.RemainingCalories > 0 && food.Calories > (0.5*state.RemainingCalories)
	isTightFat := state.RemainingFat > 0 && food.Fat > (0.6*state.RemainingFat)

	if fitsCalories && fitsFat && (isTightCalories || isTightFat) {
		reason := "Allowed, but be careful."
		if isTightFat {
			pct := (food.Fat / state.RemainingFat) * 100
			reason = fmt.Sprintf("Allowed, but this uses %.0f%% of your remaining fat budget.", pct)
		} else if isTightCalories {
			pct := (food.Calories / state.RemainingCalories) * 100
			reason = fmt.Sprintf("Allowed, but this uses %.0f%% of your remaining calories.", pct)
		}
		return PermissionResult{Status: StatusAllowWithConstraint, Reason: reason}
	}

	// Rule 4 — Protein Exception
	// If protein target not yet met AND food is protein-dominant
	// AND calories and fat fit remaining limits (checked by 1 & 2)
	// Wait, if 1 or 2 triggered, we would have returned BLOCK.
	// So if we are here, it fits limits.
	// The spec says "If protein target not yet met".
	// Implementation: remainingProtein > 0

	// Re-evaluating Rule 4 placement vs invariant "Only one rule fires".
	// Logic is top-down.
	// If Rule 1 or 2 fired, we returned Block.
	// If Rule 3 fired, we returned AllowWithConstraint.
	// So we are here only if it fits limits comfortably (not tight margins).
	// Spec says: "If protein target not yet met ... -> ALLOW (Reason: Protein prioritized)".
	// This seems to be an "Info" Allow rather than a "Conditional" Allow.
	// It just changes the reason string.

	isProteinDominant := food.Protein >= (food.Fat + food.Carbs)
	proteinTargetNotMet := state.RemainingProtein > 0

	if proteinTargetNotMet && isProteinDominant {
		return PermissionResult{Status: StatusAllow, Reason: "Protein prioritized today."}
	}

	// Rule 5 — Default Allow
	return PermissionResult{Status: StatusAllow}
}
