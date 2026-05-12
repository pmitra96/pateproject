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
	"github.com/pmitra96/pateproject/logger"
	"github.com/pmitra96/pateproject/models"
	"github.com/pmitra96/pateproject/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Orchestrator struct {
	Registry *ToolRegistry
}

type llmCallResult struct {
	msg   *llm.Message
	usage llm.Usage
	err   error
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
	if config.GetTurnPipelineV2Mode() == "launch" {
		o.processMessageLaunchFSM(s, text, imageBase64)
		return
	}
	o.processMessageControl(s, text, imageBase64)
}

func (o *Orchestrator) processMessageControl(s *Session, text string, imageBase64 string) {
	traceID := newTraceID()
	turnStart := time.Now()
	s.Logger.Info("ProcessMessage started", "trace_id", traceID, "text_len", len(text), "has_image", imageBase64 != "")
	defer func() {
		s.Logger.Info("Turn latency summary", "trace_id", traceID, "message_id", strings.TrimSpace(s.MessageID), "total_ms", time.Since(turnStart).Milliseconds())
	}()
	// 1. Daily Limit Check
	appEnv := strings.ToLower(strings.TrimSpace(config.GetEnv("APP_ENV", "development")))
	if appEnv == "production" {
		var count int64
		nowLocal := nowForUser(s.User.ID)
		todayStart, todayEnd := dayWindow(nowLocal)
		database.DB.Model(&models.LLMUsageLog{}).
			Where("user_id = ? AND created_at >= ? AND created_at < ?", s.User.ID, todayStart, todayEnd).
			Count(&count)
		if count >= int64(config.GetWhatsAppDailyLimit()) {
			s.Logger.Warn("User reached daily limit")
			o.replySafe(s, MsgErrorLimit)
			o.persistTurnAndStateAtomic(s.MessageID, s.User.ID, text, MsgErrorLimit, nil, "limit_reached", "", MsgErrorLimit)
			return
		}
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
	clearStalePendingSelectionIfNeeded(state)
	if handled := o.tryResolvePendingMealSelection(s, state, text); handled {
		return
	}
	if toolName, toolArgs, handled, pendingReply := resolvePendingActionChoice(state, s.MessageID, text); handled {
		if pendingReply != "" {
			o.finalizeReply(s, state, text, pendingReply, "pending_action", "", pendingReply)
			return
		}
		if o.executeToolAndRespond(s, state, "pending_action_execute", text, toolName, toolArgs) {
			return
		}
	}
	decision := routeWhatsAppMessage(text, state)

	overridden, hasOverride := classifyIntentOverrides(text)
	llmClient := llm.NewClient()
	intentResult := overridden
	intentClassifierMode := config.GetIntentClassifierMode()
	intentClassifierEnabled := intentClassifierMode == "launch"
	runClassifier := intentClassifierEnabled && (hasOverride || looksLikeMealCRUD(strings.ToLower(strings.TrimSpace(text))) || decision.NeedsLLM)
	if !intentClassifierEnabled {
		s.Logger.Info("Intent classifier disabled by config", "mode", intentClassifierMode, "trace_id", traceID)
	}
	if runClassifier && !hasOverride {
		classifierStart := time.Now()
		classifier := newIntentClassifier(llmClient)
		classified, err := classifier.Classify(text, userContext, traceID)
		s.Logger.Info("Latency stage", "trace_id", traceID, "message_id", strings.TrimSpace(s.MessageID), "stage", "intent_classifier", "duration_ms", time.Since(classifierStart).Milliseconds(), "ok", err == nil)
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
				o.finalizeReply(s, state, text, reply, string(intentResult.Intent), "", reply)
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
					msg := "Choose one option:\n0. Delete all " + strings.ToLower(mealType) + " entries today\n1. Delete one item by dish name"
					o.finalizeReply(s, state, text, msg, "confirm_delete_scope", "", msg)
					return
				}
				if scope == "all_today" {
					p := newPendingAction("clear_day", "confirmation", map[string]any{"scope": "all_today"}, "clear_all_meals_today", map[string]any{}, traceID, s.MessageID)
					setPendingAction(state, p)
					msg := "Please confirm: clear all meals logged today? (yes/no)"
					o.finalizeReply(s, state, text, msg, "confirm_clear_day", "", msg)
					return
				}
				if scope == "single_id" {
					raw, _ := intentResult.Entities["meal_id_raw"].(string)
					if mealID, err := strconv.Atoi(raw); err == nil {
						p := newPendingAction("delete_meal", "confirmation", map[string]any{"scope": "single_id"}, "modify_logged_meal", map[string]any{"action": "delete", "meal_id": float64(mealID)}, traceID, s.MessageID)
						setPendingAction(state, p)
						msg := "Please confirm delete for meal id " + raw + ". (yes/no)"
						o.finalizeReply(s, state, text, msg, "confirm_delete_id", "", msg)
						return
					}
				}
			}
			// If we cannot produce a concrete pending delete command, fall through to legacy deterministic/LLM paths.
		}
	}

	resolvedIntent := strings.TrimSpace(decision.Intent)
	if runClassifier && intentResult.Intent != "" && intentResult.Intent != IntentFallback {
		resolvedIntent = strings.TrimSpace(string(intentResult.Intent))
	}
	if mealParserV1Enabled() && resolvedIntent == string(IntentLogMeal) {
		if o.tryMealParserV1Log(s, state, text, userContext) {
			return
		}
	}

	s.Logger.Info("V2 route decision", "intent", decision.Intent, "tool", decision.ToolName, "needs_llm", decision.NeedsLLM)
	if decision.Intent == "crud_format_clarification" {
		decision.NeedsLLM = true
	}
	if !decision.NeedsLLM {
		if decision.DirectReply != "" {
			o.finalizeReply(s, state, text, decision.DirectReply, decision.Intent, "", decision.DirectReply)
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
	s.Logger.Info("Calling LLM", "trace_id", traceID, "history_len", len(history))
	startTime := time.Now()
	history = compactHistoryForLLM(history, 12)
	llmTimeout := 25 * time.Second
	resCh := make(chan llmCallResult, 1)
	go func() {
		msg, usage, err := llmClient.ProcessWhatsAppConversation(text, imageBase64, history, userContext)
		resCh <- llmCallResult{msg: msg, usage: usage, err: err}
	}()
	var assistantMsg *llm.Message
	var usage llm.Usage
	var err error
	select {
	case res := <-resCh:
		assistantMsg = res.msg
		usage = res.usage
		err = res.err
	case <-time.After(llmTimeout):
		msg := "I’m taking too long to process that right now. Please retry once."
		s.Logger.Error("LLM Processing Timeout", "trace_id", traceID, "timeout_ms", llmTimeout.Milliseconds())
		o.finalizeReply(s, state, text, msg, "llm_timeout", "", msg)
		return
	}
	duration := time.Since(startTime)
	s.Logger.Info("Latency stage", "trace_id", traceID, "message_id", strings.TrimSpace(s.MessageID), "stage", "llm_primary", "duration_ms", duration.Milliseconds(), "ok", err == nil)
	contentPreview, toolCalls := llmResponsePreview(assistantMsg)
	s.Logger.Info(
		"LLM call returned",
		"trace_id", traceID,
		"duration_ms", duration.Milliseconds(),
		"has_error", err != nil,
		"tool_call_count", len(toolCalls),
		"tool_calls", toolCalls,
		"content_preview", contentPreview,
	)

	if err != nil {
		s.Logger.Error("LLM Processing Error", "error", err, "duration_ms", duration.Milliseconds())
		if heuristic := o.tryHeuristicFallback(s, text); heuristic != "" {
			o.finalizeReply(s, state, text, heuristic, "heuristic_fallback", "", heuristic)
			return
		}
		o.finalizeReply(s, state, text, MsgErrorBrain, "llm_error", "", MsgErrorBrain)
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

	// 8. Handle Conversational vs Tool Replies
	if len(assistantMsg.ToolCalls) == 0 {
		content := o.extractText(assistantMsg)
		if synthetic, ok := synthesizeToolCallFromActionText(content); ok {
			if shouldBlockCorrectionLogMisfire(text, synthetic) {
				msg := correctionSingleItemPrompt()
				o.finalizeReply(s, state, text, msg, "llm_correction_guard", "", msg)
				return
			}
			synthetic = rewriteCorrectionLogMisfire(text, synthetic)
			if ok, guardReply := mutationToolCallBoundToTurn(text, synthetic); !ok {
				clearMutationFailureContext(state)
				o.finalizeReply(s, state, text, guardReply, "mutation_turn_guard", "", guardReply)
				return
			}
			resp, err := o.Registry.Execute(s, synthetic)
			if err == nil {
				result := []toolExecutionResult{{
					ToolName: synthetic.Function.Name,
					Response: parseToolResponse(resp),
				}}
				if synthetic.Function.Name == "get_daily_summary" {
					if deterministic := deterministicToolReply(result); deterministic != "" && deterministic != "Done." {
						o.finalizeReply(s, state, text, deterministic, "llm", synthetic.Function.Name, resp)
						return
					}
				}
				if finalReply := o.composeFinalReplyFromToolResults(llmClient, text, result); finalReply != "" {
					o.finalizeReply(s, state, text, finalReply, "llm", synthetic.Function.Name, resp)
					return
				}
				human := deterministicToolReply(result)
				if human == "" || human == "Done." {
					human = "Done."
				}
				o.finalizeReply(s, state, text, human, "llm", synthetic.Function.Name, resp)
				return
			}
		}
		if inferred, ok := inferDeterministicToolCall(text); ok {
			if ok, guardReply := mutationToolCallBoundToTurn(text, inferred); !ok {
				clearMutationFailureContext(state)
				o.finalizeReply(s, state, text, guardReply, "mutation_turn_guard", "", guardReply)
				return
			}
			resp, err := o.Registry.Execute(s, inferred)
			if err == nil && resp != "" {
				result := []toolExecutionResult{{ToolName: inferred.Function.Name, Response: parseToolResponse(resp)}}
				if inferred.Function.Name == "get_daily_summary" {
					if deterministic := deterministicToolReply(result); deterministic != "" && deterministic != "Done." {
						o.finalizeReply(s, state, text, deterministic, "llm", inferred.Function.Name, resp)
						return
					}
				}
				if finalReply := o.composeFinalReplyFromToolResults(llmClient, text, result); finalReply != "" {
					o.finalizeReply(s, state, text, finalReply, "llm", inferred.Function.Name, resp)
					return
				}
				human := deterministicToolReply(result)
				if human == "" || human == "Done." {
					human = "Done."
				}
				o.finalizeReply(s, state, text, human, "llm", inferred.Function.Name, resp)
				return
			}
		}
		if content == "" {
			if heuristic := o.tryHeuristicFallback(s, text); heuristic != "" {
				o.finalizeReply(s, state, text, heuristic, "heuristic_fallback", "", heuristic)
				return
			}
			o.finalizeReply(s, state, text, MsgErrorEmpty, "empty_llm_reply", "", MsgErrorEmpty)
		} else {
			s.Logger.Info("Reply mode", "type", "direct_llm")
			o.finalizeReply(s, state, text, content, "llm", "", content)
		}
		return
	}

	// 9. Handle Tool Calls
	uniqueToolCalls, skippedDuplicates := dedupeToolCalls(assistantMsg.ToolCalls)
	if skippedDuplicates > 0 {
		s.Logger.Info("Skipped duplicate tool calls", "duplicates", skippedDuplicates, "requested", len(assistantMsg.ToolCalls), "executing", len(uniqueToolCalls))
	}
	primaryToolCall, hasPrimary := selectPrimaryToolCall(uniqueToolCalls)
	if hasPrimary {
		uniqueToolCalls = []llm.ToolCall{primaryToolCall}
	}

	var toolResponses []string
	var toolResults []toolExecutionResult
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
		if ok, guardReply := mutationToolCallBoundToTurn(text, execToolCall); !ok {
			clearMutationFailureContext(state)
			o.finalizeReply(s, state, text, guardReply, "mutation_turn_guard", "", guardReply)
			return
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
			o.finalizeReply(s, state, text, deterministic, "llm", firstToolName(toolResults), marshalToolResults(toolResults))
			return
		}
	}

	if len(toolResults) > 0 && shouldUseSecondPass(toolResults) {
		if toolExecuted(toolResults, "get_daily_summary") {
			if deterministic := deterministicToolReply(toolResults); deterministic != "" && deterministic != "Done." {
				o.finalizeReply(s, state, text, deterministic, "llm", firstToolName(toolResults), marshalToolResults(toolResults))
				return
			}
		}
		finalReply := o.composeFinalReplyFromToolResults(llmClient, text, toolResults)
		if finalReply != "" {
			s.Logger.Info("Reply mode", "type", "tool_second_pass", "tool_count", len(toolResults))
			o.finalizeReply(s, state, text, finalReply, "llm", firstToolName(toolResults), marshalToolResults(toolResults))
			return
		}
	}

	if len(toolResponses) == 0 {
		o.finalizeReply(s, state, text, MsgErrorEmpty, "tool_empty_reply", "", MsgErrorEmpty)
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
	o.finalizeReply(s, state, text, finalRaw, "llm", firstToolName(toolResults), marshalToolResults(toolResults))
}

func correctionSingleItemPrompt() string {
	return "I treated that as a correction. Please use '<item> is actually <new quantity>' so I update only that single item."
}

func mutationToolCallBoundToTurn(userText string, tc llm.ToolCall) (bool, string) {
	toolName := strings.ToLower(strings.TrimSpace(tc.Function.Name))
	if toolName != "log_meals" && toolName != "modify_logged_meal" {
		return true, ""
	}
	lowerText := strings.ToLower(strings.TrimSpace(userText))
	if lowerText == "" {
		return false, "I need your meal details in this message. Please restate what to log or update."
	}
	// Allow intentional short follow-ups where context carryover is expected.
	if strings.Contains(lowerText, "add it") || strings.Contains(lowerText, "add that") {
		return true, ""
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return false, "I couldn't safely parse that action. Please restate your meal in one message."
	}
	if toolName == "modify_logged_meal" {
		if idNum, ok := args["meal_id"].(float64); ok && idNum > 0 {
			return true, ""
		}
	}

	userTokens := semanticTokens(lowerText)
	if len(userTokens) == 0 {
		return true, ""
	}
	payloadTokens := semanticTokens(extractMutationPayloadText(toolName, args))
	if len(payloadTokens) == 0 {
		return false, "I need your meal details in this message. Please restate what to log or update."
	}
	for tok := range payloadTokens {
		if _, ok := userTokens[tok]; ok {
			return true, ""
		}
	}
	return false, "I may be carrying details from a previous message. Please resend only what you want to log or update right now."
}

func extractMutationPayloadText(toolName string, args map[string]any) string {
	parts := []string{}
	appendIf := func(v any) {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			parts = append(parts, strings.TrimSpace(s))
		}
	}
	if toolName == "log_meals" {
		if meals, ok := args["meals"].([]any); ok {
			for _, one := range meals {
				m, _ := one.(map[string]any)
				appendIf(m["raw_text"])
				appendIf(m["dish_name"])
				appendIf(m["food_name"])
				appendIf(m["ingredients"])
			}
		}
	}
	if toolName == "modify_logged_meal" {
		appendIf(args["target_dish_name"])
		appendIf(args["new_ingredients"])
		appendIf(args["meal_type"])
	}
	return strings.Join(parts, " ")
}

func semanticTokens(s string) map[string]struct{} {
	out := map[string]struct{}{}
	n := normalizeMealText(s)
	if n == "" {
		return out
	}
	stop := map[string]struct{}{
		"i": {}, "had": {}, "have": {}, "a": {}, "an": {}, "the": {}, "and": {}, "to": {}, "for": {}, "of": {}, "in": {}, "on": {}, "is": {}, "it": {}, "my": {}, "with": {}, "was": {}, "today": {}, "now": {},
	}
	for _, t := range strings.Fields(n) {
		if len(t) < 3 {
			continue
		}
		if _, blocked := stop[t]; blocked {
			continue
		}
		out[t] = struct{}{}
	}
	return out
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

// Retained for test coverage and backward-compat helper semantics.
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

func selectPrimaryToolCall(calls []llm.ToolCall) (llm.ToolCall, bool) {
	if len(calls) == 0 {
		return llm.ToolCall{}, false
	}
	priority := func(name string) int {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "modify_logged_meal":
			return 1
		case "log_meals":
			return 2
		case "clear_all_meals_today":
			return 3
		case "set_daily_goal", "update_user_profile", "update_pantry":
			return 4
		case "get_daily_summary", "get_leftover_budget", "ask_advice", "get_food_nutrition":
			return 5
		default:
			return 9
		}
	}
	best := calls[0]
	bestScore := priority(best.Function.Name)
	for _, tc := range calls[1:] {
		score := priority(tc.Function.Name)
		if score < bestScore {
			best = tc
			bestScore = score
		}
	}
	return best, true
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
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var latestConv models.Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&latestConv).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

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
	if err != nil {
		logger.Error("Failed to update conversation history", "user_id", userID, "error", err)
	}
}

func (o *Orchestrator) replySafe(s *Session, text string) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if strings.HasPrefix(lower, "action:") || strings.Contains(lower, "[action:") {
		fallback := "I fetched your data, but had a formatting issue. Please retry once."
		s.Reply(fallback)
		o.persistTurnReply(s, fallback, "", "")
		return
	}
	s.Reply(text)
}

func (o *Orchestrator) finalizeReply(s *Session, st *models.ConversationState, userText, assistantText, intent, toolName, toolResult string) {
	stageStart := time.Now()
	o.replySafe(s, assistantText)
	o.persistTurnAndStateAtomic(s.MessageID, s.User.ID, userText, assistantText, st, intent, toolName, toolResult)
	s.Logger.Info("Latency stage", "message_id", strings.TrimSpace(s.MessageID), "stage", "finalize_reply_persist", "duration_ms", time.Since(stageStart).Milliseconds(), "intent", strings.TrimSpace(intent), "tool", strings.TrimSpace(toolName))
}

func (o *Orchestrator) persistTurnReply(s *Session, assistantText, intent, toolName string) {
	if s == nil || strings.TrimSpace(s.MessageID) == "" {
		return
	}
	// Do not close turn on interim thinking acknowledgements.
	if strings.TrimSpace(assistantText) == MsgThinking || strings.TrimSpace(assistantText) == MsgAnalyzing || strings.TrimSpace(assistantText) == MsgProcessing {
		return
	}
	updates := map[string]any{
		"assistant_text": assistantText,
		"status":         "completed",
		"updated_at":     time.Now(),
	}
	if strings.TrimSpace(intent) != "" {
		updates["last_intent"] = strings.TrimSpace(intent)
	}
	if strings.TrimSpace(toolName) != "" {
		updates["last_tool"] = strings.TrimSpace(toolName)
	}
	_ = database.DB.Model(&models.ConversationTurn{}).
		Where("message_id = ? AND status IN ?", s.MessageID, []string{"processing", "received", "retryable"}).
		Updates(updates).Error
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
	stageStart := time.Now()
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
		logger.L().Warn("Latency stage", "stage", "llm_tool_second_pass", "duration_ms", time.Since(stageStart).Milliseconds(), "ok", false)
		return ""
	}
	logger.L().Info("Latency stage", "stage", "llm_tool_second_pass", "duration_ms", time.Since(stageStart).Milliseconds(), "ok", true)
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
				qtyValue := getFloat(em["quantity_value"])
				qtyUnit, _ := em["quantity_unit"].(string)
				displayQty := strings.TrimSpace(servingSize)
				if qtyValue > 0 && strings.TrimSpace(qtyUnit) != "" {
					if qtyValue == float64(int64(qtyValue)) {
						displayQty = fmt.Sprintf("%d %s", int64(qtyValue), strings.TrimSpace(qtyUnit))
					} else {
						displayQty = fmt.Sprintf("%.1f %s", qtyValue, strings.TrimSpace(qtyUnit))
					}
				}
				if displayTime == "" {
					displayTime = "now"
				}
				if strings.TrimSpace(displayQty) == "" {
					displayQty = "1 serving"
				}
				lines = append(lines, fmt.Sprintf("- %s [%s] at %s: %.0f kcal, P %.1fg, C %.1fg, F %.1fg, Fi %.1fg", name, displayQty, displayTime, cal, pro, carbs, fat, fiber))
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
			errType := toolResponseErrorCode(m)
			if errType == "ambiguous_target" || errType == strings.ToLower(ErrCodeAmbiguousTarget) {
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
			if errType != "" {
				switch errType {
				case "meal_not_found", strings.ToLower(ErrCodeNotFound):
					return "I couldn't find that meal in today's logs. Reply with the exact dish name from summary, or ask for today's summary first."
				case "target_dish_and_ingredients_required", strings.ToLower(ErrCodeInvalidPayload), strings.ToLower(ErrCodeTargetDishClarification):
					return "Please use: '<dish> is actually <new quantity>' so I can update only that item."
				case "nutrition_estimation_failed", strings.ToLower(ErrCodeEstimateUnavailable):
					fallthrough
				case strings.ToLower(ErrCodeNutritionEstimateFailed):
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
			status, _ := m["status"].(string)
			if strings.TrimSpace(decision) == "" {
				switch strings.TrimSpace(status) {
				case "ALLOW":
					decision = "✅ Yes, you can have this."
				case "ALLOW_WITH_CONSTRAINT":
					decision = "⚠️ You can have this, but keep the portion small."
				default:
					decision = "❌ Not a good fit for today."
				}
			}
			out := strings.TrimSpace(decision)
			if strings.TrimSpace(reason) != "" {
				out = out + " " + strings.TrimSpace(reason)
			}
			if estimated, ok := m["estimated"].(map[string]any); ok {
				cal := getFloat(estimated["calories"])
				if cal > 0 {
					out = out + fmt.Sprintf(" (~%.0f kcal)", cal)
				}
			}
			return strings.TrimSpace(out)
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
	toolStart := time.Now()
	traceID, _ := args["trace_id"].(string)
	actionVal, _ := args["action"].(string)
	mut := MealMutationV1{
		TraceID:        traceID,
		Source:         "orchestrator:deterministic",
		Action:         MutationAction(strings.ToLower(strings.TrimSpace(actionVal))),
		MealType:       asString(args["meal_type"]),
		TargetDishName: asString(args["target_dish_name"]),
	}
	if idNum, ok := args["meal_id"].(float64); ok && idNum > 0 {
		mut.MealID = uint(idNum)
	}
	if mut.Action != "" {
		mut.IdempotencyKey = mutationIdempotencyKey(s.MessageID, mut)
		args["idempotency_key"] = mut.IdempotencyKey
	}
	resp, err := o.Registry.Execute(s, buildToolCall(toolName, args))
	s.Logger.Info("Latency stage", "trace_id", strings.TrimSpace(traceID), "message_id", strings.TrimSpace(s.MessageID), "stage", "tool_execute", "tool", toolName, "duration_ms", time.Since(toolStart).Milliseconds(), "ok", err == nil)
	if err != nil {
		s.Logger.Warn("Deterministic tool execution failed", "tool", toolName, "error", err, "trace_id", newTraceID())
		return false
	}
	parsed := parseToolResponse(resp)
	if ok, errCode := validateMutatingToolAck(toolName, parsed); !ok {
		clearMutationFailureContext(state)
		reply := "Something went wrong while saving your request. Nothing was saved. Please retry."
		o.finalizeReply(s, state, userText, reply, intent, "", reply)
		s.Logger.Error("Mutating tool acknowledgment validation failed", "tool", toolName, "error_code", errCode, "trace_id", newTraceID())
		return true
	}
	if failed, errCode := extractMutationFailure(toolName, parsed); failed && shouldClearMutationContextForError(errCode) {
		clearMutationFailureContext(state)
		s.Logger.Info("Cleared mutation context after business failure", "tool", toolName, "error_code", errCode, "message_id", strings.TrimSpace(s.MessageID))
	}
	result := []toolExecutionResult{{ToolName: toolName, Response: parsed}}
	reply := deterministicToolReply(result)
	if reply == "" || reply == "Done." {
		reply = "Done."
	}
	o.finalizeReply(s, state, userText, reply, intent, toolName, resp)
	return true
}

func clearMutationFailureContext(st *models.ConversationState) {
	if st == nil {
		return
	}
	clearPendingMealSelection(st)
	clearPendingAction(st)
}

func asString(v any) string {
	s, _ := v.(string)
	return s
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
			// For meal edits/deletes, some "ok=false" results are expected user-flow outcomes
			// (for example ambiguous match or meal not found) and should be rendered to the user.
			if toolName == "modify_logged_meal" {
				if errType := toolResponseErrorCode(m); strings.TrimSpace(errType) != "" {
					switch errType {
					case "ambiguous_target", "meal_not_found", "target_dish_and_ingredients_required", "nutrition_estimation_failed",
						strings.ToLower(ErrCodeAmbiguousTarget), strings.ToLower(ErrCodeNotFound), strings.ToLower(ErrCodeInvalidPayload), strings.ToLower(ErrCodeEstimateUnavailable), strings.ToLower(ErrCodeTargetDishClarification):
						return true, ""
					}
				}
			}
			return false, ErrCodeWriteFailed
		}
	}
	return true, ""
}

func extractMutationFailure(toolName string, response any) (bool, string) {
	switch toolName {
	case "log_meals", "modify_logged_meal", "clear_all_meals_today":
	default:
		return false, ""
	}
	m, ok := response.(map[string]any)
	if !ok {
		return false, ""
	}
	if okv, exists := m["ok"]; exists {
		if b, ok := okv.(bool); ok && !b {
			if errCode := toolResponseErrorCode(m); strings.TrimSpace(errCode) != "" {
				return true, strings.TrimSpace(errCode)
			}
			return true, ErrCodeWriteFailed
		}
	}
	if errCode := toolResponseErrorCode(m); strings.TrimSpace(errCode) != "" {
		return true, strings.TrimSpace(errCode)
	}
	if toolName == "log_meals" {
		if entries, ok := m["logged_meals"].([]any); ok {
			for _, entry := range entries {
				em, _ := entry.(map[string]any)
				if okv, _ := em["ok"].(bool); !okv {
					if errCode := toolResponseErrorCode(em); strings.TrimSpace(errCode) != "" {
						return true, strings.TrimSpace(errCode)
					}
					return true, ErrCodeWriteFailed
				}
			}
		}
	}
	return false, ""
}

func shouldClearMutationContextForError(errCode string) bool {
	return !IsClarificationErrorCode(errCode)
}

func toolResponseErrorCode(m map[string]any) string {
	if m == nil {
		return ""
	}
	if raw, _ := m["error_code"].(string); strings.TrimSpace(raw) != "" {
		return strings.ToLower(strings.TrimSpace(raw))
	}
	if raw, _ := m["error"].(string); strings.TrimSpace(raw) != "" {
		return strings.ToLower(strings.TrimSpace(raw))
	}
	return ""
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
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var conv models.Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&conv).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

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
	if err != nil {
		logger.Error("Failed to append conversation turn", "user_id", userID, "error", err)
	}
}

func (o *Orchestrator) appendAssistantMessage(userID uint, assistantText string) {
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if strings.TrimSpace(assistantText) == "" {
			return nil
		}
		var conv models.Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&conv).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

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
	if err != nil {
		logger.Error("Failed to append assistant message", "user_id", userID, "error", err)
	}
}

func (o *Orchestrator) persistTurnAndStateAtomic(messageID string, userID uint, userText, assistantText string, st *models.ConversationState, intent, toolName, toolResult string) {
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var conv models.Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&conv).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
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
			if err := tx.Create(&models.Conversation{UserID: userID, Messages: string(raw)}).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&conv).Update("messages", string(raw)).Error; err != nil {
			return err
		}

		if st != nil {
			st.LastIntent = intent
			st.LastTool = toolName
			st.LastToolResult = toolResult
			st.TurnCount++
			st.UpdatedAt = time.Now()
			if err := tx.Model(&models.ConversationState{}).Where("user_id = ?", st.UserID).Updates(map[string]interface{}{
				"last_intent":            st.LastIntent,
				"last_tool":              st.LastTool,
				"last_tool_result":       st.LastToolResult,
				"turn_count":             st.TurnCount,
				"fsm_state":              st.FSMState,
				"fsm_pending_state":      st.FSMPendingState,
				"fsm_state_version":      st.FSMStateVersion,
				"fsm_turn_id":            st.FSMTurnID,
				"fsm_message_id":         st.FSMMessageID,
				"fsm_trace_id":           st.FSMTraceID,
				"fsm_context":            st.FSMContext,
				"fsm_last_error_code":    st.FSMLastErrorCode,
				"fsm_last_transition_at": st.FSMLastTransitionAt,
				"updated_at":             st.UpdatedAt,
			}).Error; err != nil {
				return err
			}
		}

		// Update deterministic per-message turn ledger when available.
		if strings.TrimSpace(messageID) != "" {
			if err := tx.Model(&models.ConversationTurn{}).
				Where("message_id = ? AND status IN ?", strings.TrimSpace(messageID), []string{"processing", "received", "retryable"}).
				Updates(map[string]any{
					"assistant_text": strings.TrimSpace(assistantText),
					"status":         "completed",
					"last_intent":    strings.TrimSpace(intent),
					"last_tool":      strings.TrimSpace(toolName),
					"updated_at":     time.Now(),
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		logger.Error("Failed atomic turn persist", "user_id", userID, "error", err)
	}
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
	if shouldInterruptPendingSelection(text) {
		clearPendingMealSelection(st)
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
		reply := buildPendingSelectionOptionsReply(s.User.ID, ids, last)
		o.finalizeReply(s, st, text, reply, "pending_meal_resolution_prompt", "", reply)
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
			reply := "I need the corrected ingredients before I can update this meal. Please resend the correction."
			o.finalizeReply(s, st, text, reply, "pending_meal_resolution_prompt", "", reply)
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
	o.finalizeReply(s, st, text, reply, "pending_meal_resolution", "modify_logged_meal", resp)
	return true
}

func buildPendingSelectionOptionsReply(userID uint, ids []uint, last int) string {
	if len(ids) == 0 {
		return "I couldn't find matching meals anymore. Please retry your request."
	}
	var meals []models.MealLog
	_ = database.DB.Where("user_id = ? AND id IN ?", userID, ids).Find(&meals).Error
	byID := make(map[uint]models.MealLog, len(meals))
	for _, m := range meals {
		byID[m.ID] = m
	}
	lines := make([]string, 0, len(ids))
	for i, id := range ids {
		if m, ok := byID[id]; ok {
			lines = append(lines, fmt.Sprintf("%d. %s (%s, %s)", i, m.Name, m.MealType, userReadableTime(userID, m.LoggedAt)))
			continue
		}
		lines = append(lines, fmt.Sprintf("%d. meal id %d", i, id))
	}
	return fmt.Sprintf("I found multiple matches. Reply with a number between 0 and %d:\n%s", last, strings.Join(lines, "\n"))
}

func clearStalePendingSelectionIfNeeded(st *models.ConversationState) {
	if st == nil || strings.TrimSpace(st.PendingMealAction) == "" {
		return
	}
	const pendingSelectionTTL = 20 * time.Minute
	if st.UpdatedAt.IsZero() || time.Since(st.UpdatedAt) <= pendingSelectionTTL {
		return
	}
	clearPendingMealSelection(st)
}

func shouldInterruptPendingSelection(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if isGreeting(lower) {
		return true
	}
	if strings.Contains(lower, "help") || strings.Contains(lower, "summary") || strings.Contains(lower, "budget") || strings.Contains(lower, "history") {
		return true
	}
	if strings.HasPrefix(lower, "what ") || strings.HasPrefix(lower, "how ") || strings.HasPrefix(lower, "can i ") {
		return true
	}
	return false
}

func (o *Orchestrator) tryMealParserV1Log(s *Session, st *models.ConversationState, text, userContext string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	client := llm.NewClient()
	s.Logger.Info("Meal parser v1 input", "mode", strings.ToLower(strings.TrimSpace(config.GetEnv("MEAL_PARSER_V1_MODE", "control"))), "text", truncateForLog(text, 1200), "user_context", truncateForLog(userContext, 1200))
	parserStart := time.Now()
	parsed, err := client.ParseMealV1(text, userContext)
	s.Logger.Info("Latency stage", "message_id", strings.TrimSpace(s.MessageID), "stage", "meal_parser_v1", "duration_ms", time.Since(parserStart).Milliseconds(), "ok", err == nil)
	if err != nil || parsed == nil {
		if err != nil {
			s.Logger.Warn("Meal parser v1 failed", "error", err)
		}
		return false
	}
	s.Logger.Info("Meal parser v1 output", "meal_type", parsed.MealType, "item_count", len(parsed.ParsedItems), "clarification_needed", parsed.ClarificationNeeded, "items", summarizeParsedItemsForLog(parsed))
	if parsed.ClarificationNeeded {
		msg := "I need one clarification before logging this meal."
		if parsed.ClarificationQ != nil && strings.TrimSpace(*parsed.ClarificationQ) != "" {
			msg = strings.TrimSpace(*parsed.ClarificationQ)
		}
		o.finalizeReply(s, st, text, msg, "meal_parser_v1_clarification", "", msg)
		return true
	}
	if len(parsed.ParsedItems) == 0 {
		return false
	}
	mealType := canonicalMealTypeFromParsed(parsed.MealType)
	meals := make([]interface{}, 0, len(parsed.ParsedItems))
	for _, item := range parsed.ParsedItems {
		if strings.TrimSpace(item.FoodName) == "" {
			continue
		}
		u := canonicalQuantityUnit(item.Unit)
		ingredientText := buildIngredientsTextFromParsedItem(item)
		mealItem := map[string]interface{}{
			"dish_name":                   strings.TrimSpace(item.FoodName),
			"food_name":                   strings.TrimSpace(item.FoodName),
			"ingredients":                 ingredientText,
			"raw_text":                    strings.TrimSpace(item.RawText),
			"meal_type":                   mealType,
			"quantity_value":              item.Quantity,
			"quantity_unit":               u,
			"cooking_method":              strings.TrimSpace(item.CookingMethod),
			"modifiers":                   item.Modifiers,
			"assumptions":                 item.Assumptions,
			"confidence":                  strings.TrimSpace(item.Confidence),
			"quantity_in_grams_estimated": 0.0,
		}
		if len(item.Ingredients) > 0 {
			ings := make([]map[string]interface{}, 0, len(item.Ingredients))
			for _, ing := range item.Ingredients {
				ings = append(ings, map[string]interface{}{
					"name":      strings.TrimSpace(ing.Name),
					"quantity":  ing.Quantity,
					"unit":      canonicalQuantityUnit(ing.Unit),
					"brand":     strings.TrimSpace(ing.Brand),
					"calories":  ing.Calories,
					"protein_g": ing.ProteinG,
					"carbs_g":   ing.CarbsG,
					"fat_g":     ing.FatG,
					"fiber_g":   ing.FiberG,
				})
			}
			mealItem["ingredients_structured"] = ings
			mealItem["ingredients_source"] = "user_provided"
		} else if strings.TrimSpace(item.RawText) != "" {
			mealItem["ingredients_source"] = "inferred"
		} else {
			mealItem["ingredients_source"] = "missing"
		}
		if item.QuantityInGramsEstimated != nil && *item.QuantityInGramsEstimated > 0 {
			mealItem["quantity_in_grams_estimated"] = *item.QuantityInGramsEstimated
		}
		if strings.TrimSpace(item.Brand) != "" {
			mealItem["brand"] = strings.TrimSpace(item.Brand)
		}
		meals = append(meals, mealItem)
	}
	if len(meals) == 0 {
		return false
	}
	args := map[string]interface{}{"meals": meals}
	return o.executeToolAndRespond(s, st, "meal_parser_v1_log", text, "log_meals", args)
}

func canonicalMealTypeFromParsed(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "breakfast":
		return "Breakfast"
	case "lunch":
		return "Lunch"
	case "dinner":
		return "Dinner"
	case "snack":
		return "Snack"
	default:
		return "Snack"
	}
}

func canonicalQuantityUnit(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "piece":
		return "pcs"
	case "scoop":
		return "scoop"
	case "g", "gm", "gram", "grams":
		return "g"
	case "ml":
		return "ml"
	case "tsp":
		return "tsp"
	case "tbsp":
		return "tbsp"
	case "serving":
		return "serving"
	default:
		return strings.TrimSpace(raw)
	}
}

func truncateForLog(in string, max int) string {
	s := strings.TrimSpace(in)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func llmResponsePreview(msg *llm.Message) (string, []string) {
	if msg == nil {
		return "", nil
	}
	toolCalls := make([]string, 0, len(msg.ToolCalls))
	for _, tc := range msg.ToolCalls {
		name := strings.TrimSpace(tc.Function.Name)
		if name == "" {
			name = "unknown_tool"
		}
		toolCalls = append(toolCalls, name)
	}
	switch c := msg.Content.(type) {
	case string:
		return truncateForLog(c, 320), toolCalls
	case []llm.ContentPart:
		parts := make([]string, 0, len(c))
		for _, p := range c {
			if strings.TrimSpace(p.Text) != "" {
				parts = append(parts, strings.TrimSpace(p.Text))
			}
		}
		return truncateForLog(strings.Join(parts, " "), 320), toolCalls
	default:
		return "", toolCalls
	}
}

func summarizeParsedItemsForLog(parsed *llm.MealParserResult) string {
	if parsed == nil || len(parsed.ParsedItems) == 0 {
		return "[]"
	}
	type itemLog struct {
		FoodName    string `json:"food_name"`
		Quantity    string `json:"quantity"`
		Brand       string `json:"brand"`
		Ingredients int    `json:"ingredient_count"`
		Confidence  string `json:"confidence"`
	}
	items := make([]itemLog, 0, len(parsed.ParsedItems))
	for _, it := range parsed.ParsedItems {
		items = append(items, itemLog{
			FoodName:    truncateForLog(it.FoodName, 80),
			Quantity:    strings.TrimSpace(fmt.Sprintf("%.2f %s", it.Quantity, canonicalQuantityUnit(it.Unit))),
			Brand:       truncateForLog(it.Brand, 60),
			Ingredients: len(it.Ingredients),
			Confidence:  strings.TrimSpace(it.Confidence),
		})
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return "parse_error"
	}
	return truncateForLog(string(raw), 1200)
}

func buildIngredientsTextFromParsedItem(item llm.MealParserItem) string {
	if len(item.Ingredients) == 0 {
		return strings.TrimSpace(item.RawText)
	}
	parts := make([]string, 0, len(item.Ingredients))
	for _, ing := range item.Ingredients {
		name := strings.TrimSpace(ing.Name)
		if name == "" {
			continue
		}
		unit := canonicalQuantityUnit(ing.Unit)
		q := ""
		if ing.Quantity > 0 {
			if ing.Quantity == float64(int64(ing.Quantity)) {
				q = fmt.Sprintf("%d", int64(ing.Quantity))
			} else {
				q = fmt.Sprintf("%.2f", ing.Quantity)
			}
		}
		brand := strings.TrimSpace(ing.Brand)
		if q != "" && unit != "" {
			if brand != "" {
				parts = append(parts, fmt.Sprintf("%s %s %s (%s)", q, unit, name, brand))
			} else {
				parts = append(parts, fmt.Sprintf("%s %s %s", q, unit, name))
			}
			continue
		}
		if brand != "" {
			parts = append(parts, fmt.Sprintf("%s (%s)", name, brand))
		} else {
			parts = append(parts, name)
		}
	}
	if len(parts) == 0 {
		return strings.TrimSpace(item.RawText)
	}
	return strings.Join(parts, ", ")
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

func mealParserV1Enabled() bool {
	mode := strings.ToLower(strings.TrimSpace(config.GetEnv("MEAL_PARSER_V1_MODE", "control")))
	return mode == "launch"
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
