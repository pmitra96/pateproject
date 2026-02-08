package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/config"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

type Choice struct {
	Index   int     `json:"index"`
	Message Message `json:"message"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Choices []Choice `json:"choices"`
}

type Client struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func NewClient() *Client {
	return &Client{
		apiKey:  config.GetEnv("LLM_API_KEY", ""),
		baseURL: config.GetEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
		model:   config.GetEnv("LLM_MODEL", "gpt-3.5-turbo"),
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) Chat(messages []Message) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("LLM_API_KEY not configured")
	}

	reqBody := ChatRequest{
		Model:       c.model,
		Messages:    messages,
		MaxTokens:   1000,
		Temperature: 0.7,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return chatResp.Choices[0].Message.Content, nil
}

func (c *Client) GenerateStory(topic string) (string, error) {
	prompt := fmt.Sprintf("Tell me a short, creative story about: %s. Keep it under 200 words.", topic)
	if topic == "" {
		prompt = "Tell me a short, creative story. Keep it under 200 words."
	}

	messages := []Message{
		{Role: "system", Content: "You are a creative storyteller."},
		{Role: "user", Content: prompt},
	}

	return c.Chat(messages)
}

type InventoryItem struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
}

type PantryItemExtraction struct {
	Ingredient string  `json:"ingredient"`
	Brand      *string `json:"brand"`
	Product    *string `json:"product"`
	Nutrition  any     `json:"nutrition"`
}

func (c *Client) ExtractPantryItemInfo(rawName string) (*PantryItemExtraction, error) {
	prompt := fmt.Sprintf(`Split this raw pantry item name into structured fields: "%s"

Rules:
- ingredient: the canonical, brand-agnostic ingredient name (e.g., "Milk", "Curd", "Bread"). Must not contain brand names.
- brand: the brand or manufacturer name (e.g., "Amul", "Akshayakalpa"). Return null if not present.
- product: the brand-specific product name WITHOUT the brand (e.g., "Taaza Toned Milk", "Artisanal Organic Set Curd"). Return null if not present.
- nutrition: always return null.

If a field cannot be confidently determined, return null. Do not invent or guess information.

IMPORTANT: Return ONLY valid JSON in this exact format:
{
  "ingredient": "string",
  "brand": "string or null",
  "product": "string or null",
  "nutrition": null
}`, rawName)

	messages := []Message{
		{Role: "system", Content: "You are a data extraction assistant. Return ONLY valid JSON."},
		{Role: "user", Content: prompt},
	}

	response, err := c.Chat(messages)
	if err != nil {
		return nil, err
	}

	// Clean up potential markdown code blocks
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var extraction PantryItemExtraction
	if err := json.Unmarshal([]byte(response), &extraction); err != nil {
		return nil, fmt.Errorf("failed to parse extraction response: %w - response: %s", err, response)
	}

	return &extraction, nil
}

func (c *Client) ExtractPantryItemsBatch(rawNames []string) ([]PantryItemExtraction, error) {
	if len(rawNames) == 0 {
		return nil, nil
	}

	itemsList := strings.Join(rawNames, "\n- ")
	prompt := fmt.Sprintf(`Split these raw pantry item names into structured fields. Return a JSON array of objects.

Items:
- %s

Rules for each object:
- ingredient: the canonical, brand-agnostic ingredient name (e.g., "Milk", "Curd", "Bread"). Must not contain brand names.
- brand: the brand or manufacturer name (e.g., "Amul", "Akshayakalpa"). Return null if not present.
- product: the brand-specific product name WITHOUT the brand (e.g., "Taaza Toned Milk", "Artisanal Organic Set Curd").
- nutrition: always return null.

Format:
[
  {"ingredient": "...", "brand": "...", "product": "...", "nutrition": null},
  ...
]`, itemsList)

	resp, err := c.Chat([]Message{
		{Role: "system", Content: "You are a grocery data expert. You specialize in normalizing item names into canonical ingredients and brands. Always return valid JSON only."},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, err
	}

	// Clean output from possible markdown code blocks
	cleanResp := strings.TrimSpace(resp)
	if strings.HasPrefix(cleanResp, "```json") {
		cleanResp = strings.TrimPrefix(cleanResp, "```json")
		cleanResp = strings.TrimSuffix(cleanResp, "```")
	} else if strings.HasPrefix(cleanResp, "```") {
		cleanResp = strings.TrimPrefix(cleanResp, "```")
		cleanResp = strings.TrimSuffix(cleanResp, "```")
	}

	var extractions []PantryItemExtraction
	if err := json.Unmarshal([]byte(cleanResp), &extractions); err != nil {
		return nil, fmt.Errorf("failed to parse batch extraction JSON: %w", err)
	}

	return extractions, nil
}

// ExtractHeuristic provides a basic rule-based split when LLM is unavailable.
func (c *Client) ExtractHeuristic(rawName string) *PantryItemExtraction {
	lowerName := strings.ToLower(rawName)

	commonBrands := []string{"amul", "mooz", "akshayakalpa", "mother dairy", "milky mist", "britannia", "nestle", "urban platter", "dehaat", "honest farms", "hen fruit", "blinkit", "zepto", "swiggy", "instamart", "tata sampann", "tata", "fortune", "aashirvaad", "dabur", "haldiram", "epigamia"}
	commonIngredients := []string{"milk", "curd", "tofu", "bread", "egg", "eggs", "paneer", "butter", "cheese", "tomato", "potato", "onion", "broccoli", "peanuts", "atta", "wheat", "rice", "kala chana", "chana", "dal", "moong", "masoor", "besan", "sugar", "salt", "oil", "ghee"}

	var foundBrand *string
	var foundIngredient string = rawName // Default to raw name

	// 1. Try to find a brand
	for _, brand := range commonBrands {
		if strings.Contains(lowerName, brand) {
			b := strings.Title(brand)
			foundBrand = &b
			break
		}
	}

	// 2. Try to find a canonical ingredient
	for _, ing := range commonIngredients {
		if strings.Contains(lowerName, ing) {
			foundIngredient = strings.Title(ing)
			break
		}
	}

	// 3. Simple product name: strip the brand if found
	productName := rawName
	if foundBrand != nil {
		productName = strings.TrimSpace(strings.ReplaceAll(lowerName, strings.ToLower(*foundBrand), ""))
		productName = strings.Title(productName)
	}

	return &PantryItemExtraction{
		Ingredient: foundIngredient,
		Brand:      foundBrand,
		Product:    &productName,
		Nutrition:  nil,
	}
}

type GoalInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (c *Client) SuggestMeals(inventory []InventoryItem) (string, error) {
	if len(inventory) == 0 {
		return "", fmt.Errorf("no inventory items provided")
	}

	// Build inventory list for prompt
	var items string
	for _, item := range inventory {
		items += fmt.Sprintf("- %s: %.0f %s\n", item.Name, item.Quantity, item.Unit)
	}

	prompt := fmt.Sprintf(`Based on these ingredients in my pantry:

%s

Suggest 3 meals I can make. 

IMPORTANT RULES:
1. Portions MUST be for exactly 1 person (one serving).
2. Each ingredient must include a weight or quantity (e.g., "100g Paneer", "2 Eggs").

For each meal:
1. Name of the dish
2. Which ingredients from my pantry it uses (with specific weight/quantity)
3. Brief cooking instructions (2-3 sentences)

Format each meal clearly with the dish name as a header.`, items)

	messages := []Message{
		{Role: "system", Content: "You are a helpful cooking assistant. Suggest practical, easy-to-make meals based on available ingredients."},
		{Role: "user", Content: prompt},
	}

	return c.Chat(messages)
}

func (c *Client) SuggestMealsPersonalized(inventory []InventoryItem, goals []GoalInfo, timeOfDay string) (string, error) {
	if len(inventory) == 0 {
		return "", fmt.Errorf("no inventory items provided")
	}

	// Build inventory list
	var items string
	for _, item := range inventory {
		items += fmt.Sprintf("- %s: %.0f %s\n", item.Name, item.Quantity, item.Unit)
	}

	// Build goals list
	var goalsText string
	var goalsSummary string
	if len(goals) > 0 {
		goalsText = "\n\nMy health/fitness goals:\n"
		for i, goal := range goals {
			goalsText += fmt.Sprintf("- %s", goal.Title)
			if goal.Description != "" {
				goalsText += fmt.Sprintf(": %s", goal.Description)
			}
			goalsText += "\n"
			if i == 0 {
				goalsSummary = goal.Title
			}
		}
	} else {
		goalsText = "\n\nNo specific health goals set."
		goalsSummary = "General healthy eating"
	}

	// Time context
	mealType := "meal"
	switch timeOfDay {
	case "morning":
		mealType = "breakfast"
	case "afternoon":
		mealType = "lunch"
	case "evening":
		mealType = "dinner"
	case "night":
		mealType = "light snack"
	}

	prompt := fmt.Sprintf(`Based on these ingredients in my pantry:

%s
%s

Suggest 3 %s options that align with my goals.

IMPORTANT RULES:
1. All meal portions MUST be calculated for EXACTLY 1 serving (for one person).
2. Each ingredient in the "ingredients" list must include a specific weight/quantity (e.g., "150g Chicken breast", "2 Eggs", "1 cup Rice").

IMPORTANT: Return ONLY valid JSON in this exact format, no other text:
{
  "goal": "%s",
  "meal_type": "%s",
  "meals": [
    {
      "name": "Dish Name",
      "ingredients": ["100g ingredient 1", "2 units ingredient 2"],
      "instructions": "Step by step cooking instructions",
      "prep_time": "10 mins",
      "calories": 250,
      "protein": 15,
      "benefits": "How this helps achieve the goal"
    }
  ]
}`, items, goalsText, mealType, goalsSummary, mealType)

	messages := []Message{
		{Role: "system", Content: "You are a nutritionist. Return ONLY valid JSON, no markdown, no explanation. Follow the exact JSON structure requested."},
		{Role: "user", Content: prompt},
	}

	// Step 1: Generate initial suggestions
	initialResponse, err := c.Chat(messages)
	if err != nil {
		return "", err
	}

	// Step 2: Judge the response
	judgePrompt := fmt.Sprintf(`You are a nutrition expert and food critic. Evaluate this meal suggestion for someone with the goal: "%s"

Meal suggestions:
%s

Judge each meal on:
1. **Authenticity**: Are these real, properly named dishes? Are cooking instructions realistic?
2. **Goal alignment**: Do calories/protein match the stated goal?
3. **Serving Size**: Is the meal strictly for EXACTLY 1 serving (one person)?
4. **Ingredient Detail**: Does every ingredient include a specific weight or quantity (e.g., "100g", "2 units")?
5. **Nutritional accuracy**: Are calorie/protein estimates reasonable for the given quantities?

Return ONLY valid JSON:
{
  "issues": [
    {"meal": "dish name", "problem": "specific issue", "suggestion": "how to fix"}
  ],
  "overall_score": 8,
  "needs_refinement": true
}`, goalsSummary, initialResponse)

	judgeMessages := []Message{
		{Role: "system", Content: "You are a strict nutrition and culinary expert. Evaluate meal suggestions critically. Return ONLY valid JSON. Focus on serving size (1 person) and ingredient weights."},
		{Role: "user", Content: judgePrompt},
	}

	judgeResponse, err := c.Chat(judgeMessages)
	if err != nil {
		// If judge fails, return initial response
		return initialResponse, nil
	}

	// Check if refinement needed
	if !strings.Contains(judgeResponse, `"needs_refinement": true`) {
		return initialResponse, nil
	}

	// Step 3: Refine based on judge feedback
	refinePrompt := fmt.Sprintf(`Based on this expert feedback, improve the meal suggestions.

Original suggestions:
%s

Expert feedback:
%s

CRITICAL RULES TO FIX:
1. Every meal MUST be for exactly 1 serving.
2. Every ingredient MUST have a weight/quantity (e.g. "100g Paneer").

Fix ALL issues mentioned. Return the improved suggestions in the SAME JSON format:
{
  "goal": "%s",
  "meal_type": "%s",
  "meals": [
    {
      "name": "Dish Name",
      "ingredients": ["100g ingredient 1", "2 units ingredient 2"],
      "instructions": "Detailed step by step cooking instructions",
      "prep_time": "15 mins",
      "calories": 250,
      "protein": 15,
      "benefits": "How this helps achieve the goal"
    }
  ]
}

Make dishes more authentic with proper names, realistic cooking times, and accurate nutritional info and serving sizes.`, initialResponse, judgeResponse, goalsSummary, mealType)

	refineMessages := []Message{
		{Role: "system", Content: "You are an expert chef and nutritionist. Improve meal suggestions based on feedback. Return ONLY valid JSON."},
		{Role: "user", Content: refinePrompt},
	}

	refinedResponse, err := c.Chat(refineMessages)
	if err != nil {
		return initialResponse, nil
	}

	return refinedResponse, nil
}
