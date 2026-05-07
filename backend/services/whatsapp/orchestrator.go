package whatsapp

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
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

type toolExecutionResult struct {
	ToolName string `json:"tool_name"`
	Response any    `json:"response"`
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
	r.Register("get_meal_log_time", HandleGetMealLogTime)
	r.Register("get_recent_meals", HandleGetRecentMeals)
	r.Register("get_active_goal", HandleGetActiveGoal)
	r.Register("get_user_profile", HandleGetUserProfile)
	r.Register("get_recent_orders", HandleGetRecentOrders)

	return &Orchestrator{Registry: r}
}

func (o *Orchestrator) ProcessMessage(s *Session, text string, imageBase64 string) {
	// 1. Daily Limit Check
	var count int64
	nowLocal := time.Now().In(userLocationForDisplay(s.User.ID))
	todayStart, todayEnd := dayWindow(nowLocal)
	database.DB.Model(&models.LLMUsageLog{}).
		Where("user_id = ? AND created_at >= ? AND created_at < ?", s.User.ID, todayStart, todayEnd).
		Count(&count)
	if count >= int64(config.GetWhatsAppDailyLimit()) {
		s.Logger.Warn("User reached daily limit")
		o.replySafe(s, MsgErrorLimit)
		return
	}

	// 2. Initial Acknowledgement
	if len(text) > 10 || imageBase64 != "" {
		o.replySafe(s, MsgThinking)
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

	// 4.5 V2 deterministic router for core intents.
	state := getConversationState(s.User.ID)
	if handled := o.tryResolvePendingMealSelection(s, state, text); handled {
		return
	}
	decision := routeWhatsAppMessage(text, state)
	s.Logger.Info("V2 route decision", "intent", decision.Intent, "tool", decision.ToolName, "needs_llm", decision.NeedsLLM)
	if !decision.NeedsLLM {
		if decision.DirectReply != "" {
			o.replySafe(s, decision.DirectReply)
			updateConversationStateAfterReply(state, decision.Intent, decision.DirectReply)
			o.appendConversationTurn(s.User.ID, text, decision.DirectReply)
			return
		}
		if decision.ToolName != "" {
			toolCall := buildToolCall(decision.ToolName, decision.Args)
			resp, err := o.Registry.Execute(s, toolCall)
			if err == nil {
				result := []toolExecutionResult{{ToolName: decision.ToolName, Response: parseToolResponse(resp)}}
				reply := deterministicToolReply(result)
				if reply == "" || reply == "Done." {
					reply = resp
				}
				o.replySafe(s, reply)
				updateConversationStateAfterTool(state, decision.Intent, decision.ToolName, resp)
				o.appendConversationTurn(s.User.ID, text, reply)
				return
			}
			s.Logger.Warn("Deterministic route tool execution failed", "tool", decision.ToolName, "error", err)
		}
	}

	// 5. Call LLM
	llmClient := llm.NewClient()
	startTime := time.Now()
	history = compactHistoryForLLM(history, 12)
	assistantMsg, usage, err := llmClient.ProcessWhatsAppConversation(text, imageBase64, history, userContext)
	duration := time.Since(startTime)

	if err != nil {
		s.Logger.Error("LLM Processing Error", "error", err, "duration_ms", duration.Milliseconds())
		if heuristic := o.tryHeuristicFallback(s, text); heuristic != "" {
			o.replySafe(s, heuristic)
			return
		}
		o.replySafe(s, MsgErrorBrain)
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
	s.Logger.Info("LLM usage", "prompt_tokens", usage.PromptTokens, "completion_tokens", usage.CompletionTokens, "total_tokens", usage.TotalTokens, "history_len", len(history))

	// 7. Update History
	o.updateHistory(s.User.ID, text, assistantMsg)

	// 8. Handle Conversational vs Tool Replies
	if len(assistantMsg.ToolCalls) == 0 {
		content := o.extractText(assistantMsg)
		if synthetic, ok := synthesizeToolCallFromActionText(content); ok {
			resp, err := o.Registry.Execute(s, synthetic)
			if err == nil {
				result := []toolExecutionResult{{
					ToolName: synthetic.Function.Name,
					Response: parseToolResponse(resp),
				}}
				if synthetic.Function.Name == "get_daily_summary" {
					if deterministic := deterministicToolReply(result); deterministic != "" && deterministic != "Done." {
						o.replySafe(s, deterministic)
						updateConversationStateAfterTool(state, "llm", synthetic.Function.Name, resp)
						o.appendAssistantMessage(s.User.ID, deterministic)
						return
					}
				}
				if finalReply := o.composeFinalReplyFromToolResults(llmClient, text, result); finalReply != "" {
					o.replySafe(s, finalReply)
					updateConversationStateAfterTool(state, "llm", synthetic.Function.Name, resp)
					o.appendAssistantMessage(s.User.ID, finalReply)
					return
				}
				if resp != "" {
					o.replySafe(s, resp)
					updateConversationStateAfterTool(state, "llm", synthetic.Function.Name, resp)
					o.appendAssistantMessage(s.User.ID, resp)
					return
				}
			}
		}
		if inferred, ok := inferDeterministicToolCall(text); ok {
			resp, err := o.Registry.Execute(s, inferred)
			if err == nil && resp != "" {
				result := []toolExecutionResult{{ToolName: inferred.Function.Name, Response: parseToolResponse(resp)}}
				if inferred.Function.Name == "get_daily_summary" {
					if deterministic := deterministicToolReply(result); deterministic != "" && deterministic != "Done." {
						o.replySafe(s, deterministic)
						updateConversationStateAfterTool(state, "llm", inferred.Function.Name, resp)
						o.appendAssistantMessage(s.User.ID, deterministic)
						return
					}
				}
				if finalReply := o.composeFinalReplyFromToolResults(llmClient, text, result); finalReply != "" {
					o.replySafe(s, finalReply)
					updateConversationStateAfterTool(state, "llm", inferred.Function.Name, resp)
					o.appendAssistantMessage(s.User.ID, finalReply)
					return
				}
				o.replySafe(s, resp)
				updateConversationStateAfterTool(state, "llm", inferred.Function.Name, resp)
				o.appendAssistantMessage(s.User.ID, resp)
				return
			}
		}
		if content == "" {
			if heuristic := o.tryHeuristicFallback(s, text); heuristic != "" {
				o.replySafe(s, heuristic)
				return
			}
			o.replySafe(s, MsgErrorEmpty)
		} else {
			s.Logger.Info("Reply mode", "type", "direct_llm")
			o.replySafe(s, content)
			updateConversationStateAfterReply(state, "llm", content)
		}
		return
	}

	// 9. Handle Tool Calls
	uniqueToolCalls, skippedDuplicates := dedupeToolCalls(assistantMsg.ToolCalls)
	if skippedDuplicates > 0 {
		s.Logger.Info("Skipped duplicate tool calls", "duplicates", skippedDuplicates, "requested", len(assistantMsg.ToolCalls), "executing", len(uniqueToolCalls))
	}

	var toolResponses []string
	var toolResults []toolExecutionResult
	for _, tc := range uniqueToolCalls {
		resp, err := o.Registry.Execute(s, tc)
		if err != nil {
			s.Logger.Warn("Tool Execution Error", "tool", tc.Function.Name, "error", err)
			toolResponses = append(toolResponses, fmt.Sprintf("I tried to use '%s' but it's not available right now.", tc.Function.Name))
			continue
		}

		toolResults = append(toolResults, toolExecutionResult{
			ToolName: tc.Function.Name,
			Response: parseToolResponse(resp),
		})

		if resp != "" {
			toolResponses = append(toolResponses, resp)
		}
	}

	// If user asks to set a goal but model missed set_daily_goal, enforce it via deterministic fallback.
	if isGoalSetRequest(text) && !toolExecuted(toolResults, "set_daily_goal") {
		if goalResp, ok := o.ensureGoalSetFromProfile(s); ok {
			toolResults = append(toolResults, toolExecutionResult{
				ToolName: "set_daily_goal",
				Response: parseToolResponse(goalResp),
			})
			toolResponses = append(toolResponses, goalResp)
		}
	}

	// Prefer deterministic human-readable rendering for known structured tool outputs.
	if len(toolResults) > 0 {
		if deterministic := deterministicToolReply(toolResults); deterministic != "" && deterministic != "Done." {
			o.replySafe(s, deterministic)
			updateConversationStateAfterTool(state, "llm", firstToolName(toolResults), marshalToolResults(toolResults))
			o.appendAssistantMessage(s.User.ID, deterministic)
			return
		}
	}

	if len(toolResults) > 0 && shouldUseSecondPass(toolResults) {
		if toolExecuted(toolResults, "get_daily_summary") {
			if deterministic := deterministicToolReply(toolResults); deterministic != "" && deterministic != "Done." {
				o.replySafe(s, deterministic)
				o.appendAssistantMessage(s.User.ID, deterministic)
				return
			}
		}
		finalReply := o.composeFinalReplyFromToolResults(llmClient, text, toolResults)
		if finalReply != "" {
			s.Logger.Info("Reply mode", "type", "tool_second_pass", "tool_count", len(toolResults))
			o.replySafe(s, finalReply)
			updateConversationStateAfterTool(state, "llm", firstToolName(toolResults), marshalToolResults(toolResults))
			o.appendAssistantMessage(s.User.ID, finalReply)
			return
		}
	}

	if len(toolResponses) == 0 {
		o.replySafe(s, MsgErrorEmpty)
		return
	}
	s.Logger.Info("Reply mode", "type", "tool_raw", "tool_count", len(toolResponses))
	finalRaw := strings.Join(toolResponses, "\n\n")
	o.replySafe(s, finalRaw)
	updateConversationStateAfterTool(state, "llm", firstToolName(toolResults), marshalToolResults(toolResults))
	o.appendAssistantMessage(s.User.ID, finalRaw)
}

func (o *Orchestrator) tryHeuristicFallback(s *Session, text string) string {
	lower := strings.ToLower(text)

	// Heuristic 1: profile capture from free text.
	heightRe := regexp.MustCompile(`(\d{2,3})\s*cm`)
	weightRe := regexp.MustCompile(`(?:weigh|weight)\s*(\d{2,3}(?:\.\d+)?)\s*kg`)
	ageRe := regexp.MustCompile(`(\d{1,3})\s*(?:years?\s*old|yo|yrs?)`)
	gender := ""
	if strings.Contains(lower, "male") {
		gender = "male"
	} else if strings.Contains(lower, "female") {
		gender = "female"
	}

	updates := map[string]interface{}{}
	if m := heightRe.FindStringSubmatch(lower); len(m) > 1 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			updates["height"] = v
		}
	}
	if m := weightRe.FindStringSubmatch(lower); len(m) > 1 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			updates["weight"] = v
		}
	}
	if m := ageRe.FindStringSubmatch(lower); len(m) > 1 {
		if v, err := strconv.Atoi(m[1]); err == nil {
			updates["age"] = v
		}
	}
	if gender != "" {
		updates["gender"] = gender
	}
	if len(updates) > 0 {
		var prefs models.UserPreferences
		if err := database.DB.Where("user_id = ?", s.User.ID).First(&prefs).Error; err != nil {
			prefs = models.UserPreferences{UserID: s.User.ID}
			database.DB.Create(&prefs)
		}
		database.DB.Model(&models.UserPreferences{}).Where("user_id = ?", s.User.ID).Updates(updates)
		return "Profile Updated."
	}

	// Heuristic 2: explicit goal set request when profile exists.
	if goal := parseWeightLossGoal(lower); goal != nil {
		var prefs models.UserPreferences
		if err := database.DB.Where("user_id = ?", s.User.ID).First(&prefs).Error; err == nil {
			target, reason, ok := calculateGoalFromProfile(prefs, *goal)
			if ok {
				_, _ = HandleSetGoal(s, map[string]interface{}{"calories": float64(target)})
				return fmt.Sprintf("Goal target set to %d calories.\n%s", target, reason)
			}
			return reason
		}
		return "Please share your height, weight, age, gender, and activity level to calculate this goal."
	}

	// Heuristic 3: explicit goal set request when profile exists.
	if strings.Contains(lower, "set my goal") || strings.Contains(lower, "set goal") {
		var prefs models.UserPreferences
		if err := database.DB.Where("user_id = ?", s.User.ID).First(&prefs).Error; err == nil {
			target := 1800
			if prefs.Weight > 95 {
				target = 2000
			} else if prefs.Weight > 75 {
				target = 1900
			}
			_, _ = HandleSetGoal(s, map[string]interface{}{"calories": float64(target)})
			return fmt.Sprintf("Goal target set to %d calories.", target)
		}
	}

	return ""
}

func (o *Orchestrator) buildUserContext(user *models.User) string {
	state, _ := services.ComputeRemainingDayState(user.ID, time.Now())
	context := "No active goal set."
	if state != nil {
		context = fmt.Sprintf("TODAY: Goal %.0f kcal. Consumed %.0f/%.0f kcal. Remaining: %.0f kcal, %.1fg protein, %.1fg carbs, %.1fg fat, %.1fg fiber. Mode: %s.",
			state.TargetCalories, (state.TargetCalories - state.RemainingCalories), state.TargetCalories, state.RemainingCalories, state.RemainingProtein, state.RemainingCarbs, state.RemainingFat, state.RemainingFiber, state.ControlMode)
	}

	var prefs models.UserPreferences
	database.DB.Where("user_id = ?", user.ID).First(&prefs)
	profile := "\nUSER PROFILE: "
	if prefs.Height > 0 {
		profile += fmt.Sprintf("Height: %.1fcm. ", prefs.Height)
	}
	if prefs.Weight > 0 {
		profile += fmt.Sprintf("Weight: %.1fkg. ", prefs.Weight)
	}
	if prefs.Age > 0 {
		profile += fmt.Sprintf("Age: %d. ", prefs.Age)
	}
	if prefs.Gender != "" {
		profile += fmt.Sprintf("Gender: %s. ", prefs.Gender)
	}
	if prefs.ActivityLevel != "" {
		profile += fmt.Sprintf("Activity: %s. ", prefs.ActivityLevel)
	}
	if prefs.Timezone != "" {
		profile += fmt.Sprintf("Timezone: %s. ", prefs.Timezone)
	}

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

func (o *Orchestrator) replySafe(s *Session, text string) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if strings.HasPrefix(lower, "action:") || strings.Contains(lower, "[action:") {
		s.Reply("I fetched your data, but had a formatting issue. Please retry once.")
		return
	}
	s.Reply(text)
}

func buildToolCall(name string, args map[string]interface{}) llm.ToolCall {
	payload, _ := json.Marshal(args)
	return llm.ToolCall{
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      name,
			Arguments: string(payload),
		},
	}
}

func (o *Orchestrator) extractText(msg *llm.Message) string {
	switch v := msg.Content.(type) {
	case string:
		return v
	case []llm.ContentPart:
		var res string
		for _, p := range v {
			if p.Type == "text" {
				res += p.Text
			}
		}
		return res
	}
	return ""
}

func parseToolResponse(resp string) any {
	var parsed any
	if err := json.Unmarshal([]byte(resp), &parsed); err == nil {
		return parsed
	}
	return map[string]any{"message": resp}
}

func shouldUseSecondPass(results []toolExecutionResult) bool {
	for _, r := range results {
		if m, ok := r.Response.(map[string]any); ok {
			if _, hasMessage := m["message"]; !hasMessage {
				return true
			}
		}
	}
	return false
}

func (o *Orchestrator) composeFinalReplyFromToolResults(
	llmClient *llm.Client,
	userText string,
	toolResults []toolExecutionResult,
) string {
	payload, err := json.Marshal(toolResults)
	if err != nil {
		return ""
	}

	prompt := fmt.Sprintf(
		"USER MESSAGE:\n%s\n\nTOOL RESULTS JSON:\n%s\n\nUsing only this JSON, answer in 1-3 short lines. If not found, say that and ask one clarifying question.",
		userText, string(payload),
	)
	reply, _, err := llmClient.Chat([]llm.Message{
		{
			Role:    "system",
			Content: "You convert structured tool outputs into a direct user-facing answer. Keep it short and factual.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	})
	if err != nil {
		return ""
	}
	reply = strings.TrimSpace(reply)
	if strings.Contains(reply, "[Action:") || strings.HasPrefix(strings.ToLower(reply), "action:") {
		return deterministicToolReply(toolResults)
	}
	return reply
}

func deterministicToolReply(results []toolExecutionResult) string {
	if len(results) == 0 {
		return ""
	}
	r := results[0]
	switch r.ToolName {
	case "get_daily_summary":
		if m, ok := r.Response.(map[string]any); ok {
			if getCount(m["count"]) == 0 {
				return "No meals logged yet today."
			}
			out := "Today's summary:\n"
			if sections, ok := m["sections"].(map[string]any); ok {
				order := []string{"breakfast", "lunch", "snack", "dinner"}
				for _, key := range order {
					sectionAny, exists := sections[key]
					if !exists {
						continue
					}
					sectionMap, _ := sectionAny.(map[string]any)
					mealsAny, _ := sectionMap["meals"].([]any)
					totalsMap, _ := sectionMap["totals"].(map[string]any)
					out += fmt.Sprintf("\n%s:\n", strings.Title(key))
					if len(mealsAny) == 0 {
						out += "- No meals logged.\n"
					} else {
						for _, item := range mealsAny {
							if s, ok := item.(string); ok {
								out += s + "\n"
							}
						}
					}
					cal := getFloat(totalsMap["calories"])
					pro := getFloat(totalsMap["protein"])
					carbs := getFloat(totalsMap["carbs"])
					fat := getFloat(totalsMap["fat"])
					fiber := getFloat(totalsMap["fiber"])
					out += fmt.Sprintf("Total %s: %.0f kcal, P %.1fg, C %.1fg, F %.1fg, Fi %.1fg\n", strings.Title(key), cal, pro, carbs, fat, fiber)
				}
			}
			if totals, ok := m["totals"].(map[string]any); ok {
				cal := getFloat(totals["calories"])
				pro := getFloat(totals["protein"])
				carbs := getFloat(totals["carbs"])
				fat := getFloat(totals["fat"])
				fiber := getFloat(totals["fiber"])
				out += fmt.Sprintf("\nFinal Total: %.0f kcal, P %.1fg, C %.1fg, F %.1fg, Fi %.1fg\n", cal, pro, carbs, fat, fiber)
			}
			if remaining, ok := m["remaining_text"].(string); ok && strings.TrimSpace(remaining) != "" {
				out += remaining + "\n"
			}
			if feedback, ok := m["budget_feedback"].(string); ok && strings.TrimSpace(feedback) != "" {
				out += "Feedback: " + strings.TrimSpace(feedback)
			}
			return strings.TrimSpace(out)
		}
	case "modify_logged_meal":
		if m, ok := r.Response.(map[string]any); ok {
			if errType, _ := m["error"].(string); errType == "ambiguous_target" {
				out := "I found multiple matching meals. Reply with the option number:\n"
				if options, ok := m["options"].([]any); ok {
					for _, opt := range options {
						option, _ := opt.(map[string]any)
						idx := int(getFloat(option["index"]))
						dish, _ := option["dish_name"].(string)
						mealType, _ := option["meal_type"].(string)
						loggedAt, _ := option["logged_at"].(string)
						out += fmt.Sprintf("%d. %s (%s, %s)\n", idx, dish, mealType, loggedAt)
					}
				}
				return strings.TrimSpace(out)
			}
			if msg, _ := m["message"].(string); strings.TrimSpace(msg) != "" {
				return strings.TrimSpace(msg)
			}
		}
	case "get_leftover_budget":
		if m, ok := r.Response.(map[string]any); ok {
			remaining, _ := m["remaining"].(map[string]any)
			cal := getFloat(remaining["calories"])
			pro := getFloat(remaining["protein"])
			carbs := getFloat(remaining["carbs"])
			fat := getFloat(remaining["fat"])
			fiber := getFloat(remaining["fiber"])
			mode, _ := remaining["mode"].(string)
			if strings.TrimSpace(mode) == "" {
				mode = "NORMAL"
			}
			return fmt.Sprintf("Remaining today: %.0f kcal, P %.1fg, C %.1fg, F %.1fg, Fi %.1fg. Mode: %s.", cal, pro, carbs, fat, fiber, mode)
		}
	}
	return "Done."
}

func getCount(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

func getFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return 0
	}
}

func synthesizeToolCallFromActionText(content string) (llm.ToolCall, bool) {
	re := regexp.MustCompile(`(?i)\[?\s*action:\s*([a-z0-9_]+)\s*\]?`)
	m := re.FindStringSubmatch(strings.TrimSpace(content))
	if len(m) < 2 {
		return llm.ToolCall{}, false
	}
	return llm.ToolCall{
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      strings.ToLower(m[1]),
			Arguments: "{}",
		},
	}, true
}

func inferDeterministicToolCall(text string) (llm.ToolCall, bool) {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "what all did i eat today") || strings.Contains(lower, "meal history") || strings.Contains(lower, "today's summary") || strings.Contains(lower, "what did i eat today") {
		return llm.ToolCall{
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "get_daily_summary",
				Arguments: "{}",
			},
		}, true
	}
	if strings.Contains(lower, "what's my budget") || strings.Contains(lower, "how many calories") {
		return llm.ToolCall{
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "get_leftover_budget",
				Arguments: "{}",
			},
		}, true
	}
	return llm.ToolCall{}, false
}

func (o *Orchestrator) appendConversationTurn(userID uint, userText, assistantText string) {
	database.DB.Transaction(func(tx *gorm.DB) error {
		var conv models.Conversation
		tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&conv)

		var history []llm.Message
		if conv.Messages != "" {
			_ = json.Unmarshal([]byte(conv.Messages), &history)
		}
		history = append(history, llm.Message{Role: "user", Content: userText})
		if strings.TrimSpace(assistantText) != "" {
			history = append(history, llm.Message{Role: "assistant", Content: assistantText})
		}
		if len(history) > config.GetWhatsAppHistoryWindow() {
			history = history[len(history)-config.GetWhatsAppHistoryWindow():]
		}
		raw, _ := json.Marshal(history)
		if conv.ID == 0 {
			return tx.Create(&models.Conversation{UserID: userID, Messages: string(raw)}).Error
		}
		return tx.Model(&conv).Update("messages", string(raw)).Error
	})
}

func (o *Orchestrator) appendAssistantMessage(userID uint, assistantText string) {
	database.DB.Transaction(func(tx *gorm.DB) error {
		if strings.TrimSpace(assistantText) == "" {
			return nil
		}
		var conv models.Conversation
		tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&conv)

		var history []llm.Message
		if conv.Messages != "" {
			_ = json.Unmarshal([]byte(conv.Messages), &history)
		}
		history = append(history, llm.Message{Role: "assistant", Content: assistantText})
		if len(history) > config.GetWhatsAppHistoryWindow() {
			history = history[len(history)-config.GetWhatsAppHistoryWindow():]
		}
		raw, _ := json.Marshal(history)
		if conv.ID == 0 {
			return tx.Create(&models.Conversation{UserID: userID, Messages: string(raw)}).Error
		}
		return tx.Model(&conv).Update("messages", string(raw)).Error
	})
}

func compactHistoryForLLM(history []llm.Message, max int) []llm.Message {
	if max <= 0 || len(history) <= max {
		return history
	}
	return history[len(history)-max:]
}

func (o *Orchestrator) tryResolvePendingMealSelection(s *Session, st *models.ConversationState, text string) bool {
	if st == nil || st.PendingMealAction == "" {
		return false
	}
	ids := getPendingMealIDs(st)
	if len(ids) == 0 {
		clearPendingMealSelection(st)
		return false
	}

	selectedID, ok := resolveMealChoiceFromText(text, ids)
	if !ok {
		return false
	}

	var meal models.MealLog
	if err := database.DB.Where("user_id = ? AND id = ?", s.User.ID, selectedID).First(&meal).Error; err != nil {
		clearPendingMealSelection(st)
		return false
	}

	args := map[string]interface{}{
		"action":           st.PendingMealAction,
		"meal_id":          float64(selectedID),
		"meal_type":        meal.MealType,
		"target_dish_name": meal.Name,
	}
	if st.PendingMealAction == "update" {
		args["new_ingredients"] = text
	}
	tc := buildToolCall("modify_logged_meal", args)
	resp, err := o.Registry.Execute(s, tc)
	clearPendingMealSelection(st)
	if err != nil || strings.TrimSpace(resp) == "" {
		return false
	}
	o.replySafe(s, "Updated. "+resp)
	o.appendConversationTurn(s.User.ID, text, "Updated.")
	return true
}

func resolveMealChoiceFromText(text string, ids []uint) (uint, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if n, err := strconv.Atoi(lower); err == nil {
		if n >= 0 && n < len(ids) {
			return ids[n], true
		}
	}
	if lower == "first" || strings.Contains(lower, "first one") || lower == "1" {
		return ids[0], true
	}
	if lower == "second" || strings.Contains(lower, "second one") || lower == "2" {
		if len(ids) >= 2 {
			return ids[1], true
		}
	}
	if strings.Contains(lower, "last") {
		return ids[len(ids)-1], true
	}
	for i := range ids {
		n := i
		if strings.Contains(lower, strconv.Itoa(n)) {
			return ids[i], true
		}
	}
	return 0, false
}

func isGoalSetRequest(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "set my goal") || strings.Contains(lower, "set goal") || strings.Contains(lower, "target calories")
}

func toolExecuted(results []toolExecutionResult, toolName string) bool {
	for _, r := range results {
		if r.ToolName == toolName {
			return true
		}
	}
	return false
}

func dedupeToolCalls(calls []llm.ToolCall) ([]llm.ToolCall, int) {
	if len(calls) <= 1 {
		return calls, 0
	}
	seen := make(map[string]struct{}, len(calls))
	unique := make([]llm.ToolCall, 0, len(calls))
	dups := 0
	for _, tc := range calls {
		sig := toolCallSignature(tc)
		if _, ok := seen[sig]; ok {
			dups++
			continue
		}
		seen[sig] = struct{}{}
		unique = append(unique, tc)
	}
	return unique, dups
}

func toolCallSignature(tc llm.ToolCall) string {
	return strings.ToLower(strings.TrimSpace(tc.Function.Name)) + "|" + canonicalToolArguments(tc.Function.Arguments)
}

func canonicalToolArguments(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "{}"
	}
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return trimmed
	}
	normalized, err := json.Marshal(parsed)
	if err != nil {
		return trimmed
	}
	return string(normalized)
}

func firstToolName(results []toolExecutionResult) string {
	if len(results) == 0 {
		return ""
	}
	return results[0].ToolName
}

func marshalToolResults(results []toolExecutionResult) string {
	if len(results) == 0 {
		return ""
	}
	raw, err := json.Marshal(results)
	if err != nil {
		return ""
	}
	return string(raw)
}

func (o *Orchestrator) ensureGoalSetFromProfile(s *Session) (string, bool) {
	var prefs models.UserPreferences
	if err := database.DB.Where("user_id = ?", s.User.ID).First(&prefs).Error; err != nil {
		return "", false
	}
	if prefs.Weight <= 0 {
		return "", false
	}

	target := 1800
	if prefs.Weight > 95 {
		target = 2000
	} else if prefs.Weight > 75 {
		target = 1900
	}
	resp, _ := HandleSetGoal(s, map[string]interface{}{"calories": float64(target)})
	return resp, true
}
