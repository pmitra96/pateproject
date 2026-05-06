package whatsapp

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/pmitra96/pateproject/models"
)

type weightLossGoal struct {
	Kg   float64
	Days int
	Mode string // lose or gain
}

func defaultFiberTargetFromCalories(calories int) float64 {
	if calories <= 0 {
		return 30
	}
	return math.Round(float64(calories) * 14.0 / 1000.0)
}

func parseWeightLossGoal(text string) *weightLossGoal {
	lower := strings.ToLower(text)
	re := regexp.MustCompile(`(lose|gain)\s+(\d+(?:\.\d+)?)\s*kg(?:\s+in\s+(\d+)\s*(day|days|week|weeks|month|months))?`)
	m := re.FindStringSubmatch(lower)
	if len(m) < 3 {
		return nil
	}
	mode := m[1]
	kg, _ := strconv.ParseFloat(m[2], 64)
	if kg <= 0 {
		return nil
	}
	days := 30
	if len(m) >= 5 && m[3] != "" {
		n, _ := strconv.Atoi(m[3])
		if n > 0 {
			unit := m[4]
			switch unit {
			case "day", "days":
				days = n
			case "week", "weeks":
				days = n * 7
			default:
				days = n * 30
			}
		}
	}
	return &weightLossGoal{Kg: kg, Days: days, Mode: mode}
}

func calculateGoalFromProfile(p models.UserPreferences, g weightLossGoal) (int, string, bool) {
	if p.Height <= 0 || p.Weight <= 0 || p.Age <= 0 || strings.TrimSpace(p.Gender) == "" {
		return 0, "To calculate this goal, I need your height, weight, age, gender, and activity level.", false
	}
	activity := strings.TrimSpace(strings.ToLower(p.ActivityLevel))
	if activity == "" {
		return 0, "To calculate this goal, please share your activity level: sedentary, light, moderate, active, or very_active.", false
	}

	factor := 1.2
	switch activity {
	case "light":
		factor = 1.375
	case "moderate":
		factor = 1.55
	case "active":
		factor = 1.725
	case "very_active":
		factor = 1.9
	}

	bmr := 10*p.Weight + 6.25*p.Height - 5*float64(p.Age)
	switch strings.ToLower(strings.TrimSpace(p.Gender)) {
	case "male":
		bmr += 5
	case "female":
		bmr -= 161
	}
	tdee := bmr * factor
	dailyDelta := (g.Kg * 7700.0) / float64(g.Days)
	if dailyDelta > 1000 {
		dailyDelta = 1000
	}
	if dailyDelta < 200 {
		dailyDelta = 200
	}

	target := int(math.Round(tdee - dailyDelta))
	if g.Mode == "gain" {
		target = int(math.Round(tdee + dailyDelta))
	}
	minCalories := 1300
	switch strings.ToLower(strings.TrimSpace(p.Gender)) {
	case "male":
		minCalories = 1500
	case "female":
		minCalories = 1200
	}
	if target < minCalories {
		target = minCalories
	}

	modeWord := "deficit"
	if g.Mode == "gain" {
		modeWord = "surplus"
	}
	explanation := fmt.Sprintf("Calculated from BMR %.0f and TDEE %.0f with a daily %s of %.0f kcal for %.1fkg in %d days.", bmr, tdee, modeWord, dailyDelta, g.Kg, g.Days)
	return target, explanation, true
}
