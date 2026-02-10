package websearch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/config"
)

type SearchResult struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Snippet     string `json:"snippet"`
	DisplayLink string `json:"display_link"`
}

type DishSearchResult struct {
	DishName    string   `json:"dish_name"`
	Description string   `json:"description"`
	Cuisine     string   `json:"cuisine"`
	Source      string   `json:"source"`
	Ingredients []string `json:"ingredients,omitempty"`
}

type Client struct {
	apiKey     string
	searchType string
	httpClient *http.Client
}

func NewClient() *Client {
	searchType := config.GetEnv("WEBSEARCH_TYPE", "duckduckgo")
	return &Client{
		apiKey:     config.GetEnv("SERPAPI_KEY", ""),
		searchType: searchType,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) IsConfigured() bool {
	// DuckDuckGo doesn't need an API key
	if c.searchType == "duckduckgo" {
		return true
	}
	return c.apiKey != ""
}

func (c *Client) SearchDishes(location, cuisine string, limit int) ([]DishSearchResult, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("web search API key not configured")
	}

	if limit <= 0 {
		limit = 10
	}

	query := buildDishSearchQuery(location, cuisine)

	switch c.searchType {
	case "serpapi":
		return c.searchWithSerpAPI(query, location, limit)
	case "duckduckgo":
		return c.searchWithDuckDuckGo(query, limit)
	default:
		return c.searchWithDuckDuckGo(query, limit)
	}
}

func buildDishSearchQuery(location, cuisine string) string {
	parts := []string{}

	if cuisine != "" {
		parts = append(parts, cuisine+" dishes")
	} else {
		parts = append(parts, "popular dishes")
	}

	if location != "" {
		parts = append(parts, "in "+location)
	}

	parts = append(parts, "with recipes ingredients")

	return strings.Join(parts, " ")
}

type SerpAPIResponse struct {
	OrganicResults []struct {
		Title   string `json:"title"`
		Link    string `json:"link"`
		Snippet string `json:"snippet"`
	} `json:"organic_results"`
	Error string `json:"error,omitempty"`
}

func (c *Client) searchWithSerpAPI(query, location string, limit int) ([]DishSearchResult, error) {
	baseURL := "https://serpapi.com/search"

	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("q", query)
	params.Set("engine", "google")
	params.Set("num", fmt.Sprintf("%d", limit))
	if location != "" {
		params.Set("location", location)
	}

	reqURL := baseURL + "?" + params.Encode()

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search API error (status %d): %s", resp.StatusCode, string(body))
	}

	var serpResp SerpAPIResponse
	if err := json.Unmarshal(body, &serpResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if serpResp.Error != "" {
		return nil, fmt.Errorf("SerpAPI error: %s", serpResp.Error)
	}

	results := make([]DishSearchResult, 0, len(serpResp.OrganicResults))
	for _, r := range serpResp.OrganicResults {
		results = append(results, DishSearchResult{
			DishName:    extractDishName(r.Title),
			Description: r.Snippet,
			Cuisine:     cuisine(query),
			Source:      r.Link,
		})
	}

	return results, nil
}

func extractDishName(title string) string {
	separators := []string{" - ", " | ", ": ", " – "}
	for _, sep := range separators {
		if idx := strings.Index(title, sep); idx > 0 {
			return strings.TrimSpace(title[:idx])
		}
	}
	return strings.TrimSpace(title)
}

func cuisine(query string) string {
	lower := strings.ToLower(query)
	cuisines := []string{"indian", "south indian", "north indian", "chinese", "italian", "mexican", "thai", "japanese", "korean", "mediterranean", "american", "french", "continental"}
	for _, c := range cuisines {
		if strings.Contains(lower, c) {
			return strings.Title(c)
		}
	}
	return "Mixed"
}

func (c *Client) SearchDishesWithIngredients(location, cuisine string, pantryItems []string, limit int) ([]DishSearchResult, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("web search API key not configured")
	}

	if limit <= 0 {
		limit = 8
	}

	query := buildDishWithIngredientsQuery(location, cuisine, pantryItems)

	switch c.searchType {
	case "serpapi":
		return c.searchWithSerpAPI(query, location, limit)
	case "duckduckgo":
		return c.searchWithDuckDuckGo(query, limit)
	default:
		return c.searchWithDuckDuckGo(query, limit)
	}
}

// DuckDuckGo search (no API key required)
type DuckDuckGoResponse struct {
	RelatedTopics []struct {
		Text      string `json:"Text"`
		FirstURL  string `json:"FirstURL"`
		Result    string `json:"Result"`
	} `json:"RelatedTopics"`
	Abstract     string `json:"Abstract"`
	AbstractText string `json:"AbstractText"`
}

func (c *Client) searchWithDuckDuckGo(query string, limit int) ([]DishSearchResult, error) {
	// DuckDuckGo Instant Answer API
	baseURL := "https://api.duckduckgo.com/"
	
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("no_redirect", "1")
	params.Set("no_html", "1")
	
	reqURL := baseURL + "?" + params.Encode()
	
	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("DuckDuckGo request failed: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var ddgResp DuckDuckGoResponse
	if err := json.Unmarshal(body, &ddgResp); err != nil {
		return nil, fmt.Errorf("failed to parse DuckDuckGo response: %w", err)
	}
	
	results := make([]DishSearchResult, 0)
	
	// Add abstract if available
	if ddgResp.AbstractText != "" {
		results = append(results, DishSearchResult{
			DishName:    extractDishName(query),
			Description: ddgResp.AbstractText,
			Cuisine:     cuisine(query),
			Source:      "DuckDuckGo",
		})
	}
	
	// Add related topics
	for _, topic := range ddgResp.RelatedTopics {
		if topic.Text != "" && len(results) < limit {
			results = append(results, DishSearchResult{
				DishName:    extractDishName(topic.Text),
				Description: topic.Text,
				Cuisine:     cuisine(query),
				Source:      topic.FirstURL,
			})
		}
	}
	
	// If DuckDuckGo returns limited results, generate dish suggestions based on cuisine
	if len(results) < 3 {
		results = append(results, generateFallbackDishes(query)...)
	}
	
	if len(results) > limit {
		results = results[:limit]
	}
	
	return results, nil
}

// generateFallbackDishes creates dish suggestions when search returns limited results
func generateFallbackDishes(query string) []DishSearchResult {
	lower := strings.ToLower(query)
	var dishes []DishSearchResult
	
	if strings.Contains(lower, "italian") {
		dishes = []DishSearchResult{
			{DishName: "Pasta Primavera", Description: "Light pasta with fresh vegetables, olive oil and herbs", Cuisine: "Italian", Source: "web_fallback"},
			{DishName: "Caprese Salad", Description: "Fresh tomatoes, mozzarella, basil with olive oil and balsamic", Cuisine: "Italian", Source: "web_fallback"},
			{DishName: "Bruschetta", Description: "Toasted bread topped with tomatoes, garlic, basil and olive oil", Cuisine: "Italian", Source: "web_fallback"},
			{DishName: "Minestrone Soup", Description: "Hearty vegetable soup with beans and pasta", Cuisine: "Italian", Source: "web_fallback"},
			{DishName: "Risotto", Description: "Creamy rice dish cooked with broth and vegetables", Cuisine: "Italian", Source: "web_fallback"},
		}
	} else if strings.Contains(lower, "mexican") {
		dishes = []DishSearchResult{
			{DishName: "Vegetable Tacos", Description: "Soft tortillas with grilled vegetables and salsa", Cuisine: "Mexican", Source: "web_fallback"},
			{DishName: "Guacamole", Description: "Fresh avocado dip with lime, cilantro and tomatoes", Cuisine: "Mexican", Source: "web_fallback"},
			{DishName: "Burrito Bowl", Description: "Rice bowl with beans, vegetables, salsa and cheese", Cuisine: "Mexican", Source: "web_fallback"},
		}
	} else if strings.Contains(lower, "chinese") {
		dishes = []DishSearchResult{
			{DishName: "Vegetable Stir Fry", Description: "Mixed vegetables stir fried in soy sauce and ginger", Cuisine: "Chinese", Source: "web_fallback"},
			{DishName: "Fried Rice", Description: "Rice stir fried with vegetables, egg and soy sauce", Cuisine: "Chinese", Source: "web_fallback"},
			{DishName: "Hot and Sour Soup", Description: "Spicy and tangy soup with vegetables and tofu", Cuisine: "Chinese", Source: "web_fallback"},
		}
	} else if strings.Contains(lower, "thai") {
		dishes = []DishSearchResult{
			{DishName: "Pad Thai", Description: "Stir fried rice noodles with vegetables and peanuts", Cuisine: "Thai", Source: "web_fallback"},
			{DishName: "Green Curry", Description: "Coconut milk curry with vegetables and Thai basil", Cuisine: "Thai", Source: "web_fallback"},
			{DishName: "Tom Yum Soup", Description: "Spicy and sour soup with lemongrass and mushrooms", Cuisine: "Thai", Source: "web_fallback"},
		}
	} else if strings.Contains(lower, "japanese") {
		dishes = []DishSearchResult{
			{DishName: "Miso Soup", Description: "Traditional soup with tofu, seaweed and miso paste", Cuisine: "Japanese", Source: "web_fallback"},
			{DishName: "Vegetable Tempura", Description: "Light battered and fried vegetables", Cuisine: "Japanese", Source: "web_fallback"},
			{DishName: "Edamame", Description: "Steamed soybeans with sea salt", Cuisine: "Japanese", Source: "web_fallback"},
		}
	} else if strings.Contains(lower, "mediterranean") {
		dishes = []DishSearchResult{
			{DishName: "Greek Salad", Description: "Fresh vegetables with feta cheese and olives", Cuisine: "Mediterranean", Source: "web_fallback"},
			{DishName: "Hummus", Description: "Chickpea dip with tahini, lemon and garlic", Cuisine: "Mediterranean", Source: "web_fallback"},
			{DishName: "Falafel", Description: "Fried chickpea patties with herbs and spices", Cuisine: "Mediterranean", Source: "web_fallback"},
		}
	} else {
		// Generic healthy dishes
		dishes = []DishSearchResult{
			{DishName: "Vegetable Stir Fry", Description: "Mixed vegetables sauteed with garlic and soy sauce", Cuisine: "Asian Fusion", Source: "web_fallback"},
			{DishName: "Garden Salad", Description: "Fresh mixed greens with vegetables and light dressing", Cuisine: "International", Source: "web_fallback"},
			{DishName: "Vegetable Soup", Description: "Hearty soup with seasonal vegetables", Cuisine: "International", Source: "web_fallback"},
		}
	}
	
	return dishes
}

func buildDishWithIngredientsQuery(location, cuisine string, pantryItems []string) string {
	parts := []string{}

	if cuisine != "" {
		parts = append(parts, cuisine+" recipes")
	} else {
		parts = append(parts, "recipes")
	}

	if len(pantryItems) > 0 {
		topItems := pantryItems
		if len(topItems) > 5 {
			topItems = topItems[:5]
		}
		parts = append(parts, "with "+strings.Join(topItems, " "))
	}

	if location != "" {
		parts = append(parts, "popular in "+location)
	}

	return strings.Join(parts, " ")
}
