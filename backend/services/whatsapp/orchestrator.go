package whatsapp

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
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
	r.Register("get_food_nutrition", HandleGetFoodNutrition)
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
	nowLocal := nowForUser(s.User.ID)
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
	traceID := newTraceID()
	if toolName, toolArgs, handled, pendingReply := resolvePendingActionChoice(state, s.MessageID, text); handled {
		if pendingReply != "" {
			o.replySafe(s, pendingReply)
			updateConversationStateAfterReply(state, "pending_action", pendingReply)
			o.appendConversationTurn(s.User.ID, text, pendingReply)
			return
		}
		if o.executeToolAndRespond(s, state, "pending_action_execute", text, toolName, toolArgs) {
			return
		}
	}

	overridden, hasOverride := classifyIntentOverrides(text)
	llmClient := llm.NewClient()
	intentResult := overridden
	runClassifier := hasOverride || looksLikeMealCRUD(strings.ToLower(strings.TrimSpace(text)))
	if runClassifier && !hasOverride {
		classifier := newIntentClassifier(llmClient)
		classified, err := classifier.Classify(text, userContext, traceID)
		if err == nil {
			intentResult = classified
		} else {
			s.Logger.Warn("Intent classifier failed", "error", err, "trace_id", traceID)
		}
	}
	if runClassifier && intentResult.Intent != "" && intentResult.Intent != IntentFallback {
		s.Logger.Info("Intent classification", "intent", intentResult.Intent, "confidence", intentResult.Confidence, "trace_id", traceID)
		policy := decideIntentPolicy(intentResult)
		switch policy.Decision {
		case DecisionExecute:
			reply := ""
			toolName, args, directReply, ok := mapIntentToExecution(intentResult)
			if directReply != "" {
				reply = directReply
				o.replySafe(s, reply)
				updateConversationStateAfterReply(state, string(intentResult.Intent), reply)
				o.appendConversationTurn(s.User.ID, text, reply)
				return
			}
			if ok && toolName != "" {
				if o.executeToolAndRespond(s, state, string(intentResult.Intent), text, toolName, args) {
					return
				}
			}
			// fallthrough to existing router/LLM if no safe deterministic mapping
		case DecisionConfirm:
			if intentResult.Intent == IntentDeleteMeal {
				scope, _ := intentResult.Entities["scope"].(string)
				if scope == "meal_type" {
					mealType, _ := intentResult.Entities["meal_type"].(string)
					p := newPendingAction("delete_meal", "delete_scope_choice", map[string]any{"meal_type": mealType}, "", nil, traceID, s.MessageID)
					setPendingAction(state, p)
					msg := "Do you want to delete all " + strings.ToLower(mealType) + " entries today, or just one item?"
					o.replySafe(s, msg)
					updateConversationStateAfterReply(state, "confirm_delete_scope", msg)
					o.appendConversationTurn(s.User.ID, text, msg)
					return
				}
				if scope == "all_today" {
					p := newPendingAction("clear_day", "confirmation", map[string]any{"scope": "all_today"}, "clear_all_meals_today", map[string]any{}, traceID, s.MessageID)
					setPendingAction(state, p)
					msg := "Please confirm: clear all meals logged today? (yes/no)"
					o.replySafe(s, msg)
					updateConversationStateAfterReply(state, "confirm_clear_day", msg)
					o.appendConversationTurn(s.User.ID, text, msg)
					return
				}
				if scope == "single_id" {
					raw, _ := intentResult.Entities["meal_id_raw"].(string)
					if mealID, err := strconv.Atoi(raw); err == nil {
						p := newPendingAction("delete_meal", "confirmation", map[string]any{"scope": "single_id"}, "modify_logged_meal", map[string]any{"action": "delete", "meal_id": float64(mealID)}, traceID, s.MessageID)
						setPendingAction(state, p)
						msg := "Please confirm delete for meal id " + raw + ". (yes/no)"
						o.replySafe(s, msg)
						updateConversationStateAfterReply(state, "confirm_delete_id", msg)
						o.appendConversationTurn(s.User.ID, text, msg)
						return
					}
				}
			}
			// If we cannot produce a concrete pending delete command, fall through to legacy deterministic/LLM paths.
		}
	}

	decision := routeWhatsAppMessage(text, state)
	s.Logger.Info("V2 route decision", "intent", decision.Intent, "tool", decision.ToolName, "needs_llm", decision.NeedsLLM)
	if decision.Intent == "crud_format_clarification" {
		decision.NeedsLLM = true
	}
	if !decision.NeedsLLM {
		if decision.DirectReply != "" {
			o.replySafe(s, decision.DirectReply)
			updateConversationStateAfterReply(state, decision.Intent, decision.DirectReply)
			o.appendConversationTurn(s.User.ID, text, decision.DirectReply)
			return
		}
		if decision.ToolName != "" {
			if o.executeToolAndRespond(s, state, decision.Intent, text, decision.ToolName, decision.Args) {
				return
			}
			s.Logger.Warn("Deterministic route tool execution failed", "tool", decision.ToolName)
		}
	}

	// 5. Call LLM
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
			if shouldBlockCorrectionLogMisfire(text, synthetic) {
				msg := correctionSingleItemPrompt()
				o.replySafe(s, msg)
				updateConversationStateAfterReply(state, "llm_correction_guard", msg)
				o.appendAssistantMessage(s.User.ID, msg)
				return
			}
			synthetic = rewriteCorrectionLogMisfire(text, synthetic)
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
				human := deterministicToolReply(result)
				if human == "" || human == "Done." {
					human = "Done."
				}
				o.replySafe(s, human)
				updateConversationStateAfterTool(state, "llm", synthetic.Function.Name, resp)
				o.appendAssistantMessage(s.User.ID, human)
				return
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
				human := deterministicToolReply(result)
				if human == "" || human == "Done." {
					human = "Done."
				}
				o.replySafe(s, human)
				updateConversationStateAfterTool(state, "llm", inferred.Function.Name, resp)
				o.appendAssistantMessage(s.User.ID, human)
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
	mealCreateExecuted := false
	for _, tc := range uniqueToolCalls {
		if shouldBlockCorrectionLogMisfire(text, tc) {
			toolResults = append(toolResults, toolExecutionResult{
				ToolName: "modify_logged_meal",
				Response: map[string]any{
					"ok":      false,
					"error":   "correction_requires_update",
					"message": correctionSingleItemPrompt(),
				},
			})
			continue
		}
		execToolCall := rewriteCorrectionLogMisfire(text, tc)
		if isMealCreateToolCall(execToolCall) {
			if mealCreateExecuted {
				toolResults = append(toolResults, toolExecutionResult{
					ToolName: execToolCall.Function.Name,
					Response: map[string]any{
						"ok":      false,
						"error":   "duplicate_write_blocked",
						"message": "I skipped an extra meal logging action in this message to avoid duplicate entries.",
					},
				})
				continue
			}
			mealCreateExecuted = true
		}
		resp, err := o.Registry.Execute(s, execToolCall)
		if err != nil {
			s.Logger.Warn("Tool Execution Error", "tool", execToolCall.Function.Name, "error", err)
			toolResponses = append(toolResponses, fmt.Sprintf("I tried to use '%s' but it's not available right now.", execToolCall.Function.Name))
			continue
		}

		toolResults = append(toolResults, toolExecutionResult{
			ToolName: execToolCall.Function.Name,
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
	finalRaw := "Done."
	if len(toolResults) > 0 {
		if human := deterministicToolReply(toolResults); human != "" && human != "Done." {
			finalRaw = human
		}
	}
	if finalRaw == "Done." {
		finalRaw = "I finished that request."
	}
	o.replySafe(s, finalRaw)
	updateConversationStateAfterTool(state, "llm", firstToolName(toolResults), marshalToolResults(toolResults))
	o.appendAssistantMessage(s.User.ID, finalRaw)
}

func correctionSingleItemPrompt() string {
	return "I treated that as a correction. Please use '<item> is actually <new quantity>' so I update only that single item."
}

func rewriteCorrectionLogMisfire(text string, tc llm.ToolCall) llm.ToolCall {
	if strings.ToLower(strings.TrimSpace(tc.Function.Name)) != "log_meals" {
		return tc
	}
	decision, ok := parseDeterministicCRUD(text)
	if !ok || decision.ToolName != "modify_logged_meal" {
		return tc
	}
	return buildToolCall(decision.ToolName, decision.Args)
}

func shouldBlockCorrectionLogMisfire(text string, tc llm.ToolCall) bool {
	if strings.ToLower(strings.TrimSpace(tc.Function.Name)) != "log_meals" {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	isCorrection := strings.Contains(lower, " is actually ") ||
		strings.HasPrefix(lower, "no it is ") ||
		strings.HasPrefix(lower, "it is ") ||
		strings.HasPrefix(lower, "it's ") ||
		strings.Contains(lower, " wrong") ||
		strings.Contains(lower, " incorrect") ||
		strings.Contains(lower, " typo") ||
		strings.Contains(lower, "change ") ||
		strings.Contains(lower, "update ") ||
		strings.Contains(lower, "correct ") ||
		strings.Contains(lower, " should be ") ||
		strings.HasPrefix(lower, "should be ") ||
		strings.Contains(lower, " meant ") ||
		strings.HasPrefix(lower, "meant ") ||
		strings.Contains(lower, " instead ") ||
		strings.Contains(lower, " replace ") ||
		strings.HasPrefix(lower, "replace ") ||
		strings.Contains(lower, " that was ") ||
		strings.HasPrefix(lower, "that was ")
	if !isCorrection {
		return false
	}
	decision, ok := parseDeterministicCRUD(text)
	return !ok || decision.ToolName != "modify_logged_meal"
}

func isMealCreateToolCall(tc llm.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(tc.Function.Name))
	if name == "log_meals" {
		return true
	}
	if name != "modify_logged_meal" {
		return false
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return false
	}
	action, _ := args["action"].(string)
	return strings.EqualFold(strings.TrimSpace(action), "add")
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
	state, _ := services.ComputeRemainingDayState(user.ID, nowForUser(user.ID))
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
	case "log_meals":
		if m, ok := r.Response.(map[string]any); ok {
			entries, _ := m["logged_meals"].([]any)
			if len(entries) == 0 {
				return "I couldn't log that meal. Please rephrase once."
			}
			lines := make([]string, 0, len(entries)+2)
			for _, e := range entries {
				em, _ := e.(map[string]any)
				okv, _ := em["ok"].(bool)
				name, _ := em["dish_name"].(string)
				if !okv {
					errCode, _ := em["error"].(string)
					if strings.TrimSpace(name) == "" {
						name = "item"
					}
					lines = append(lines, fmt.Sprintf("- %s: could not log (%s)", name, errCode))
					continue
				}
				cal := getFloat(em["calories"])
				pro := getFloat(em["protein"])
				carbs := getFloat(em["carbs"])
				fat := getFloat(em["fat"])
				fiber := getFloat(em["fiber"])
				displayTime, _ := em["display_time"].(string)
				servingSize, _ := em["serving_size"].(string)
				if displayTime == "" {
					displayTime = "now"
				}
				if strings.TrimSpace(servingSize) == "" {
					servingSize = "1 serving"
				}
				lines = append(lines, fmt.Sprintf("- %s [%s] at %s: %.0f kcal, P %.1fg, C %.1fg, F %.1fg, Fi %.1fg", name, servingSize, displayTime, cal, pro, carbs, fat, fiber))
			}
			out := "Logged meals:\n" + strings.Join(lines, "\n")
			if remaining, ok := m["remaining"].(map[string]any); ok {
				out += fmt.Sprintf("\n\nRemaining today: %.0f kcal, P %.1fg, C %.1fg, F %.1fg, Fi %.1fg.",
					getFloat(remaining["calories"]),
					getFloat(remaining["protein"]),
					getFloat(remaining["carbs"]),
					getFloat(remaining["fat"]),
					getFloat(remaining["fiber"]),
				)
			}
			return out
		}
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
			if errType, _ := m["error"].(string); errType != "" {
				switch errType {
				case "meal_not_found":
					return "I couldn't find that meal in today's logs. Reply with the exact dish name from summary, or ask for today's summary first."
				case "target_dish_and_ingredients_required":
					return "Please use: '<dish> is actually <new quantity>' so I can update only that item."
				case "nutrition_estimation_failed":
					fallthrough
				case ErrCodeNutritionEstimateFailed:
					return "I couldn't estimate nutrition for that correction. Please retry with clearer quantity (for example: 'curd is actually 100g')."
				default:
					return "I couldn't apply that meal change. Please retry using 'delete <dish>' or '<dish> is actually <new quantity>'."
				}
			}
			if msg, _ := m["message"].(string); strings.TrimSpace(msg) != "" {
				return strings.TrimSpace(msg)
			}
			if okv, _ := m["ok"].(bool); okv {
				action, _ := m["action"].(string)
				dish, _ := m["dish_name"].(string)
				mealType, _ := m["meal_type"].(string)
				if dish == "" {
					dish = "meal"
				}
				switch strings.ToLower(strings.TrimSpace(action)) {
				case "delete":
					if mealType != "" {
						return fmt.Sprintf("Deleted %s from %s.", dish, mealType)
					}
					return fmt.Sprintf("Deleted %s.", dish)
				case "update":
					cal := getFloat(m["calories"])
					pro := getFloat(m["protein"])
					carbs := getFloat(m["carbs"])
					fat := getFloat(m["fat"])
					fiber := getFloat(m["fiber"])
					servingSize, _ := m["serving_size"].(string)
					if strings.TrimSpace(servingSize) == "" {
						servingSize = "1 serving"
					}
					if cal > 0 || pro > 0 || carbs > 0 || fat > 0 || fiber > 0 {
						return fmt.Sprintf("Updated %s [%s]: %.0f kcal, P %.1fg, C %.1fg, F %.1fg, Fi %.1fg.", dish, servingSize, cal, pro, carbs, fat, fiber)
					}
					return fmt.Sprintf("Updated %s.", dish)
				case "add":
					cal := getFloat(m["calories"])
					pro := getFloat(m["protein"])
					carbs := getFloat(m["carbs"])
					fat := getFloat(m["fat"])
					fiber := getFloat(m["fiber"])
					servingSize, _ := m["serving_size"].(string)
					if strings.TrimSpace(servingSize) == "" {
						servingSize = "1 serving"
					}
					if mealType != "" {
						return fmt.Sprintf("Added %s [%s] to %s: %.0f kcal, P %.1fg, C %.1fg, F %.1fg, Fi %.1fg.", dish, servingSize, mealType, cal, pro, carbs, fat, fiber)
					}
					return fmt.Sprintf("Added %s.", dish)
				}
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
	case "clear_all_meals_today":
		if m, ok := r.Response.(map[string]any); ok {
			return fmt.Sprintf("Cleared %.0f meals from today.", getFloat(m["deleted_count"]))
		}
	case "set_daily_goal":
		if m, ok := r.Response.(map[string]any); ok {
			if !getBool(m["ok"]) {
				return "I couldn't update your goal. Please share a valid calorie target."
			}
			return fmt.Sprintf("Updated your daily calorie goal to %.0f kcal.", getFloat(m["daily_calorie_target"]))
		}
	case "ask_advice":
		if m, ok := r.Response.(map[string]any); ok {
			decision, _ := m["decision"].(string)
			reason, _ := m["reason"].(string)
			if decision == "" {
				decision = "Here is your guidance."
			}
			if strings.TrimSpace(reason) != "" {
				return strings.TrimSpace(decision) + " " + strings.TrimSpace(reason)
			}
			return strings.TrimSpace(decision)
		}
	case "get_food_nutrition":
		if m, ok := r.Response.(map[string]any); ok {
			if !getBool(m["ok"]) {
				return "I couldn't estimate nutrition for that item. Please retry with quantity, like '100g carrot' or '1 carrot'."
			}
			food, _ := m["food_description"].(string)
			estimated, _ := m["estimated"].(map[string]any)
			cal := getFloat(estimated["calories"])
			pro := getFloat(estimated["protein"])
			carbs := getFloat(estimated["carbs"])
			fat := getFloat(estimated["fat"])
			fiber := getFloat(estimated["fiber"])
			servingSize, _ := estimated["serving_size"].(string)
			if strings.TrimSpace(servingSize) == "" {
				servingSize = "1 serving"
			}
			if strings.TrimSpace(food) == "" {
				food = "that food"
			}
			return fmt.Sprintf("Estimated nutrition for %s (%s): %.0f kcal, P %.1fg, C %.1fg, F %.1fg, Fi %.1fg.", food, servingSize, cal, pro, carbs, fat, fiber)
		}
	case "update_user_profile":
		return "Profile updated."
	case "update_pantry":
		return "Pantry updated."
	case "get_past_day_summary":
		if m, ok := r.Response.(map[string]any); ok {
			date, _ := m["date"].(string)
			count := getCount(m["count"])
			if count == 0 {
				if date == "" {
					return "No meals found for that date."
				}
				return fmt.Sprintf("No meals found for %s.", date)
			}
			out := "Past day summary:\n"
			if lines, ok := m["lines"].([]any); ok {
				for _, l := range lines {
					if s, ok := l.(string); ok {
						out += s + "\n"
					}
				}
			}
			return strings.TrimSpace(out)
		}
	case "create_recipe":
		if m, ok := r.Response.(map[string]any); ok {
			name, _ := m["name"].(string)
			if name == "" {
				name = "recipe"
			}
			return fmt.Sprintf("Saved recipe: %s.", name)
		}
	case "get_pantry":
		if m, ok := r.Response.(map[string]any); ok {
			if items, ok := m["items"].([]any); ok {
				if len(items) == 0 {
					return "Your pantry is empty."
				}
				out := "Pantry items:\n"
				for _, it := range items {
					if s, ok := it.(string); ok {
						out += s + "\n"
					}
				}
				return strings.TrimSpace(out)
			}
		}
	case "get_recipes":
		if m, ok := r.Response.(map[string]any); ok {
			if recipes, ok := m["recipes"].([]any); ok {
				if len(recipes) == 0 {
					return "No saved recipes yet."
				}
				out := "Saved recipes:\n"
				for _, it := range recipes {
					if s, ok := it.(string); ok {
						out += s + "\n"
					}
				}
				return strings.TrimSpace(out)
			}
		}
	case "delete_recipe":
		if m, ok := r.Response.(map[string]any); ok {
			if !getBool(m["ok"]) {
				return "Recipe not found."
			}
			name, _ := m["deleted_recipe"].(string)
			if name == "" {
				return "Recipe deleted."
			}
			return fmt.Sprintf("Deleted recipe: %s.", name)
		}
	case "get_meal_log_time":
		if m, ok := r.Response.(map[string]any); ok {
			if !getBool(m["found"]) {
				return "I couldn't find that meal log."
			}
			if meal, ok := m["meal"].(map[string]any); ok {
				name, _ := meal["name"].(string)
				ts, _ := meal["logged_at"].(string)
				if name != "" && ts != "" {
					return fmt.Sprintf("%s was logged at %s.", name, ts)
				}
			}
		}
	case "get_recent_meals":
		if m, ok := r.Response.(map[string]any); ok {
			if meals, ok := m["meals"].([]any); ok {
				if len(meals) == 0 {
					return "No recent meals found."
				}
				out := "Recent meals:\n"
				for _, mi := range meals {
					if mm, ok := mi.(map[string]any); ok {
						name, _ := mm["name"].(string)
						mt, _ := mm["meal_type"].(string)
						cal := getFloat(mm["calories"])
						out += fmt.Sprintf("- %s (%s): %.0f kcal\n", name, mt, cal)
					}
				}
				return strings.TrimSpace(out)
			}
		}
	case "get_active_goal":
		if m, ok := r.Response.(map[string]any); ok {
			if !getBool(m["has_active_goal"]) {
				return "No active goal set."
			}
			if g, ok := m["goal"].(map[string]any); ok {
				return fmt.Sprintf("Active goal: %.0f kcal, P %.1fg, C %.1fg, F %.1fg, Fi %.1fg.",
					getFloat(g["daily_calorie_target"]),
					getFloat(g["daily_protein_target"]),
					getFloat(g["daily_carbs_target"]),
					getFloat(g["daily_fat_target"]),
					getFloat(g["daily_fiber_target"]),
				)
			}
		}
	case "get_user_profile":
		if m, ok := r.Response.(map[string]any); ok {
			if !getBool(m["has_profile"]) {
				return "No profile saved yet."
			}
			return "Profile details fetched."
		}
	case "get_recent_orders":
		if m, ok := r.Response.(map[string]any); ok {
			if orders, ok := m["orders"].([]any); ok {
				if len(orders) == 0 {
					return "No recent orders found."
				}
				return fmt.Sprintf("Found %d recent orders.", len(orders))
			}
		}
	}
	return "Done."
}

func (o *Orchestrator) executeToolAndRespond(s *Session, state *models.ConversationState, intent, userText, toolName string, args map[string]interface{}) bool {
	resp, err := o.Registry.Execute(s, buildToolCall(toolName, args))
	if err != nil {
		s.Logger.Warn("Deterministic tool execution failed", "tool", toolName, "error", err, "trace_id", newTraceID())
		return false
	}
	parsed := parseToolResponse(resp)
	if ok, errCode := validateMutatingToolAck(toolName, parsed); !ok {
		reply := "Something went wrong while saving your request. Nothing was saved. Please retry."
		o.replySafe(s, reply)
		updateConversationStateAfterReply(state, intent, reply)
		o.appendConversationTurn(s.User.ID, userText, reply)
		s.Logger.Error("Mutating tool acknowledgment validation failed", "tool", toolName, "error_code", errCode, "trace_id", newTraceID())
		return true
	}
	result := []toolExecutionResult{{ToolName: toolName, Response: parsed}}
	reply := deterministicToolReply(result)
	if reply == "" || reply == "Done." {
		reply = "Done."
	}
	o.replySafe(s, reply)
	updateConversationStateAfterTool(state, intent, toolName, resp)
	o.appendConversationTurn(s.User.ID, userText, reply)
	return true
}

func validateMutatingToolAck(toolName string, response any) (bool, string) {
	m, ok := response.(map[string]any)
	if !ok {
		return true, ""
	}
	switch toolName {
	case "log_meals":
		if okv, _ := m["ok"].(bool); !okv {
			return false, ErrCodeWriteFailed
		}
		entries, _ := m["logged_meals"].([]any)
		if len(entries) == 0 {
			return false, ErrCodeNoMealsLogged
		}
		for _, entry := range entries {
			em, _ := entry.(map[string]any)
			if okv, _ := em["ok"].(bool); !okv {
				return false, ErrCodeWriteFailed
			}
			if getFloat(em["meal_id"]) <= 0 {
				return false, ErrCodeReadbackFailed
			}
		}
	case "modify_logged_meal", "clear_all_meals_today", "set_daily_goal", "update_user_profile", "update_pantry":
		if okv, exists := m["ok"]; exists && !getBool(okv) {
			return false, ErrCodeWriteFailed
		}
	}
	return true, ""
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

func getBool(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	default:
		return false
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
		last := len(ids) - 1
		if last < 0 {
			last = 0
		}
		o.replySafe(s, fmt.Sprintf("Please reply with a number between 0 and %d to choose the meal.", last))
		return true
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
		meta := getPendingSelectionMeta(st)
		newIngredients, _ := meta["pending_update_ingredients"].(string)
		if strings.TrimSpace(newIngredients) == "" {
			o.replySafe(s, "I need the corrected ingredients before I can update this meal. Please resend the correction.")
			return true
		}
		args["new_ingredients"] = newIngredients
	}
	tc := buildToolCall("modify_logged_meal", args)
	resp, err := o.Registry.Execute(s, tc)
	if err != nil || strings.TrimSpace(resp) == "" {
		return false
	}
	clearPendingMealSelection(st)
	result := []toolExecutionResult{{ToolName: "modify_logged_meal", Response: parseToolResponse(resp)}}
	reply := deterministicToolReply(result)
	if reply == "" || reply == "Done." {
		reply = "Done."
	}
	o.replySafe(s, reply)
	updateConversationStateAfterTool(st, "pending_meal_resolution", "modify_logged_meal", resp)
	o.appendConversationTurn(s.User.ID, text, reply)
	return true
}

func resolveMealChoiceFromText(text string, ids []uint) (uint, bool) {
	choice := strings.ToLower(strings.TrimSpace(text))
	choice = strings.Trim(choice, ".,!?")

	if !regexp.MustCompile(`^\d+$`).MatchString(choice) {
		switch choice {
		case "first":
			if len(ids) >= 1 {
				return ids[0], true
			}
			return 0, false
		case "second":
			if len(ids) >= 2 {
				return ids[1], true
			}
			return 0, false
		case "last":
			if len(ids) >= 1 {
				return ids[len(ids)-1], true
			}
			return 0, false
		default:
			return 0, false
		}
	}

	if n, err := strconv.Atoi(choice); err == nil {
		if n >= 0 && n < len(ids) {
			return ids[n], true
		}
		return 0, false
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

func newTraceID() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	var out strings.Builder
	for i := 0; i < 12; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			out.WriteByte('0')
			continue
		}
		out.WriteByte(alphabet[n.Int64()])
	}
	return out.String()
}

func mapIntentToExecution(result IntentResult) (toolName string, args map[string]any, directReply string, ok bool) {
	switch result.Intent {
	case IntentHelp:
		return "", nil, "I can help you log meals, get today's summary, check budget, and update goals/profile.", true
	case IntentGetSummary:
		return "get_daily_summary", map[string]any{}, "", true
	case IntentGetBudget:
		return "get_leftover_budget", map[string]any{}, "", true
	case IntentDeleteMeal:
		scope, _ := result.Entities["scope"].(string)
		if scope == "single_id" {
			raw, _ := result.Entities["meal_id_raw"].(string)
			if mealID, err := strconv.Atoi(raw); err == nil {
				return "modify_logged_meal", map[string]any{"action": "delete", "meal_id": float64(mealID)}, "", true
			}
		}
		return "", nil, "", false
	}
	return "", nil, "", false
}
