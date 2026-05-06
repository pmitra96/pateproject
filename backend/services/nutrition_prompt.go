package services

import (
	"os"
	"path/filepath"
	"strings"
)

func loadNutritionEstimatorPrompt() string {
	data, err := os.ReadFile(filepath.Join("llm", "nutrition_estimator.md"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
