package whatsapp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/config"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/llm"
	"github.com/pmitra96/pateproject/models"
	"github.com/pmitra96/pateproject/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Orchestrator struct {
	Registry *ToolRegistry
}

func NewOrchestrator() *Orchestrator {
	r := NewToolRegistry()
	r.Register("log_meals", HandleLogMeals)
	r.Register("set_daily_goal", HandleSetGoal)
	r.Register("ask_advice", HandleAskAdvice)
	r.Register("get_leftover_budget", HandleGetBudget)
	r.Register("update_user_profile", HandleUpdateProfile)
	r.Register("update_pantry", HandleUpdatePantry)
	r.Register("get_daily_summary", HandleGetDailySummary)
	r.Register("modify_logged_meal", HandleModifyMeal)
	r.Register("get_past_day_summary", HandleGetPastDaySummary)
	r.Register("clear_all_meals_today", HandleClearAllMealsToday)
	r.Register("create_recipe", HandleCreateRecipe)
	r.Register("get_pantry", HandleGetPantry)
	r.Register("get_recipes", HandleGetRecipes)
	r.Register("delete_recipe", HandleDeleteRecipe)
	
	return &Orchestrator{Registry: r}
}

func (o *Orchestrator) ProcessMessage(s *Session, text string, imageBase64 string) {
	// 1. Daily Limit Check
	var count int64
	todayStart := time.Now().Truncate(24 * time.Hour)
	database.DB.Model(&models.LLMUsageLog{}).Where("user_id = ? AND created_at >= ?", s.User.ID, todayStart).Count(&count)
	if count >= int64(config.GetWhatsAppDailyLimit()) {
		s.Logger.Warn("User reached daily limit")
		s.Reply(MsgErrorLimit)
		return
	}

	// 2. Initial Acknowledgement
	if len(text) > 10 || imageBase64 != "" {
		s.Reply(MsgThinking)
	}

	// 3. Build AI Context
	userContext := o.buildUserContext(s.User)

	// 4. Fetch History
	var conv models.Conversation
	database.DB.Where("user_id = ?", s.User.ID).First(&conv)
	var history []llm.Message
	if conv.Messages != "" {
		json.Unmarshal([]byte(conv.Messages), &history)
	}

	// 5. Call LLM
	llmClient := llm.NewClient()
	startTime := time.Now()
	assistantMsg, usage, err := llmClient.ProcessWhatsAppConversation(text, imageBase64, history, userContext)
	duration := time.Since(startTime)

	if err != nil {
		s.Logger.Error("LLM Processing Error", "error", err, "duration_ms", duration.Milliseconds())
		s.Reply(MsgErrorBrain)
		return
	}
	s.Logger.Info("LLM Processing Complete", "duration_ms", duration.Milliseconds(), "total_tokens", usage.TotalTokens)

	// 6. Log Usage
	database.DB.Create(&models.LLMUsageLog{
		UserID:           s.User.ID,
		Model:            config.GetPreferredLLMModel(),
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		Feature:          "whatsapp",
		CreatedAt:        time.Now(),
	})

	// 7. Update History
	o.updateHistory(s.User.ID, text, assistantMsg)

	// 8. Handle Conversational vs Tool Replies
	if len(assistantMsg.ToolCalls) == 0 {
		content := o.extractText(assistantMsg)
		if content == "" {
			s.Reply(MsgErrorEmpty)
		} else {
			s.Reply(content)
		}
		return
	}

	// 9. Handle Tool Calls
	var toolResponses []string
	for _, tc := range assistantMsg.ToolCalls {
		resp, err := o.Registry.Execute(s, tc)
		if err != nil {
			s.Logger.Warn("Tool Execution Error", "tool", tc.Function.Name, "error", err)
			toolResponses = append(toolResponses, fmt.Sprintf("I tried to use '%s' but it's not available right now.", tc.Function.Name))
			continue
		}
		if resp != "" {
			toolResponses = append(toolResponses, resp)
		}
	}

	if len(toolResponses) > 0 {
		s.Reply(strings.Join(toolResponses, "\n\n"))
	} else {
		s.Reply(MsgErrorEmpty)
	}
}

func (o *Orchestrator) buildUserContext(user *models.User) string {
	state, _ := services.ComputeRemainingDayState(user.ID, time.Now())
	context := "No active goal set."
	if state != nil {
		context = fmt.Sprintf("TODAY: Goal %.0f kcal. Consumed %.0f/%.0f kcal. Remaining: %.0f kcal, %.1fg protein. Mode: %s.",
			state.TargetCalories, (state.TargetCalories - state.RemainingCalories), state.TargetCalories, state.RemainingCalories, state.RemainingProtein, state.ControlMode)
	}

	var prefs models.UserPreferences
	database.DB.Where("user_id = ?", user.ID).First(&prefs)
	profile := "\nUSER PROFILE: "
	if prefs.Height > 0 { profile += fmt.Sprintf("Height: %.1fcm. ", prefs.Height) }
	if prefs.Weight > 0 { profile += fmt.Sprintf("Weight: %.1fkg. ", prefs.Weight) }
	if prefs.Age > 0 { profile += fmt.Sprintf("Age: %d. ", prefs.Age) }
	if prefs.Gender != "" { profile += fmt.Sprintf("Gender: %s. ", prefs.Gender) }
	
	return context + profile
}

func (o *Orchestrator) updateHistory(userID uint, userText string, assistantMsg *llm.Message) {
	database.DB.Transaction(func(tx *gorm.DB) error {
		var latestConv models.Conversation
		tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&latestConv)

		var latestHistory []llm.Message
		if latestConv.Messages != "" {
			json.Unmarshal([]byte(latestConv.Messages), &latestHistory)
		}

		updatedHistory := append(latestHistory, llm.Message{Role: "user", Content: userText})
		if len(assistantMsg.ToolCalls) == 0 && assistantMsg.Content != "" {
			updatedHistory = append(updatedHistory, llm.Message{Role: "assistant", Content: assistantMsg.Content})
		} else if len(assistantMsg.ToolCalls) > 0 {
			updatedHistory = append(updatedHistory, llm.Message{Role: "assistant", Content: fmt.Sprintf("[Action: %s]", assistantMsg.ToolCalls[0].Function.Name)})
		}

		if len(updatedHistory) > config.GetWhatsAppHistoryWindow() {
			updatedHistory = updatedHistory[len(updatedHistory)-config.GetWhatsAppHistoryWindow():]
		}
		historyJSON, _ := json.Marshal(updatedHistory)
		
		if latestConv.ID == 0 {
			return tx.Create(&models.Conversation{UserID: userID, Messages: string(historyJSON)}).Error
		}
		return tx.Model(&latestConv).Update("messages", string(historyJSON)).Error
	})
}

func (o *Orchestrator) extractText(msg *llm.Message) string {
	switch v := msg.Content.(type) {
	case string:
		return v
	case []llm.ContentPart:
		var res string
		for _, p := range v {
			if p.Type == "text" { res += p.Text }
		}
		return res
	}
	return ""
}
