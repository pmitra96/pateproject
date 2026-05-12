package tests

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/pmitra96/pateproject/controllers"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/llm"
	"github.com/pmitra96/pateproject/models"
	"github.com/pmitra96/pateproject/services/whatsapp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoldenFSMConversations(t *testing.T) {
	setupTestDB()
	t.Setenv("TURN_PIPELINE_V2_MODE", "launch")
	t.Setenv("INTENT_CLASSIFIER_MODE", "launch")

	mockProvider := &MockProvider{}
	llm.SetMockProvider(mockProvider)

	parserCalls := 0
	mockProvider.ChatFunc = func(messages []llm.Message) (string, llm.Usage, error) {
		if len(messages) > 0 {
			if sys, ok := messages[0].Content.(string); ok && strings.Contains(strings.ToLower(sys), "strict json classifier") {
				userBlob, _ := messages[len(messages)-1].Content.(string)
				lower := strings.ToLower(userBlob)
				switch {
				case strings.Contains(lower, "what all did i eat today"):
					return `{"intent":"get_summary","confidence":0.98,"entities":{"date":"today","metric":"all"},"missing_slots":[],"reason":"User asked for today's eaten items summary."}`, llm.Usage{}, nil
				case strings.Contains(lower, "i had salad"):
					return `{"intent":"log_meal","confidence":0.95,"entities":{"meal_type":"unknown","date":"today","items":[{"name":"salad","quantity":null}]},"missing_slots":["quantity"],"reason":"User is logging consumed food."}`, llm.Usage{}, nil
				case strings.Contains(lower, "3 eggs with 1 small onion") && strings.Contains(lower, "150 ml coffee"):
					return `{"intent":"log_meal","confidence":0.99,"entities":{"meal_type":"breakfast","date":"today","items":[{"name":"eggs scramble","quantity":"3 eggs"},{"name":"coffee","quantity":"150 ml"}]},"missing_slots":[],"reason":"User is logging multiple consumed items."}`, llm.Usage{}, nil
				default:
					return `{"intent":"fallback","confidence":0.4,"entities":{},"missing_slots":[],"reason":"Could not classify confidently."}`, llm.Usage{}, nil
				}
			}
			if sys, ok := messages[0].Content.(string); ok && strings.Contains(strings.ToLower(sys), "strict json meal parser") {
				parserCalls++
				userBlob, _ := messages[len(messages)-1].Content.(string)
				lower := strings.ToLower(userBlob)
				switch {
				case strings.Contains(lower, "i had salad"):
					return `{
  "meal_type":"unknown",
  "parsed_items":[],
  "meal_total":{"calories":0,"protein_g":0,"carbs_g":0,"fat_g":0},
  "flags":[],
  "clarification_needed":true,
  "clarification_question":"What was the quantity in grams/ml or serving size?"
}`, llm.Usage{}, nil
				default:
					return `{
  "meal_type":"breakfast",
  "parsed_items":[
    {"raw_text":"3 eggs with 1 small onion, 1 small tomato and few sprays of oil","food_name":"Scrambled Eggs With Onion And Tomato","brand":null,"quantity":1,"unit":"serving","quantity_in_grams_estimated":null,"ingredients":[{"name":"whole egg","quantity":3,"unit":"piece","brand":null,"calories":198,"protein_g":20.1,"carbs_g":1.5,"fat_g":12,"fiber_g":0},{"name":"onion","quantity":1,"unit":"piece","brand":null,"calories":16,"protein_g":0.4,"carbs_g":3.7,"fat_g":0.0,"fiber_g":0.7},{"name":"tomato","quantity":1,"unit":"piece","brand":null,"calories":18,"protein_g":0.9,"carbs_g":3.9,"fat_g":0.2,"fiber_g":1.2},{"name":"oil","quantity":1,"unit":"tsp","brand":null,"calories":45,"protein_g":0,"carbs_g":0,"fat_g":5,"fiber_g":0}],"cooking_method":"panfried","modifiers":[],"assumptions":[],"calories":277,"protein_g":21.4,"carbs_g":9.1,"fat_g":17.2,"confidence":"medium"},
    {"raw_text":"150 ml coffee with akshayakalpa a2 buffalo milk","food_name":"Coffee With A2 Buffalo Milk","brand":"Akshayakalpa","quantity":150,"unit":"ml","quantity_in_grams_estimated":150,"ingredients":[{"name":"coffee with milk","quantity":150,"unit":"ml","brand":"Akshayakalpa","calories":90,"protein_g":5.5,"carbs_g":6,"fat_g":5.5,"fiber_g":0}],"cooking_method":"unknown","modifiers":[],"assumptions":[],"calories":90,"protein_g":5.5,"carbs_g":6,"fat_g":5.5,"confidence":"medium"}
  ],
  "meal_total":{"calories":367,"protein_g":26.9,"carbs_g":15.1,"fat_g":22.7},
  "flags":[],
  "clarification_needed":false,
  "clarification_question":null
}`, llm.Usage{}, nil
				}
			}
			if userContent, ok := messages[len(messages)-1].Content.(string); ok && strings.Contains(userContent, "TOOL RESULTS JSON:") {
				return "Logged.", llm.Usage{}, nil
			}
		}
		return `{"calories":120,"protein":4,"carbs":22,"fat":2,"fiber":3,"serving_size":"1 serving"}`, llm.Usage{}, nil
	}
	mockProvider.ChatWithToolsFunc = func(messages []llm.Message, tools []llm.Tool) (*llm.Message, llm.Usage, error) {
		return &llm.Message{Role: "assistant", Content: "Hello! How can I help with your nutrition today?"}, llm.Usage{}, nil
	}

	runTurn := func(t *testing.T, user *models.User, msgID string, text string) string {
		t.Helper()
		mockClient := &whatsapp.MockClient{}
		sess := whatsapp.NewSession(user, msgID, slog.Default())
		sess.Client = mockClient
		whatsapp.NewOrchestrator().ProcessMessage(sess, text, "")
		return mockClient.LastMessage
	}

	t.Run("golden_multi_item_then_summary", func(t *testing.T) {
		user, err := controllers.GetOrCreateWhatsAppUser("919001001001")
		require.NoError(t, err)

		resp := runTurn(t, user, "golden-msg-1", "i had 3 eggs with 1 small onion, 1 small tomato and few sprays of oil and also 150 ml coffee with akshayakalpa a2 buffalo milk")
		assert.Contains(t, strings.ToLower(resp), "logged meals")

		var mealCount int64
		database.DB.Model(&models.MealLog{}).Where("user_id = ?", user.ID).Count(&mealCount)
		assert.Equal(t, int64(2), mealCount)

		var st models.ConversationState
		require.NoError(t, database.DB.Where("user_id = ?", user.ID).First(&st).Error)
		assert.Equal(t, "replied", st.FSMState)
		assert.Equal(t, "log_meals", st.LastTool)

		parserCallsAfterLog := parserCalls
		summaryResp := runTurn(t, user, "golden-msg-2", "what all did i eat today")
		assert.Contains(t, strings.ToLower(summaryResp), "today's summary")
		assert.Equal(t, parserCallsAfterLog, parserCalls, "parser must not run for read/summary turn")

		require.NoError(t, database.DB.Where("user_id = ?", user.ID).First(&st).Error)
		assert.Equal(t, "replied", st.FSMState)
		assert.Equal(t, "get_daily_summary", st.LastTool)
	})

	t.Run("golden_clarification_preserves_pending_state", func(t *testing.T) {
		user, err := controllers.GetOrCreateWhatsAppUser("919001001002")
		require.NoError(t, err)

		resp := runTurn(t, user, "golden-msg-3", "i had salad")
		assert.Contains(t, strings.ToLower(resp), "quantity")

		var st models.ConversationState
		require.NoError(t, database.DB.Where("user_id = ?", user.ID).First(&st).Error)
		assert.Equal(t, "replied", st.FSMState)
		assert.Equal(t, "awaiting_clarification", st.FSMPendingState)

		var mealCount int64
		database.DB.Model(&models.MealLog{}).Where("user_id = ?", user.ID).Count(&mealCount)
		assert.Equal(t, int64(0), mealCount)
	})

	t.Run("golden_repeat_message_id_no_fsm_regression", func(t *testing.T) {
		user, err := controllers.GetOrCreateWhatsAppUser("919001001003")
		require.NoError(t, err)
		_ = runTurn(t, user, "golden-msg-4", "i had salad")
		first := models.ConversationState{}
		require.NoError(t, database.DB.Where("user_id = ?", user.ID).First(&first).Error)

		_ = runTurn(t, user, "golden-msg-4", "i had salad")
		second := models.ConversationState{}
		require.NoError(t, database.DB.Where("user_id = ?", user.ID).First(&second).Error)
		assert.Equal(t, "replied", second.FSMState)
		assert.True(t, second.FSMStateVersion >= first.FSMStateVersion, fmt.Sprintf("fsm version regressed: %d -> %d", first.FSMStateVersion, second.FSMStateVersion))
	})
}
