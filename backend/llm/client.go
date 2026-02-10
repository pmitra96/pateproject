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
	"github.com/pmitra96/pateproject/websearch"
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
	// Nutrition per 100g/ml or per unit
	Calories float64 `json:"calories"`
	Protein  float64 `json:"protein"`
	Fat      float64 `json:"fat"`
	Carbs    float64 `json:"carbs"`
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

type UserPreferencesInfo struct {
	Country           string   `json:"country"`
	State             string   `json:"state"`
	City              string   `json:"city"`
	PreferredCuisines []string `json:"preferred_cuisines"`
}

type DishSampleInfo struct {
	Dish        string   `json:"dish"`
	Cuisine     string   `json:"cuisine"`
	Details     string   `json:"details"`
	Ingredients []string `json:"ingredients"`
	Calories    string   `json:"calories"`
}

func (c *Client) SuggestMeals(inventory []InventoryItem) (string, error) {
	items := ""
	for _, item := range inventory {
		items += fmt.Sprintf("- %s: %.2f %s\n", item.Name, item.Quantity, item.Unit)
	}

	prompt := fmt.Sprintf(`I have the following ingredients in my pantry:
%s

Suggest 3 meals I can cook using these ingredients. You can assume I have basic spices (salt, pepper, oil, turmeric, chili powder).
For each meal, provide:
1. Name
2. Ingredients needed (with quantities)
3. Brief instructions
4. Estimated calories and protein per serving

Format the output as a JSON list of objects with keys: "name", "ingredients" (list of strings), "instructions", "calories" (number), "protein" (number).`, items)

	resp, err := c.Chat([]Message{
		{Role: "system", Content: "You are a grocery data expert. You specialize in normalizing item names into canonical ingredients and brands. Always return valid JSON only."},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return "", err
	}
	return resp, nil
}

func (c *Client) SuggestMealsPersonalized(inventory []InventoryItem, goals []GoalInfo, timeOfDay string, preferences *UserPreferencesInfo, dishSamples []DishSampleInfo) (string, error) {
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

	// Build user preferences context
	var preferencesText string
	if preferences != nil {
		if preferences.Country != "" || preferences.State != "" || preferences.City != "" {
			preferencesText = "\n\nUser's Location: "
			locationParts := []string{}
			if preferences.City != "" {
				locationParts = append(locationParts, preferences.City)
			}
			if preferences.State != "" {
				locationParts = append(locationParts, preferences.State)
			}
			if preferences.Country != "" {
				locationParts = append(locationParts, preferences.Country)
			}
			preferencesText += strings.Join(locationParts, ", ")
		}
		if len(preferences.PreferredCuisines) > 0 {
			preferencesText += "\nPreferred Cuisines: " + strings.Join(preferences.PreferredCuisines, ", ")
		}
	}

	// Build dish samples context
	var dishSamplesText string
	if len(dishSamples) > 0 {
		dishSamplesText = "\n\nReference dishes from user's preferred cuisines (use these as inspiration):\n"
		for _, dish := range dishSamples {
			dishSamplesText += fmt.Sprintf("- %s (%s): %s\n", dish.Dish, dish.Cuisine, dish.Details)
			if dish.Calories != "" {
				dishSamplesText += fmt.Sprintf("  Calories: %s\n", dish.Calories)
			}
		}
	}

	// Time context
	mealType := "meal"
	lowerTime := strings.ToLower(timeOfDay)
	switch lowerTime {
	case "morning", "breakfast":
		mealType = "breakfast"
	case "afternoon", "lunch":
		mealType = "lunch"
	case "evening", "dinner":
		mealType = "dinner"
	case "night", "snack", "light snack":
		mealType = "light snack"
	default:
		// If it's a specific meal type passed directly, use it
		// This covers "snack", "brunch", etc. if passed explicitly
		mealType = lowerTime
	}

	prompt := fmt.Sprintf(`Based on these ingredients in my pantry:

%s
%s%s%s

Suggest 3 %s options that align with my goals and preferred cuisines.

IMPORTANT QUALITY GUIDELINES - Self-evaluate before responding:
- Use AUTHENTIC dish names, BUT they must match the ingredients.
- **CRITICAL**: The dish name MUST reflect the actual main ingredients used.
- **Strictly Forbidden**: Do NOT use traditional names that imply ingredients (especially meats) not present in the list.
- If a traditional recipe uses a substitute (e.g. Tofu instead of Meat), the name MUST change to reflect the substitute.
- Ensure cooking instructions are REALISTIC and detailed
- Calorie estimates must be ACCURATE for portion sizes
- Protein values must match the actual ingredients used
- Prioritize dishes from user's preferred cuisines when possible

IMPORTANT RULES:
1. All meal portions MUST be calculated for EXACTLY 1 serving (for one person).
2. Each ingredient in the "ingredients" list must include a specific weight/quantity (e.g., "150g Chicken breast", "2 Eggs", "1 cup Rice").
3. If dish samples are provided, use them as inspiration for authentic dish names and preparation methods.
4. **CRITICAL - RECIPE FIRST APPROACH**: 
    a. First, decide on a standard, authentic single-serving recipe. (e.g., "I need 1 Capsicum and 100g Paneer").
    b. Ignore the *Total Quantity* I have in stock (e.g., if I have 55 capsicums, do NOT use 55. Use only 1).
    c. THEN, find the nutrition density of that item from my list (e.g., "Capsicum: 20kcal/pc").
    d. Multiply your recipe amount by the nutrition density (e.g. 1 pc * 20kcal/pc = 20kcal).
5. **CRITICAL**: Calculate the total calories, protein, fat, and carbs by SUMMING these specific calculated values. Do NOT guess generic values. Use the data provided.
6. **NAMING CONVENTION**: The dish name must be descriptive of the *ingredients actually present*. (e.g. "Spicy [Main Ingredient] Curry", not just "Spicy Curry" or the name of a meat dish if no meat is used).

IMPORTANT: Return ONLY valid JSON in this exact format, no other text:
{
  "goal": "%s",
  "meal_type": "%s",
  "confidence": 8,
  "meals": [
    {
      "name": "Dish Name",
      "cuisine": "Cuisine Type",
      "ingredients": ["100g ingredient 1", "2 units ingredient 2"],
      "instructions": "Step by step cooking instructions",
      "prep_time": "10 mins",
      "calories": 250,
      "protein": 15,
      "fat": 10,
      "carbs": 30,
      "benefits": "How this helps achieve the goal"
    }
  ]
}

Set "confidence" (1-10) based on how well you followed the quality guidelines.`, items, goalsText, preferencesText, dishSamplesText, mealType, goalsSummary, mealType)

	messages := []Message{
		{Role: "system", Content: "You are an expert nutritionist and chef. Suggest authentic, well-researched meals. Self-evaluate your response quality. Return ONLY valid JSON."},
		{Role: "user", Content: prompt},
	}

	// Log the prompt being sent
	fmt.Println("\n========== LLM PROMPT ==========")
	fmt.Println("SYSTEM:", messages[0].Content)
	fmt.Println("\nUSER:", messages[1].Content)
	fmt.Println("================================\n")

	// Step 1: Generate with self-evaluation
	initialResponse, err := c.Chat(messages)
	if err != nil {
		return "", err
	}

	// Step 2: Check confidence - only refine if low confidence (<7)
	if !strings.Contains(initialResponse, `"confidence"`) ||
		strings.Contains(initialResponse, `"confidence": 9`) ||
		strings.Contains(initialResponse, `"confidence": 10`) ||
		strings.Contains(initialResponse, `"confidence": 8`) ||
		strings.Contains(initialResponse, `"confidence": 7`) {
		// High confidence, clean and return
		return cleanJSONResponse(initialResponse), nil
	}

	// Low confidence - run judge and refine
	refinePrompt := fmt.Sprintf(`The following meal suggestions have low confidence. Improve them.

Original:
%s

Requirements:
- Use AUTHENTIC dish names from real cuisines
- Detailed, realistic cooking instructions
- Accurate calorie/protein for the portions
- Clear goal alignment

Return improved JSON in same format with confidence 8+:
{
  "goal": "%s",
  "meal_type": "%s",
  "confidence": 9,
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

Make dishes more authentic with proper names, realistic cooking times, and accurate nutritional info and serving sizes.`, initialResponse, goalsSummary, mealType)

	refineMessages := []Message{
		{Role: "system", Content: "You are an expert chef. Improve low-quality meal suggestions to be authentic and accurate. Return ONLY valid JSON."},
		{Role: "user", Content: refinePrompt},
	}

	refinedResponse, err := c.Chat(refineMessages)
	if err != nil {
		return cleanJSONResponse(initialResponse), nil
	}

	return cleanJSONResponse(refinedResponse), nil
}

// ChatMessage represents a conversation message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatWithContext handles chatbot conversations with inventory and goals context
func (c *Client) ChatWithContext(userMessage string, history []ChatMessage, inventory []InventoryItem, goals []GoalInfo) (string, error) {
	// Build inventory context
	var inventoryText string
	if len(inventory) > 0 {
		inventoryText = "User's pantry inventory:\n"
		for _, item := range inventory {
			inventoryText += fmt.Sprintf("- %s: %.0f %s\n", item.Name, item.Quantity, item.Unit)
		}
	} else {
		inventoryText = "User's pantry is empty."
	}

	// Build goals context
	var goalsText string
	if len(goals) > 0 {
		goalsText = "\nUser's health goals:\n"
		for _, goal := range goals {
			goalsText += fmt.Sprintf("- %s", goal.Title)
			if goal.Description != "" {
				goalsText += fmt.Sprintf(": %s", goal.Description)
			}
			goalsText += "\n"
		}
	} else {
		goalsText = "\nNo specific health goals set."
	}

	systemPrompt := fmt.Sprintf(`You are a helpful kitchen assistant for a pantry management app. You help users with:
- Questions about their inventory (what they have, what's low, expiring soon)
- Meal suggestions based on available ingredients
- Nutrition advice aligned with their goals
- Cooking tips and recipes

%s
%s

Be concise, friendly, and helpful. If asked about items not in the inventory, mention that.
For meal suggestions, use only ingredients from the inventory.`, inventoryText, goalsText)

	// Build messages array
	messages := []Message{
		{Role: "system", Content: systemPrompt},
	}

	// Add conversation history
	for _, msg := range history {
		messages = append(messages, Message{Role: msg.Role, Content: msg.Content})
	}

	// Add current user message
	messages = append(messages, Message{Role: "user", Content: userMessage})

	return c.Chat(messages)
}

// SummarizeConversation creates a brief summary of a chat conversation
func (c *Client) SummarizeConversation(messages []ChatMessage) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("no messages to summarize")
	}

	// Build conversation text
	var conversationText string
	for _, msg := range messages {
		role := "User"
		if msg.Role == "assistant" {
			role = "Assistant"
		}
		conversationText += fmt.Sprintf("%s: %s\n", role, msg.Content)
	}

	prompt := fmt.Sprintf(`Summarize this kitchen/pantry conversation in 1-2 sentences. Focus on what the user asked about and key recommendations given.

Conversation:
%s

Return ONLY the summary, no other text.`, conversationText)

	summaryMessages := []Message{
		{Role: "system", Content: "You are a summarizer. Create brief, informative summaries of conversations."},
		{Role: "user", Content: prompt},
	}

	return c.Chat(summaryMessages)
}

// WebSearchDishInfo represents dish info from web search
type WebSearchDishInfo struct {
	DishName    string `json:"dish_name"`
	Description string `json:"description"`
	Cuisine     string `json:"cuisine"`
	Source      string `json:"source"`
}

// SuggestMealsWithWebSearch uses web search results when database samples aren't found
func (c *Client) SuggestMealsWithWebSearch(inventory []InventoryItem, goals []GoalInfo, timeOfDay string, preferences *UserPreferencesInfo, webResults []websearch.DishSearchResult) (string, error) {
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

	// Build user preferences context
	var preferencesText string
	var locationStr string
	if preferences != nil {
		locationParts := []string{}
		if preferences.City != "" {
			locationParts = append(locationParts, preferences.City)
		}
		if preferences.State != "" {
			locationParts = append(locationParts, preferences.State)
		}
		if preferences.Country != "" {
			locationParts = append(locationParts, preferences.Country)
		}
		if len(locationParts) > 0 {
			locationStr = strings.Join(locationParts, ", ")
			preferencesText = "\n\nUser's Location: " + locationStr
		}
		if len(preferences.PreferredCuisines) > 0 {
			preferencesText += "\nPreferred Cuisines: " + strings.Join(preferences.PreferredCuisines, ", ")
		}
	}

	// Build web search results context
	var webSearchText string
	if len(webResults) > 0 {
		webSearchText = "\n\nDishes found via web search (popular in user's region and cuisine preferences):\n"
		for _, dish := range webResults {
			webSearchText += fmt.Sprintf("- %s (%s): %s\n", dish.DishName, dish.Cuisine, dish.Description)
		}
		webSearchText += "\nUse these web search results as inspiration. Select dishes that can be made with the user's available pantry items."
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

	// Get cuisine for prompt
	cuisineForPrompt := "regional"
	if preferences != nil && len(preferences.PreferredCuisines) > 0 {
		cuisineForPrompt = preferences.PreferredCuisines[0]
	}

	prompt := fmt.Sprintf(`Based on these ingredients in my pantry:

%s
%s%s%s

I searched the web for popular %s dishes and found the above results.

Now suggest 3 %s options by:
1. Looking at the web search results for dish ideas popular in my location/cuisine
2. Selecting dishes that can be made with my available pantry ingredients
3. Adapting the recipes to use what I have

IMPORTANT QUALITY GUIDELINES:
- Prioritize dishes from web search that match my available ingredients
- Use AUTHENTIC dish names from the search results
- Ensure cooking instructions are REALISTIC and detailed
- Calorie estimates must be ACCURATE for portion sizes
- If a web search dish needs ingredients I don't have, suggest a close alternative I can make

IMPORTANT RULES:
1. All meal portions MUST be calculated for EXACTLY 1 serving (for one person).
2. Each ingredient in the "ingredients" list must include a specific weight/quantity.
3. Clearly indicate which dish from web search inspired each suggestion.

IMPORTANT: Return ONLY valid JSON in this exact format, no other text:
{
  "goal": "%s",
  "meal_type": "%s",
  "source": "web_search",
  "confidence": 8,
  "meals": [
    {
      "name": "Dish Name",
      "cuisine": "Cuisine Type",
      "inspired_by": "Name of dish from web search that inspired this",
      "ingredients": ["100g ingredient 1", "2 units ingredient 2"],
      "instructions": "Step by step cooking instructions",
      "prep_time": "10 mins",
      "calories": 250,
      "protein": 15,
      "benefits": "How this helps achieve the goal"
    }
  ]
}`, items, goalsText, preferencesText, webSearchText, cuisineForPrompt, mealType, goalsSummary, mealType)

	messages := []Message{
		{Role: "system", Content: "You are an expert nutritionist and chef. You help users cook authentic regional dishes using their available pantry items. Use web search results as inspiration to suggest dishes that match the user's location and preferences. Return ONLY valid JSON."},
		{Role: "user", Content: prompt},
	}

	// Log the prompt being sent
	fmt.Println("\n========== LLM PROMPT (Web Search) ==========")
	fmt.Println("SYSTEM:", messages[0].Content)
	fmt.Println("\nUSER:", messages[1].Content)
	fmt.Println("==============================================\n")

	response, err := c.Chat(messages)
	if err != nil {
		return "", err
	}

	// Clean markdown code blocks from response
	return cleanJSONResponse(response), nil
}

// cleanJSONResponse removes markdown code blocks from LLM JSON responses
func cleanJSONResponse(response string) string {
	response = strings.TrimSpace(response)
	// Remove ```json or ``` markers
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
	}
	if strings.HasSuffix(response, "```") {
		response = strings.TrimSuffix(response, "```")
	}
	return strings.TrimSpace(response)
}

// ExtractDishesFromWebSearch uses LLM to parse and clean web search results into structured dish info
func (c *Client) ExtractDishesFromWebSearch(webResults []websearch.DishSearchResult, cuisine string) ([]DishSampleInfo, error) {
	if len(webResults) == 0 {
		return nil, nil
	}

	var resultsText string
	for i, r := range webResults {
		resultsText += fmt.Sprintf("%d. %s - %s\n", i+1, r.DishName, r.Description)
	}

	prompt := fmt.Sprintf(`Extract structured dish information from these web search results about %s cuisine:

%s

For each dish found, extract:
- dish: The dish name (clean, no website names)
- cuisine: The cuisine type
- details: Brief description of the dish
- ingredients: Common ingredients (if mentioned or if you know them)
- calories: Approximate calories per serving (estimate if not mentioned)

Return ONLY valid JSON array:
[
  {
    "dish": "Dish Name",
    "cuisine": "%s",
    "details": "Brief description",
    "ingredients": ["ingredient1", "ingredient2"],
    "calories": "250 kcal"
  }
]

Only include actual dish names, skip any non-food results.`, cuisine, resultsText, cuisine)

	messages := []Message{
		{Role: "system", Content: "You are a culinary expert. Extract structured dish information from search results. Return ONLY valid JSON."},
		{Role: "user", Content: prompt},
	}

	response, err := c.Chat(messages)
	if err != nil {
		return nil, err
	}

	// Clean response
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var dishes []DishSampleInfo
	if err := json.Unmarshal([]byte(response), &dishes); err != nil {
		return nil, fmt.Errorf("failed to parse dish extraction: %w", err)
	}

	return dishes, nil
}
