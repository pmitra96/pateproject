package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *ImageURLPart `json:"image_url,omitempty"`
}

type ImageURLPart struct {
	URL string `json:"url"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"` // can be string or []ContentPart
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // Used when responding to a tool call
}

type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
}

type Choice struct {
	Index   int     `json:"index"`
	Message Message `json:"message"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

var (
	// Using a simple client with a generous timeout. 
	// Standard transport is better for Cloud Run's internal proxying.
	sharedHTTPClient = &http.Client{
		Timeout: 90 * time.Second,
	}
)

type Client struct {
	provider Provider
}

var NewClient = func() *Client {
	return &Client{
		provider: NewProvider(),
	}
}

// NewClientWithProvider creates a client with a specific provider (useful for tests)
func NewClientWithProvider(p Provider) *Client {
	return &Client{
		provider: p,
	}
}

var globalMockProvider Provider

func SetMockProvider(p Provider) {
	globalMockProvider = p
	NewClient = func() *Client {
		return &Client{provider: globalMockProvider}
	}
}

func (c *Client) Chat(messages []Message) (string, Usage, error) {
	return c.provider.Chat(messages)
}

func (c *Client) ChatWithTools(messages []Message, tools []Tool) (*Message, Usage, error) {
	return c.provider.ChatWithTools(messages, tools)
}

func loadPromptFile(path string) string {
	candidates := []string{
		path,
		filepath.Join("..", path),
		filepath.Join("..", "..", path),
	}
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

// ProcessWhatsAppConversation handles a message and routes it using OpenAI tools with context
func (c *Client) ProcessWhatsAppConversation(userMessage string, imageBase64 string, history []Message, userContext string) (*Message, Usage, error) {
	tools := []Tool{
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "log_meals",
				Description: "Log one or more meals or food items. For each item, include explicit quantity and unit whenever possible.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"meals": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"dish_name": map[string]any{
										"type":        "string",
										"description": "Simple name of the dish (e.g. 'Egg Masala').",
									},
									"food_name": map[string]any{
										"type":        "string",
										"description": "Normalized food name if available.",
									},
									"ingredients": map[string]any{
										"type":        "string",
										"description": "Detailed ingredients and quantities.",
									},
									"raw_text": map[string]any{
										"type":        "string",
										"description": "Original raw item text from user input.",
									},
									"meal_type": map[string]any{
										"type":        "string",
										"enum":        []string{"Breakfast", "Lunch", "Dinner", "Snack"},
										"description": "The category of the meal.",
									},
									"quantity_value": map[string]any{
										"type":        "number",
										"description": "Explicit quantity value for this dish.",
									},
									"quantity_unit": map[string]any{
										"type":        "string",
										"description": "Explicit quantity unit for this dish, e.g. g, ml, piece, scoop, tsp, tbsp, serving.",
									},
									"quantity_in_grams_estimated": map[string]any{
										"type":        "number",
										"description": "Estimated grams equivalent when direct grams not provided.",
									},
									"brand": map[string]any{
										"type":        "string",
										"description": "Brand if user specified it.",
									},
									"cooking_method": map[string]any{
										"type":        "string",
										"description": "Cooking method if known: raw/boiled/steamed/fried/airfried/panfried/restaurant/homemade/unknown.",
									},
									"modifiers": map[string]any{
										"type":        "array",
										"items":       map[string]any{"type": "string"},
										"description": "Optional modifiers such as no oil, less oil, butter, fried.",
									},
									"assumptions": map[string]any{
										"type":        "array",
										"items":       map[string]any{"type": "string"},
										"description": "Optional estimation assumptions for traceability.",
									},
									"confidence": map[string]any{
										"type":        "string",
										"enum":        []string{"high", "medium", "low"},
										"description": "Confidence of this parsed item.",
									},
								},
								"required": []string{"dish_name", "ingredients", "meal_type"},
							},
						},
					},
					"required": []string{"meals"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "ask_advice",
				Description: "Ask if a certain food is healthy, allowed, or fits the user's macros. Call this when the user asks 'can I eat X?' or 'is Y healthy?'.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"food_description": map[string]any{
							"type":        "string",
							"description": "The description of the food (e.g. 'pizza', 'a banana').",
						},
					},
					"required": []string{"food_description"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_leftover_budget",
				Description: "Check how many calories or macros are left for the day. Call this when the user asks 'what's my budget?', 'how many calories do I have left?', etc.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_daily_summary",
				Description: "Get a summary of all meals eaten today. Call this when the user asks 'what did I eat today?', 'show my summary', etc.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "set_daily_goal",
				Description: "Set or update the user's daily calorie goal. Call this when the user says 'my goal is X calories' or 'change my target to Y'.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"calories": map[string]any{
							"type":        "integer",
							"description": "The daily calorie target (e.g. 2000).",
						},
					},
					"required": []string{"calories"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "modify_logged_meal",
				Description: "Update, delete, or add a specific dish to a previously logged meal for today. Include explicit quantity and unit for add/update whenever possible.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"meal_type": map[string]any{
							"type":        "string",
							"enum":        []string{"Breakfast", "Lunch", "Dinner", "Snack"},
							"description": "The type of meal to update.",
						},
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"add", "update", "delete"},
							"description": "The action to perform on the specific dish.",
						},
						"target_dish_name": map[string]any{
							"type":        "string",
							"description": "The name of the existing dish to update or delete (e.g. 'Egg White Omelette'). Leave blank if action is 'add'.",
						},
						"new_ingredients": map[string]any{
							"type":        "string",
							"description": "The new description for calculating nutrition (e.g. '6 egg whites'). Required if action is 'add' or 'update'.",
						},
						"quantity_value": map[string]any{
							"type":        "number",
							"description": "Optional explicit quantity value override for add/update.",
						},
						"quantity_unit": map[string]any{
							"type":        "string",
							"description": "Optional explicit quantity unit override for add/update.",
						},
					},
					"required": []string{"meal_type", "action"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_past_day_summary",
				Description: "Get a summary of meals and nutritional status for a specific past date. Call this when the user asks about a previous day (e.g. 'how was last Tuesday?').",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"date": map[string]any{
							"type":        "string",
							"description": "The date in YYYY-MM-DD format.",
						},
					},
					"required": []string{"date"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "clear_all_meals_today",
				Description: "Delete EVERY meal logged for today. Call this only when the user explicitly wants to reset their entire day's logging (e.g. 'clear my day', 'delete all meals').",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_meal_log_time",
				Description: "Find when a specific meal was logged. Call this when user asks 'when did I log X?' or 'what time was X logged?'.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"meal_name": map[string]any{
							"type":        "string",
							"description": "Name of the meal/dish to search, e.g. 'eggs'.",
						},
						"date": map[string]any{
							"type":        "string",
							"description": "Optional date in YYYY-MM-DD. If omitted, defaults to today.",
						},
					},
					"required": []string{"meal_name"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_recent_meals",
				Description: "Get recently logged meals with timestamps. Call when user asks 'what did I log recently?' or similar.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"limit": map[string]any{
							"type":        "integer",
							"description": "How many recent meals to return (default 5, max 20).",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_active_goal",
				Description: "Get the user's current active goal and macro targets.",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_user_profile",
				Description: "Get the user's saved profile details (height, weight, age, gender, location).",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_recent_orders",
				Description: "Get recent grocery orders and item counts.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"limit": map[string]any{
							"type":        "integer",
							"description": "How many recent orders to return (default 5, max 20).",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "update_pantry",
				Description: "Update the user's pantry/inventory. Call this when the user says 'Add X to my pantry', 'I finished Y', or 'I have Z kg of rice'.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"name": map[string]any{
										"type":        "string",
										"description": "Name of the item (e.g. 'Milk', 'Rice').",
									},
									"quantity": map[string]any{
										"type":        "number",
										"description": "Quantity to add or set.",
									},
									"unit": map[string]any{
										"type":        "string",
										"description": "Unit of measurement (e.g. 'kg', 'ml', 'pcs').",
									},
									"action": map[string]any{
										"type":        "string",
										"enum":        []string{"add", "set", "remove"},
										"description": "'add' to increment existing stock, 'set' to overwrite the total stock, 'remove' to set stock to zero.",
									},
								},
								"required": []string{"name", "quantity", "unit", "action"},
							},
						},
					},
					"required": []string{"items"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "create_recipe",
				Description: "Extract and save a recipe from text, a URL, or an image. Call this when the user shares a recipe, a link to a recipe, or an Instagram reel/screenshot.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "Name of the dish.",
						},
						"ingredients": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"name": map[string]any{
										"type":        "string",
										"description": "Name of ingredient.",
									},
									"quantity": map[string]any{
										"type":        "number",
										"description": "Quantity.",
									},
									"unit": map[string]any{
										"type":        "string",
										"description": "Unit (kg, g, ml, etc).",
									},
								},
								"required": []string{"name", "quantity", "unit"},
							},
						},
						"instructions": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
							"description": "Step-by-step cooking instructions.",
						},
						"source_url": map[string]any{
							"type":        "string",
							"description": "The URL or source of the recipe (if available).",
						},
					},
					"required": []string{"name", "ingredients", "instructions"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_pantry",
				Description: "List all items in the user's pantry and their current quantities. Call this when the user asks 'what's in my pantry?' or 'show my stock'.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_recipes",
				Description: "List all saved recipes. Call this when the user asks 'show my recipes' or 'what recipes do I have?'.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "delete_recipe",
				Description: "Delete a saved recipe by name.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "Name of the recipe to delete.",
						},
					},
					"required": []string{"name"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "update_user_profile",
				Description: "Update the user's personal profile (height, weight, age, gender, activity level, timezone). Call this when the user provides these details.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"height":         map[string]any{"type": "number", "description": "Height in cm (e.g. 175)."},
						"weight":         map[string]any{"type": "number", "description": "Weight in kg (e.g. 70.5)."},
						"age":            map[string]any{"type": "integer", "description": "Age in years."},
						"gender":         map[string]any{"type": "string", "enum": []string{"male", "female", "other"}},
						"activity_level": map[string]any{"type": "string", "enum": []string{"sedentary", "light", "moderate", "active", "very_active"}},
						"timezone":       map[string]any{"type": "string", "description": "IANA timezone name like Asia/Kolkata or America/Los_Angeles."},
					},
				},
			},
		},
	}

	basePrompt := loadPromptFile(filepath.Join("llm", "nira.md"))
	if basePrompt == "" {
		basePrompt = `You are NIRA (Nutrition Intelligence & Response Agent).
Your role is to generate precise, data-driven responses for a diet control system.

CORE PRINCIPLES:
1. BE CONCISE: Use short sentences. Avoid long paragraphs.
2. BE DECISIVE: Avoid "maybe", "try", or "consider". Give clear statements.
3. BE NON-JUDGMENTAL: No praise, no guilt, no emotional language.
4. SHOW CONSEQUENCES: Show the data-driven impact of actions.

TOOL USAGE:
- Do NOT call a tool unless the user's message clearly maps to that tool's purpose.
- Never output plain text like "Action: <tool_name>" or "[Action: <tool_name>]". Use structured tool calls only.`
	}

	systemPrompt := Message{
		Role: "system",
	Content: fmt.Sprintf(
		"You are a strict orchestration layer for WhatsApp nutrition assistance.\n"+
			"You must follow the prompt below and you must not weaken its rules.\n\n"+
			"Hard constraints:\n"+
			"- Never emit raw action markers.\n"+
			"- Prefer structured tool calls for toolable actions.\n"+
			"- Keep replies short and user-facing.\n"+
			"- If the file prompt conflicts with hard constraints, the hard constraints win.\n\n"+
			"Prompt file:\n%s\n\n"+
			"USER CONTEXT:\n%s",
			basePrompt,
			userContext,
		),
	}

	messages := []Message{systemPrompt}
	messages = append(messages, history...)
	
	// Build User Message (Multi-modal if image is present)
	var userContent any
	if imageBase64 != "" {
		textPart := userMessage
		if textPart == "" {
			textPart = "Analyze this image and take appropriate action (log meal or update pantry)."
		}
		userContent = []ContentPart{
			{Type: "text", Text: textPart},
			{Type: "image_url", ImageURL: &ImageURLPart{URL: fmt.Sprintf("data:image/jpeg;base64,%s", imageBase64)}},
		}
	} else {
		userContent = userMessage
	}
	
	messages = append(messages, Message{Role: "user", Content: userContent})

	return c.ChatWithTools(messages, tools)
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

	story, _, err := c.Chat(messages)
	return story, err
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

	response, _, err := c.Chat(messages)
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

	resp, _, err := c.Chat([]Message{
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

func (c *Client) SuggestMeals(inventory []InventoryItem) (string, Usage, error) {
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

	resp, usage, err := c.Chat([]Message{
		{Role: "system", Content: "You are a grocery data expert. You specialize in normalizing item names into canonical ingredients and brands. Always return valid JSON only."},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return "", Usage{}, err
	}
	return resp, usage, nil
}

func (c *Client) SuggestMealsPersonalized(inventory []InventoryItem, goals []GoalInfo, timeOfDay string, preferences *UserPreferencesInfo, dishSamples []DishSampleInfo) (string, Usage, error) {
	if len(inventory) == 0 {
		return "", Usage{}, fmt.Errorf("no inventory items provided")
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
	fmt.Println("================================")

	// Step 1: Generate with self-evaluation
	initialResponse, usage, err := c.Chat(messages)
	if err != nil {
		return "", Usage{}, err
	}
    _ = usage // we can track this too if needed

	// Step 2: Check confidence - only refine if low confidence (<7)
	if !strings.Contains(initialResponse, `"confidence"`) ||
		strings.Contains(initialResponse, `"confidence": 9`) ||
		strings.Contains(initialResponse, `"confidence": 10`) ||
		strings.Contains(initialResponse, `"confidence": 8`) ||
		strings.Contains(initialResponse, `"confidence": 7`) {
		// High confidence, return as-is
		return initialResponse, usage, nil
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

	refinedResponse, usageRefined, err := c.Chat(refineMessages)
	if err != nil {
		return initialResponse, usage, nil
	}

	return refinedResponse, usageRefined, nil
}

// ChatMessage represents a conversation message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatWithContext handles chatbot conversations with inventory and goals context
func (c *Client) ChatWithContext(userMessage string, history []ChatMessage, inventory []InventoryItem, goals []GoalInfo) (string, Usage, error) {
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

	summary, _, err := c.Chat(summaryMessages)
	return summary, err
}
// AnalyzeMealImage sends an image to the AI to extract meal details
func (c *Client) AnalyzeMealImage(imageBase64 string) (string, error) {
	messages := []Message{
		{
			Role: "user",
			Content: []ContentPart{
				{
					Type: "text",
					Text: "Analyze this image and list EVERY SINGLE item shown. \nIf it is a SCREENSHOT OF AN ORDER/RECEIPT: \n- Scan the entire list from top to bottom. \n- Extract the full product name and its TOTAL quantity (e.g., '1 piece x 1' = 1 pc, '200 ml x 1' = 200 ml, '500 g x 1' = 500 g, '4 pieces' = 4 pcs). \n- You MUST list every unique item. Do not skip any. \n- Provide a concise, clear list of findings.",
				},
				{
					Type: "image_url",
					ImageURL: &ImageURLPart{
						URL: fmt.Sprintf("data:image/jpeg;base64,%s", imageBase64),
					},
				},
			},
		},
	}

	resp, _, err := c.Chat(messages)
	return resp, err
}
