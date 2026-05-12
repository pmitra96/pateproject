package tests

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/pmitra96/pateproject/controllers"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/llm"
	"github.com/pmitra96/pateproject/models"
	"github.com/pmitra96/pateproject/services/whatsapp"
	"github.com/stretchr/testify/assert"
)

func runWhatsAppIntent(user *models.User, text string) string {
	mockClient := &whatsapp.MockClient{}
	sess := whatsapp.NewSession(user, "test-msg-id", slog.Default())
	sess.Client = mockClient

	orch := whatsapp.NewOrchestrator()
	orch.ProcessMessage(sess, text, "")
	return mockClient.LastMessage
}

// MockProvider implements llm.Provider for testing
type MockProvider struct {
	ChatFunc          func(messages []llm.Message) (string, llm.Usage, error)
	ChatWithToolsFunc func(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error)
}

func (m *MockProvider) Chat(messages []llm.Message) (string, llm.Usage, error) {
	if m.ChatFunc != nil {
		return m.ChatFunc(messages)
	}
	return "Mock response", llm.Usage{}, nil
}

func (m *MockProvider) ChatWithTools(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error) {
	if m.ChatWithToolsFunc != nil {
		return m.ChatWithToolsFunc(messages, tools)
	}
	return &llm.Message{Role: "assistant", Content: "Mock response"}, llm.Usage{}, nil
}

func TestWhatsAppLLMIntegration(t *testing.T) {
	setupTestDB()

	mockProvider := &MockProvider{}
	llm.SetMockProvider(mockProvider)

	setupUserWithGoal := func(phone string, calories int) *models.User {
		user, _ := controllers.GetOrCreateWhatsAppUser(phone)
		if calories > 0 {
			var goal models.Goal
			database.DB.Where("user_id = ? AND is_active = ?", user.ID, true).First(&goal)
			database.DB.Model(&models.GoalMacroProfile{}).Where("goal_id = ?", goal.ID).Update("daily_calorie_target", calories)
		}
		return user
	}

	t.Run("TC_GOAL_02: Set daily goal via WhatsApp", func(t *testing.T) {
		user := setupUserWithGoal("101", 2000)

		mockProvider.ChatWithToolsFunc = func(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error) {
			args := `{"calories": 1800}`
			return &llm.Message{
				Role:      "assistant",
				ToolCalls: []llm.ToolCall{{Function: llm.ToolCallFunction{Name: "set_daily_goal", Arguments: args}}},
			}, llm.Usage{}, nil
		}
		mockProvider.ChatFunc = func(messages []llm.Message) (string, llm.Usage, error) {
			return "Goal updated to 1800 calories.", llm.Usage{}, nil
		}

		resp := runWhatsAppIntent(user, "My goal is 1800 calories")
		assert.Contains(t, resp, "1800")

		var profile models.GoalMacroProfile
		database.DB.Joins("JOIN goals ON goals.id = goal_macro_profiles.goal_id").
			Where("goals.user_id = ? AND goals.is_active = ?", user.ID, true).First(&profile)
		assert.Equal(t, 1800, profile.DailyCalorieTarget)
	})

	t.Run("TC_LOG_04: Correct a previously logged meal", func(t *testing.T) {
		user := setupUserWithGoal("102", 2000)

		meal := models.MealLog{UserID: user.ID, Name: "Eggs", MealType: "Breakfast", Calories: 140, LoggedAt: time.Now()}
		database.DB.Create(&meal)

		mockProvider.ChatWithToolsFunc = func(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error) {
			args := `{"meals":[{"dish_name":"Eggs", "ingredients":"3 eggs", "meal_type":"Breakfast"}]}`
			return &llm.Message{
				Role:      "assistant",
				ToolCalls: []llm.ToolCall{{Function: llm.ToolCallFunction{Name: "log_meals", Arguments: args}}},
			}, llm.Usage{}, nil
		}
		mockProvider.ChatFunc = func(messages []llm.Message) (string, llm.Usage, error) {
			last := messages[len(messages)-1]
			if s, ok := last.Content.(string); ok && strings.Contains(s, "TOOL RESULTS JSON:") {
				return "Logged Eggs at 210 kcal.", llm.Usage{}, nil
			}
			return `{"calories": 210, "protein": 18, "carbs": 1, "fat": 15}`, llm.Usage{}, nil
		}

		resp := runWhatsAppIntent(user, "Actually I had 3 eggs")
		assert.Contains(t, resp, "210")
	})

	t.Run("TC_ADV_03: Protein prioritized in Damage Control", func(t *testing.T) {
		user := setupUserWithGoal("103", 2000)
		meal := models.MealLog{UserID: user.ID, Name: "Huge Pizza", Calories: 1800, LoggedAt: time.Now()}
		database.DB.Create(&meal)

		mockProvider.ChatWithToolsFunc = func(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error) {
			args := `{"food_description":"protein shake"}`
			return &llm.Message{
				Role:      "assistant",
				ToolCalls: []llm.ToolCall{{Function: llm.ToolCallFunction{Name: "ask_advice", Arguments: args}}},
			}, llm.Usage{}, nil
		}
		mockProvider.ChatFunc = func(messages []llm.Message) (string, llm.Usage, error) {
			last := messages[len(messages)-1]
			if s, ok := last.Content.(string); ok && strings.Contains(s, "TOOL RESULTS JSON:") {
				return "Yes, you can eat this now.", llm.Usage{}, nil
			}
			return `{"calories": 150, "protein": 25, "carbs": 5, "fat": 2}`, llm.Usage{}, nil
		}

		resp := runWhatsAppIntent(user, "Can I have a protein shake?")
		assert.Contains(t, resp, "Yes, you can eat this")
	})

	t.Run("TC_PAN_01: Add items to pantry via text", func(t *testing.T) {
		user := setupUserWithGoal("104", 2000)

		mockProvider.ChatWithToolsFunc = func(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error) {
			args := `{"items":[{"name":"Basmati Rice", "quantity": 2, "unit": "kg", "action": "add"}]}`
			return &llm.Message{
				Role:      "assistant",
				ToolCalls: []llm.ToolCall{{Function: llm.ToolCallFunction{Name: "update_pantry", Arguments: args}}},
			}, llm.Usage{}, nil
		}
		mockProvider.ChatFunc = func(messages []llm.Message) (string, llm.Usage, error) {
			last := messages[len(messages)-1]
			if s, ok := last.Content.(string); ok && strings.Contains(s, "TOOL RESULTS JSON:") {
				return "Pantry updated with rice.", llm.Usage{}, nil
			}
			return `[{"ingredient": "Rice", "brand": null, "product": "Basmati Rice"}]`, llm.Usage{}, nil
		}

		resp := runWhatsAppIntent(user, "Add 2kg rice to my pantry")
		assert.Contains(t, resp, "Pantry")

		var pi models.PantryItem
		database.DB.Joins("JOIN ingredients ON ingredients.id = pantry_items.ingredient_id").
			Where("pantry_items.user_id = ? AND LOWER(ingredients.name) = ?", user.ID, "rice").First(&pi)
		assert.NotZero(t, pi.ID)
	})

	t.Run("TC_QRY_01: Check remaining budget", func(t *testing.T) {
		user := setupUserWithGoal("105", 2000)

		mockProvider.ChatWithToolsFunc = func(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error) {
			return &llm.Message{
				Role:      "assistant",
				ToolCalls: []llm.ToolCall{{Function: llm.ToolCallFunction{Name: "get_leftover_budget", Arguments: "{}"}}},
			}, llm.Usage{}, nil
		}
		mockProvider.ChatFunc = func(messages []llm.Message) (string, llm.Usage, error) {
			return "You have 2000 calories remaining.", llm.Usage{}, nil
		}

		resp := runWhatsAppIntent(user, "What's my budget?")
		assert.Contains(t, resp, "2000")
	})

	t.Run("TC_SUM_01: Log meal then ask daily summary", func(t *testing.T) {
		user := setupUserWithGoal("106", 2000)

		mockProvider.ChatWithToolsFunc = func(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error) {
			last := messages[len(messages)-1]
			userText, _ := last.Content.(string)
			if strings.Contains(strings.ToLower(userText), "what all did i eat today") {
				return &llm.Message{
					Role:      "assistant",
					ToolCalls: []llm.ToolCall{{Function: llm.ToolCallFunction{Name: "get_daily_summary", Arguments: "{}"}}},
				}, llm.Usage{}, nil
			}
			args := `{"meals":[{"dish_name":"Oats Bowl", "ingredients":"60g oats with milk", "meal_type":"Breakfast"}]}`
			return &llm.Message{
				Role:      "assistant",
				ToolCalls: []llm.ToolCall{{Function: llm.ToolCallFunction{Name: "log_meals", Arguments: args}}},
			}, llm.Usage{}, nil
		}
		mockProvider.ChatFunc = func(messages []llm.Message) (string, llm.Usage, error) {
			last := messages[len(messages)-1]
			if s, ok := last.Content.(string); ok && strings.Contains(s, "TOOL RESULTS JSON:") {
				return "Logged.", llm.Usage{}, nil
			}
			return `{"calories": 320, "protein": 14, "carbs": 50, "fat": 8}`, llm.Usage{}, nil
		}

		logResp := runWhatsAppIntent(user, "I ate oats bowl for breakfast")
		assert.Contains(t, strings.ToLower(logResp), "logged")

		summaryResp := runWhatsAppIntent(user, "what all did i eat today")
		assert.Contains(t, summaryResp, "Today's summary")
		assert.Contains(t, summaryResp, "Oats Bowl")
	})

	t.Run("TC_SUM_02: WhatsApp text redirects to get_daily_summary tool", func(t *testing.T) {
		user := setupUserWithGoal("107", 2000)
		var calledTools []string

		mockProvider.ChatWithToolsFunc = func(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error) {
			last := messages[len(messages)-1]
			userText, _ := last.Content.(string)
			lower := strings.ToLower(userText)

			if strings.Contains(lower, "i ate") {
				args := `{"meals":[{"dish_name":"Dal Rice", "ingredients":"1 bowl dal and 1 cup rice", "meal_type":"Lunch"}]}`
				calledTools = append(calledTools, "log_meals")
				return &llm.Message{
					Role:      "assistant",
					ToolCalls: []llm.ToolCall{{Function: llm.ToolCallFunction{Name: "log_meals", Arguments: args}}},
				}, llm.Usage{}, nil
			}

			if strings.Contains(lower, "what all did i eat today") {
				calledTools = append(calledTools, "get_daily_summary")
				return &llm.Message{
					Role:      "assistant",
					ToolCalls: []llm.ToolCall{{Function: llm.ToolCallFunction{Name: "get_daily_summary", Arguments: "{}"}}},
				}, llm.Usage{}, nil
			}

			return &llm.Message{Role: "assistant", Content: "Can you clarify?"}, llm.Usage{}, nil
		}
		mockProvider.ChatFunc = func(messages []llm.Message) (string, llm.Usage, error) {
			last := messages[len(messages)-1]
			if s, ok := last.Content.(string); ok && strings.Contains(s, "TOOL RESULTS JSON:") {
				return "Summary generated.", llm.Usage{}, nil
			}
			return `{"calories": 520, "protein": 16, "carbs": 82, "fat": 11}`, llm.Usage{}, nil
		}

		_ = runWhatsAppIntent(user, "I ate dal rice for lunch")
		resp := runWhatsAppIntent(user, "what all did i eat today")

		assert.True(t, strings.Contains(strings.Join(calledTools, ","), "get_daily_summary") || len(calledTools) <= 1, "Expected summary to be served via tool path (LLM or deterministic router)")
		assert.Contains(t, resp, "Today's summary")
		assert.Contains(t, resp, "Dal Rice")
		assert.NotContains(t, strings.ToLower(resp), "action:")

		var st models.ConversationState
		database.DB.Where("user_id = ?", user.ID).First(&st)
		assert.Equal(t, "daily_summary", st.LastIntent)
		assert.Equal(t, "get_daily_summary", st.LastTool)
	})

	t.Run("TC_V2_01: Greeting handled deterministically", func(t *testing.T) {
		user := setupUserWithGoal("110", 2000)
		called := false
		mockProvider.ChatWithToolsFunc = func(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error) {
			called = true
			return &llm.Message{Role: "assistant", Content: "This should not be called"}, llm.Usage{}, nil
		}
		resp := runWhatsAppIntent(user, "hey")
		assert.Contains(t, strings.ToLower(resp), "hello")
		assert.False(t, called, "LLM should not be called for deterministic greeting route")
	})

	t.Run("TC_SUM_03: Action text fallback executes get_daily_summary", func(t *testing.T) {
		user := setupUserWithGoal("108", 2000)
		database.DB.Create(&models.MealLog{
			UserID: user.ID, Name: "Curd Rice", MealType: "Lunch",
			Ingredients: "1 bowl curd rice", Calories: 420, Protein: 10, Carbs: 62, Fat: 14, LoggedAt: time.Now(),
		})

		mockProvider.ChatWithToolsFunc = func(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error) {
			return &llm.Message{Role: "assistant", Content: "Action: get_daily_summary"}, llm.Usage{}, nil
		}
		mockProvider.ChatFunc = func(messages []llm.Message) (string, llm.Usage, error) {
			return "Ignored", llm.Usage{}, nil
		}

		resp := runWhatsAppIntent(user, "what all did i eat today")
		assert.Contains(t, resp, "Today's summary")
		assert.Contains(t, resp, "Curd Rice")
		assert.NotContains(t, strings.ToLower(resp), "action:")

		var conv models.Conversation
		database.DB.Where("user_id = ?", user.ID).First(&conv)
		assert.NotContains(t, strings.ToLower(conv.Messages), "[action:")
	})

	t.Run("TC_SUM_04: Deterministic router handles summary intent if model misses tool call", func(t *testing.T) {
		user := setupUserWithGoal("109", 2000)
		database.DB.Create(&models.MealLog{
			UserID: user.ID, Name: "Idli", MealType: "Breakfast",
			Ingredients: "2 idli", Calories: 180, Protein: 6, Carbs: 32, Fat: 2, LoggedAt: time.Now(),
		})

		mockProvider.ChatWithToolsFunc = func(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error) {
			return &llm.Message{Role: "assistant", Content: "Sure, let me check."}, llm.Usage{}, nil
		}
		mockProvider.ChatFunc = func(messages []llm.Message) (string, llm.Usage, error) {
			return "Ignored", llm.Usage{}, nil
		}

		resp := runWhatsAppIntent(user, "what all did i eat today")
		assert.Contains(t, resp, "Today's summary")
		assert.Contains(t, resp, "Idli")
		assert.NotContains(t, strings.ToLower(resp), "action:")
	})

	t.Run("TC_PHASE1_01: Duplicate meal logs always create new entries", func(t *testing.T) {
		user := setupUserWithGoal("201", 2000)

		mockProvider.ChatWithToolsFunc = func(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error) {
			args := `{"meals":[{"dish_name":"Papaya","ingredients":"100g papaya","meal_type":"Breakfast"}]}`
			return &llm.Message{
				Role:      "assistant",
				ToolCalls: []llm.ToolCall{{Function: llm.ToolCallFunction{Name: "log_meals", Arguments: args}}},
			}, llm.Usage{}, nil
		}
		mockProvider.ChatFunc = func(messages []llm.Message) (string, llm.Usage, error) {
			last := messages[len(messages)-1]
			if s, ok := last.Content.(string); ok && strings.Contains(s, "TOOL RESULTS JSON:") {
				return "Logged.", llm.Usage{}, nil
			}
			return `{"calories": 43, "protein": 0.5, "carbs": 11, "fat": 0.1, "fiber": 1.7}`, llm.Usage{}, nil
		}

		_ = runWhatsAppIntent(user, "papaya")
		_ = runWhatsAppIntent(user, "papaya")

		var count int64
		database.DB.Model(&models.MealLog{}).
			Where("user_id = ? AND LOWER(name) = ? AND LOWER(ingredients) = ?", user.ID, "papaya", "100g papaya").
			Count(&count)
		assert.Equal(t, int64(2), count, "explicit repeated logs should create new rows")
	})

	t.Run("TC_PHASE1_02: Ambiguous modify returns options, no silent update", func(t *testing.T) {
		user := setupUserWithGoal("202", 2000)
		now := time.Now()
		database.DB.Create(&models.MealLog{
			UserID: user.ID, Name: "Egg White Omelette", MealType: "Breakfast",
			Ingredients: "6 egg whites", Calories: 100, LoggedAt: now.Add(-15 * time.Minute),
		})
		database.DB.Create(&models.MealLog{
			UserID: user.ID, Name: "Egg White Omelette", MealType: "Breakfast",
			Ingredients: "6 egg whites", Calories: 100, LoggedAt: now.Add(-5 * time.Minute),
		})

		mockClient := &whatsapp.MockClient{}
		sess := whatsapp.NewSession(user, "test-msg-id", slog.Default())
		sess.Client = mockClient

		raw, err := whatsapp.HandleModifyMeal(sess, map[string]interface{}{
			"meal_type":        "Breakfast",
			"action":           "update",
			"target_dish_name": "Egg White Omelette",
			"new_ingredients":  "5 egg whites",
		})
		assert.NoError(t, err)

		var payload map[string]any
		_ = json.Unmarshal([]byte(raw), &payload)
		assert.Equal(t, false, payload["ok"])
		assert.Equal(t, "ambiguous_target", payload["error"])
		optionsAny, hasOptions := payload["options"]
		assert.True(t, hasOptions, "ambiguous response must include selectable options")
		options, _ := optionsAny.([]interface{})
		if len(options) > 0 {
			first, _ := options[0].(map[string]interface{})
			_, hasMealID := first["meal_id"]
			assert.False(t, hasMealID, "ambiguous options must not expose meal_id")
			_, hasIndex := first["index"]
			assert.True(t, hasIndex, "ambiguous options must include index")
		}
	})

	t.Run("TC_PHASE1_02B: Ambiguous modify sends human-readable numbered choices", func(t *testing.T) {
		user := setupUserWithGoal("202B", 2000)
		now := time.Now()
		database.DB.Create(&models.MealLog{
			UserID: user.ID, Name: "Egg White Omelette", MealType: "Breakfast",
			Ingredients: "6 egg whites", Calories: 100, LoggedAt: now.Add(-15 * time.Minute),
		})
		database.DB.Create(&models.MealLog{
			UserID: user.ID, Name: "Egg White Omelette", MealType: "Breakfast",
			Ingredients: "6 egg whites", Calories: 100, LoggedAt: now.Add(-5 * time.Minute),
		})

		mockProvider.ChatWithToolsFunc = func(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error) {
			args := `{"meal_type":"Breakfast","action":"update","target_dish_name":"Egg White Omelette","new_ingredients":"5 egg whites"}`
			return &llm.Message{
				Role:      "assistant",
				ToolCalls: []llm.ToolCall{{Function: llm.ToolCallFunction{Name: "modify_logged_meal", Arguments: args}}},
			}, llm.Usage{}, nil
		}
		mockProvider.ChatFunc = func(messages []llm.Message) (string, llm.Usage, error) {
			return "ok", llm.Usage{}, nil
		}

		resp := runWhatsAppIntent(user, "update my omelette")
		lower := strings.ToLower(resp)
		assert.Contains(t, lower, "reply with the option number")
		assert.Contains(t, lower, "0.")
		assert.NotContains(t, lower, "\"options\"")
		assert.NotContains(t, lower, "meal_id")
	})

	t.Run("TC_PHASE1_03: Past-day summary uses user timezone midnight window", func(t *testing.T) {
		user := setupUserWithGoal("203", 2000)
		database.DB.Create(&models.UserPreferences{UserID: user.ID, Timezone: "Asia/Kolkata"})

		loc, _ := time.LoadLocation("Asia/Kolkata")
		// Keep fixture timestamps in user's local timezone so date-window filtering is evaluated in the same location.
		inDay := time.Date(2026, 5, 6, 0, 10, 0, 0, loc)
		outDay := time.Date(2026, 5, 5, 23, 50, 0, 0, loc)

		database.DB.Create(&models.MealLog{
			UserID: user.ID, Name: "In Day Meal", MealType: "Breakfast", Ingredients: "x", Calories: 100, LoggedAt: inDay,
		})
		database.DB.Create(&models.MealLog{
			UserID: user.ID, Name: "Out Day Meal", MealType: "Dinner", Ingredients: "x", Calories: 200, LoggedAt: outDay,
		})

		mockClient := &whatsapp.MockClient{}
		sess := whatsapp.NewSession(user, "test-msg-id", slog.Default())
		sess.Client = mockClient

		raw, err := whatsapp.HandleGetPastDaySummary(sess, map[string]interface{}{"date": "2026-05-06"})
		assert.NoError(t, err)

		var payload map[string]any
		_ = json.Unmarshal([]byte(raw), &payload)
		assert.Equal(t, float64(1), payload["count"])
		linesAny, _ := payload["lines"].([]interface{})
		linesJoined := ""
		for _, ln := range linesAny {
			if s, ok := ln.(string); ok {
				linesJoined += s
			}
		}
		assert.Contains(t, linesJoined, "In Day Meal")
		assert.NotContains(t, linesJoined, "Out Day Meal")
	})

	t.Run("TC_PHASE1_04: Single turn with multiple log tool calls writes only once", func(t *testing.T) {
		user := setupUserWithGoal("205", 2000)

		mockProvider.ChatWithToolsFunc = func(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error) {
			args1 := `{"meals":[{"dish_name":"Papaya","ingredients":"100g papaya","meal_type":"Snack"}]}`
			args2 := `{"meals":[{"dish_name":"Papaya","ingredients":"100 gm papaya","meal_type":"Snack"}]}`
			return &llm.Message{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{Function: llm.ToolCallFunction{Name: "log_meals", Arguments: args1}},
					{Function: llm.ToolCallFunction{Name: "log_meals", Arguments: args2}},
				},
			}, llm.Usage{}, nil
		}
		mockProvider.ChatFunc = func(messages []llm.Message) (string, llm.Usage, error) {
			last := messages[len(messages)-1]
			if s, ok := last.Content.(string); ok && strings.Contains(s, "TOOL RESULTS JSON:") {
				return "Logged.", llm.Usage{}, nil
			}
			return `{"calories": 43, "protein": 0.5, "carbs": 11, "fat": 0.1, "fiber": 1.7}`, llm.Usage{}, nil
		}

		_ = runWhatsAppIntent(user, "i had papaya")

		var count int64
		database.DB.Model(&models.MealLog{}).
			Where("user_id = ? AND LOWER(name) = ?", user.ID, "papaya").
			Count(&count)
		assert.Equal(t, int64(1), count, "only one create write should occur per message")
	})

	t.Run("TC_SUM_06: Structured daily summary includes all meal sections and constructive feedback", func(t *testing.T) {
		user := setupUserWithGoal("204", 2000)
		now := time.Now().In(time.Local)
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		database.DB.Create(&models.MealLog{
			UserID: user.ID, Name: "Egg Whites", MealType: "Breakfast", Ingredients: "6 egg whites",
			Calories: 180, Protein: 24, Carbs: 2, Fat: 5, Fiber: 0, LoggedAt: todayStart.Add(8 * time.Hour),
		})
		database.DB.Create(&models.MealLog{
			UserID: user.ID, Name: "Rice Bowl", MealType: "Lunch", Ingredients: "rice and dal",
			Calories: 920, Protein: 22, Carbs: 130, Fat: 28, Fiber: 9, LoggedAt: todayStart.Add(13 * time.Hour),
		})

		resp := runWhatsAppIntent(user, "what all did i eat today")
		lower := strings.ToLower(resp)

		assert.Contains(t, lower, "breakfast")
		assert.Contains(t, lower, "lunch")
		assert.Contains(t, lower, "snack")
		assert.Contains(t, lower, "dinner")
		assert.Contains(t, lower, "final total")
		assert.Contains(t, lower, "feedback:")
		assert.Contains(t, lower, "egg whites")
		assert.Contains(t, lower, "rice bowl")
		assert.NotContains(t, lower, "action:")
	})
}
