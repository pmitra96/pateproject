package llm

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pmitra96/pateproject/logger"
)

type MealParserItem struct {
	RawText                  string   `json:"raw_text"`
	FoodName                 string   `json:"food_name"`
	Brand                    string   `json:"brand"`
	Quantity                 float64  `json:"quantity"`
	Unit                     string   `json:"unit"`
	QuantityInGramsEstimated *float64 `json:"quantity_in_grams_estimated"`
	Ingredients              []MealParserIngredient `json:"ingredients"`
	CookingMethod            string   `json:"cooking_method"`
	Modifiers                []string `json:"modifiers"`
	Assumptions              []string `json:"assumptions"`
	Calories                 float64  `json:"calories"`
	ProteinG                 float64  `json:"protein_g"`
	CarbsG                   float64  `json:"carbs_g"`
	FatG                     float64  `json:"fat_g"`
	Confidence               string   `json:"confidence"`
}

type MealParserIngredient struct {
	Name      string  `json:"name"`
	Quantity  float64 `json:"quantity"`
	Unit      string  `json:"unit"`
	Brand     string  `json:"brand"`
	Calories  float64 `json:"calories"`
	ProteinG  float64 `json:"protein_g"`
	CarbsG    float64 `json:"carbs_g"`
	FatG      float64 `json:"fat_g"`
	FiberG    float64 `json:"fiber_g"`
}

type MealParserResult struct {
	MealType            string           `json:"meal_type"`
	ParsedItems         []MealParserItem `json:"parsed_items"`
	ClarificationNeeded bool             `json:"clarification_needed"`
	ClarificationQ      *string          `json:"clarification_question"`
}

func (c *Client) ParseMealV1(userMessage string, userContext string) (*MealParserResult, error) {
	prompt := loadPromptFile(filepath.Join("llm", "meal_parser_v1.md"))
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("meal parser prompt missing")
	}
	messages := []Message{
		{
			Role: "system",
			Content: "You are a strict JSON meal parser. Output valid JSON only. " +
				"Do not use markdown or code fences.",
		},
		{
			Role: "user",
			Content: fmt.Sprintf("%s\n\nUser context:\n%s\n\nUser input:\n%s",
				prompt, strings.TrimSpace(userContext), strings.TrimSpace(userMessage)),
		},
	}
	out, _, err := c.ChatJSON(messages)
	if err != nil {
		return nil, err
	}
	parsed := strings.TrimSpace(out)
	parsed = strings.TrimPrefix(parsed, "```json")
	parsed = strings.TrimPrefix(parsed, "```")
	parsed = strings.TrimSuffix(parsed, "```")
	parsed = strings.TrimSpace(parsed)

	var result MealParserResult
	if err := json.Unmarshal([]byte(parsed), &result); err != nil {
		preview := strings.ReplaceAll(parsed, "\n", "\\n")
		if len(preview) > 1800 {
			preview = preview[:1800] + "...(truncated)"
		}
		logger.Warn("Meal parser JSON unmarshal failed", "error", err.Error(), "raw_preview", preview, "raw_len", len(parsed))
		return nil, fmt.Errorf("meal parser json parse: %w", err)
	}
	if result.ParsedItems == nil {
		result.ParsedItems = []MealParserItem{}
	}
	for i := range result.ParsedItems {
		if result.ParsedItems[i].Ingredients == nil {
			result.ParsedItems[i].Ingredients = []MealParserIngredient{}
		}
	}
	return &result, nil
}
