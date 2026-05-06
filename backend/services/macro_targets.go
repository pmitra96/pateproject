package services

import "math"

// DefaultFiberTargetFromCalories returns a simple fiber target derived from daily calories.
func DefaultFiberTargetFromCalories(calories int) float64 {
	if calories <= 0 {
		return 30
	}
	return math.Round(float64(calories) * 14.0 / 1000.0)
}
