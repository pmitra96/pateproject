package whatsapp

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/models"
	"gorm.io/gorm"
)

const dedupeWindow = 20 * time.Minute
var reMealTextPunct = regexp.MustCompile(`[^\w\s]`)

func normalizeMealText(s string) string {
	n := strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer(
		"omlette", "omelette",
		"protien", "protein",
	)
	n = replacer.Replace(n)
	n = reMealTextPunct.ReplaceAllString(n, " ")
	return strings.Join(strings.Fields(n), " ")
}

func findMealsForDay(userID uint, mealType string, dayStart, dayEnd time.Time) ([]models.MealLog, error) {
	var meals []models.MealLog
	q := database.DB.Where("user_id = ? AND logged_at >= ? AND logged_at < ?", userID, dayStart, dayEnd)
	if strings.TrimSpace(mealType) != "" {
		q = q.Where("LOWER(meal_type) = ?", strings.ToLower(strings.TrimSpace(mealType)))
	}
	err := q.Order("logged_at DESC").Find(&meals).Error
	return meals, err
}

func selectMealForCorrection(candidates []models.MealLog, targetDish string) (models.MealLog, string, bool) {
	if len(candidates) == 0 {
		return models.MealLog{}, "meal_not_found", false
	}
	if strings.TrimSpace(targetDish) == "" {
		if len(candidates) == 1 {
			return candidates[0], "", true
		}
		return models.MealLog{}, "ambiguous_target", false
	}

	targetNorm := normalizeMealText(targetDish)
	var matched []models.MealLog
	for _, meal := range candidates {
		nameNorm := normalizeMealText(meal.Name)
		targetCompact := strings.ReplaceAll(targetNorm, " ", "")
		nameCompact := strings.ReplaceAll(nameNorm, " ", "")
		if nameNorm == targetNorm ||
			strings.Contains(nameNorm, targetNorm) ||
			strings.Contains(targetNorm, nameNorm) ||
			(nameCompact != "" && targetCompact != "" && (strings.Contains(nameCompact, targetCompact) || strings.Contains(targetCompact, nameCompact))) {
			matched = append(matched, meal)
		}
	}

	if len(matched) == 0 {
		return models.MealLog{}, "meal_not_found", false
	}
	if len(matched) > 1 {
		return models.MealLog{}, "ambiguous_target", false
	}
	return matched[0], "", true
}

func recentDuplicateMeal(userID uint, mealType, dishName, ingredients string, now time.Time) (*models.MealLog, error) {
	windowStart := now.Add(-dedupeWindow)
	dishNorm := normalizeMealText(dishName)
	ingNorm := normalizeMealText(ingredients)

	var recent []models.MealLog
	if err := database.DB.Where("user_id = ? AND logged_at >= ? AND logged_at <= ?", userID, windowStart, now).
		Order("logged_at DESC").Find(&recent).Error; err != nil {
		return nil, err
	}

	for i := range recent {
		m := recent[i]
		if normalizeMealText(m.MealType) != normalizeMealText(mealType) {
			continue
		}
		if normalizeMealText(m.Name) == dishNorm && normalizeMealText(m.Ingredients) == ingNorm {
			return &m, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func ambiguousMealsPayload(meals []models.MealLog, reason string) map[string]any {
	options := make([]map[string]any, 0, len(meals))
	for i, meal := range meals {
		options = append(options, map[string]any{
			"index":     i,
			"dish_name": meal.Name,
			"meal_type": meal.MealType,
			"logged_at": meal.LoggedAt.Format(time.RFC3339),
		})
	}
	return map[string]any{
		"ok":      false,
		"error":   reason,
		"message": "Multiple matching meals found. Reply with the option number (starting from 0).",
		"options": options,
	}
}

func mealDisplayLine(userID uint, m models.MealLog) string {
	return fmt.Sprintf("- %s (%s, %s): %.0f kcal, P %.1fg, C %.1fg, F %.1fg, Fi %.1fg",
		m.Name, m.MealType, userReadableTime(userID, m.LoggedAt), m.Calories, m.Protein, m.Carbs, m.Fat, m.Fiber)
}
