package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/config"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/llm"
	"github.com/pmitra96/pateproject/logger"
	"github.com/pmitra96/pateproject/models"
)

type NutritionService struct {
	llmClient *llm.Client
}

func NewNutritionService() *NutritionService {
	return &NutritionService{
		llmClient: llm.NewClient(),
	}
}

// FetchItemNutrition attempts to fetch nutrition data for an item.
func (s *NutritionService) FetchItemNutrition(item *models.Item) error {
	// Step 0: Check our own Scraper (Zepto)
	err := s.fetchFromPythonScraper(item)
	if err == nil && item.NutritionVerified {
		logger.Info("Nutrition fetched from Zepto Scraper", "item", item.Name)
		return nil
	}

	// Step 1: Check Open Food Facts
	err = s.fetchFromOpenFoodFacts(item)
	if err == nil && item.NutritionVerified {
		logger.Info("Nutrition fetched from Open Food Facts", "item", item.Name)
		return nil
	}

	// Step 2: Fallback to LLM Estimation
	return s.estimateWithLLM(item)
}

func (s *NutritionService) fetchFromPythonScraper(item *models.Item) error {
	baseURL := config.GetEnv("SCRAPER_API_URL", "http://localhost:8000")

	cleanProductName := strings.TrimSpace(item.ProductName)
	brandName := ""
	if item.Brand != nil {
		brandName = strings.TrimSpace(item.Brand.Name)
	}

	query := cleanProductName
	if brandName != "" && !strings.Contains(strings.ToLower(cleanProductName), strings.ToLower(brandName)) {
		query = brandName + " " + cleanProductName
	}

	url := fmt.Sprintf("%s/api/v1/products/search?query=%s", baseURL, strings.ReplaceAll(query, " ", "+"))
	logger.Info("Searching Python Scraper", "query", query, "url", url)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("scraper request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("scraper returned status: %d", resp.StatusCode)
	}

	var results []struct {
		Name          string `json:"name"`
		NutritionInfo struct {
			Energy        float64 `json:"energy"`
			Protein       float64 `json:"protein"`
			Fat           float64 `json:"fat"`
			Carbohydrates float64 `json:"carbohydrates"`
		} `json:"nutrition_info"`
		ServingSizeValue float64 `json:"serving_size_value"`
		ServingSizeUnit  string  `json:"serving_size_unit"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return fmt.Errorf("failed to decode scraper response: %v", err)
	}

	if len(results) > 0 {
		// Take the first result
		p := results[0]
		if p.NutritionInfo.Energy > 0 || p.NutritionInfo.Protein > 0 {
			item.Calories = p.NutritionInfo.Energy
			item.Protein = p.NutritionInfo.Protein
			item.Carbs = p.NutritionInfo.Carbohydrates
			item.Fat = p.NutritionInfo.Fat

			// Track portion context
			if p.ServingSizeValue > 0 {
				item.ServingWeight = p.ServingSizeValue
				item.ServingUnit = p.ServingSizeUnit
			} else {
				// Default to 100g standard if not specified
				item.ServingWeight = 100
				item.ServingUnit = "g"
			}

			item.NutritionVerified = true
			return nil
		}
	}

	return fmt.Errorf("no products found in scraper")
}

func (s *NutritionService) fetchFromOpenFoodFacts(item *models.Item) error {
	queries := []string{}

	cleanProductName := strings.TrimSpace(item.ProductName)
	brandName := ""
	if item.Brand != nil {
		brandName = strings.TrimSpace(item.Brand.Name)
	}

	if brandName != "" {
		// Tier 1: Brand + Specific Product Name
		// Check if brand is already at the start of product name to avoid "Mooz Mooz ..."
		fullQuery := cleanProductName
		if !strings.HasPrefix(strings.ToLower(cleanProductName), strings.ToLower(brandName)) {
			fullQuery = brandName + " " + cleanProductName
		}
		queries = append(queries, fullQuery)

		// Tier 2: Brand + Ingredient Name
		queries = append(queries, brandName+" "+item.Ingredient.Name)

		// Tier 3: Brand + Simplified Product Name (most specific noun)
		// Strip common generic words that clutter search
		stripWords := []string{"organic", "set", "artisanal", "pure", "fresh", "toned", "natural"}
		simplified := strings.ToLower(cleanProductName)
		for _, w := range stripWords {
			simplified = strings.ReplaceAll(simplified, w, "")
		}
		parts := strings.Fields(simplified)
		if len(parts) > 0 {
			// Try Brand + Last Word (usually the noun)
			nounSearch := brandName + " " + parts[len(parts)-1]
			if !strings.Contains(strings.ToLower(fullQuery), strings.ToLower(nounSearch)) {
				queries = append(queries, nounSearch)
			}
		}
	} else {
		if cleanProductName != "" {
			queries = append(queries, cleanProductName)
		}
		if item.Ingredient.Name != "" {
			queries = append(queries, item.Ingredient.Name)
		}
	}

	uniqueQueries := []string{}
	seen := make(map[string]bool)
	for _, q := range queries {
		q = strings.TrimSpace(q)
		if q != "" && !seen[strings.ToLower(q)] {
			uniqueQueries = append(uniqueQueries, q)
			seen[strings.ToLower(q)] = true
		}
	}

	limit := 3
	for i, query := range uniqueQueries {
		if i >= limit {
			break
		}
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		logger.Info("Searching Open Food Facts", "query", query)
		url := fmt.Sprintf("https://world.openfoodfacts.org/cgi/search.pl?search_terms=%s&search_simple=1&action=process&json=1", strings.ReplaceAll(query, " ", "+"))

		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			logger.Warn("Open Food Facts search failed or timed out", "query", query, "error", err)
			continue
		}
		defer resp.Body.Close()

		var result struct {
			Products []struct {
				Nutriments struct {
					EnergyKcal100g    json.Number `json:"energy-kcal_100g"`
					Proteins100g      json.Number `json:"proteins_100g"`
					Carbohydrates100g json.Number `json:"carbohydrates_100g"`
					Fat100g           json.Number `json:"fat_100g"`
					Fiber100g         json.Number `json:"fiber_100g"`
				} `json:"nutriments"`
			} `json:"products"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			logger.Warn("Failed to decode Open Food Facts response", "query", query, "error", err)
			continue
		}

		if len(result.Products) > 0 {
			p := result.Products[0]
			kcal, _ := p.Nutriments.EnergyKcal100g.Float64()
			protein, _ := p.Nutriments.Proteins100g.Float64()
			carbs, _ := p.Nutriments.Carbohydrates100g.Float64()
			fat, _ := p.Nutriments.Fat100g.Float64()
			fiber, _ := p.Nutriments.Fiber100g.Float64()

			// Only accept if we have meaningful energy data
			if kcal > 0 {
				item.Calories = kcal
				item.Protein = protein
				item.Carbs = carbs
				item.Fat = fat
				item.Fiber = fiber
				item.NutritionVerified = true
				logger.Info("Nutrition fetched from Open Food Facts", "item", item.Name, "query", query)
				return nil
			}
			logger.Warn("Open Food Facts returned zero calories", "query", query)
		}
	}

	return fmt.Errorf("no valid products found on Open Food Facts for any tried queries")
}

func (s *NutritionService) estimateWithLLM(item *models.Item) error {
	logger.Info("Using LLM to estimate nutrition", "item", item.Name)

	unitType := "per 100g"
	isCountBased := false
	lowerUnit := strings.ToLower(item.Unit)
	if lowerUnit == "pc" || lowerUnit == "pcs" || lowerUnit == "unit" || lowerUnit == "units" || lowerUnit == "piece" || lowerUnit == "pieces" || lowerUnit == "pack" || lowerUnit == "dozen" {
		unitType = "per 1 unit/piece"
		isCountBased = true
	} else if lowerUnit == "ml" || lowerUnit == "l" {
		unitType = "per 100ml"
	}

	prompt := fmt.Sprintf(`Provide nutritional information %s for this item. 
Item: %s (Brand: %s, Ingredient: %s, Unit: %s)

Return ONLY a JSON object:
{
  "calories": float,
  "protein": float,
  "carbs": float,
  "fat": float,
  "fiber": float
}`, unitType, item.ProductName, func() string {
		if item.Brand != nil {
			return item.Brand.Name
		}
		return "Unknown"
	}(), item.Ingredient.Name, item.Unit)

	resp, err := s.llmClient.Chat([]llm.Message{
		{Role: "system", Content: fmt.Sprintf("You are a nutrition expert. Provide estimated nutritional data %s. If brand info is unavailable, use average values for the ingredient.", unitType)},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return err
	}

	// Clean output from possible markdown code blocks
	cleanResp := strings.TrimSpace(resp)
	if strings.HasPrefix(cleanResp, "```json") {
		cleanResp = strings.TrimPrefix(cleanResp, "```json")
		cleanResp = strings.TrimSuffix(cleanResp, "```")
	}

	var data struct {
		Calories float64 `json:"calories"`
		Protein  float64 `json:"protein"`
		Carbs    float64 `json:"carbs"`
		Fat      float64 `json:"fat"`
		Fiber    float64 `json:"fiber"`
	}

	if err := json.Unmarshal([]byte(cleanResp), &data); err != nil {
		return err
	}

	// Sanity Checks: Ensure values are realistic per 100g
	// Max possible calories in 100g (pure fat) is ~900.
	if data.Calories > 900 {
		logger.Warn("Insane calorie value detected, capping at 900", "val", data.Calories)
		data.Calories = 900
	}
	// Max macros per 100g is 100g
	if data.Protein > 100 {
		data.Protein = 100
	}
	if data.Carbs > 100 {
		data.Carbs = 100
	}
	if data.Fat > 100 {
		data.Fat = 100
	}
	if data.Fiber > 100 {
		data.Fiber = 100
	}

	item.Calories = data.Calories
	item.Protein = data.Protein
	item.Carbs = data.Carbs
	item.Fat = data.Fat
	item.Fiber = data.Fiber
	item.NutritionVerified = false // It's an estimation

	msg := "🔥 Nutrition estimated (per 100g/ml)"
	if isCountBased {
		msg = "🔥 Nutrition estimated (per piece/unit)"
	}
	logger.Info(msg, "item", item.Name, "kcal", item.Calories)
	return nil
}

// EstimateNutritionFromQuery estimates nutrition from a text query
// It first checks the database for a matching item, then falls back to LLM
func (s *NutritionService) EstimateNutritionFromQuery(query string) (*FoodEstimate, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	// 1. Search DB for exact or close match
	// We'll search Items table.
	var item models.Item
	// Try simplified search: Name ILIKE query
	err := database.DB.Where("name ILIKE ?", query).Or("product_name ILIKE ?", query).Order("nutrition_verified DESC").First(&item).Error
	if err == nil {
		// Found it! use its macros
		// Check if it has non-zero macros
		if item.Calories > 0 {
			logger.Info("Found item in DB for query", "query", query, "item", item.Name)
			return &FoodEstimate{
				Calories: item.Calories,
				Protein:  item.Protein,
				Fat:      item.Fat,
				Carbs:    item.Carbs,
				Name:     item.Name,
			}, nil
		}
	}

	// 2. Fallback to LLM
	logger.Info("No DB match found, estimating with LLM", "query", query)

	// We need to be careful with unit inference.
	// If query is "2 eggs", we should parse it?
	// For "Can I Eat This", the user usually asks "Can I eat a pizza?" (1 unit implied)
	// or "Can I eat 2 slices of pizza?".
	//
	// Let's refine the LLM prompt in a new private method or reuse.
	// estimateWithLLM builds a prompt based on Item fields.
	// Let's just create a new simple method for query-based estimation to avoid hacky Item struct usage.

	prompt := fmt.Sprintf(`Estimate nutritional information for: "%s".
Assume a standard serving size if quantity is not specified.

Return ONLY a JSON object:
{
  "calories": float,
  "protein": float,
  "carbs": float,
  "fat": float,
  "serving_size": string
}`, query)

	// Using the same client
	resp, err := s.llmClient.Chat([]llm.Message{
		{Role: "system", Content: "You are a nutrition expert. Provide estimated nutritional data. Be conservative but realistic."},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, err
	}

	cleanResp := strings.TrimSpace(resp)
	if strings.HasPrefix(cleanResp, "```json") {
		cleanResp = strings.TrimPrefix(cleanResp, "```json")
		cleanResp = strings.TrimSuffix(cleanResp, "```")
	}

	var data struct {
		Calories    float64 `json:"calories"`
		Protein     float64 `json:"protein"`
		Carbs       float64 `json:"carbs"`
		Fat         float64 `json:"fat"`
		ServingSize string  `json:"serving_size"`
	}

	if err := json.Unmarshal([]byte(cleanResp), &data); err != nil {
		return nil, err
	}

	return &FoodEstimate{
		Calories: data.Calories,
		Protein:  data.Protein,
		Fat:      data.Fat,
		Carbs:    data.Carbs,
		Name:     query, // or data.ServingSize + " " + query
	}, nil
}
