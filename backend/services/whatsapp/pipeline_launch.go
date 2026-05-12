package whatsapp

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/config"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/llm"
	"github.com/pmitra96/pateproject/models"
)

func (o *Orchestrator) processMessageLaunchFSM(s *Session, text string, imageBase64 string) {
	traceID := newTraceID()
	turnStart := time.Now()
	s.Logger.Info("ProcessMessage launch FSM started", "trace_id", traceID, "text_len", len(text), "has_image", imageBase64 != "")
	defer func() {
		s.Logger.Info("Turn latency summary", "trace_id", traceID, "message_id", strings.TrimSpace(s.MessageID), "total_ms", time.Since(turnStart).Milliseconds(), "pipeline_version", "v2_fsm")
	}()

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

	state := getConversationState(s.User.ID)
	state.FSMTurnID = strings.TrimSpace(s.MessageID)
	if state.FSMTurnID == "" {
		state.FSMTurnID = fmt.Sprintf("turn-%d", time.Now().UnixNano())
	}
	if err := transitionFSMState(state, FSMStateClassifying, state.FSMPendingState, traceID, s.MessageID, map[string]interface{}{"stage": "classifying"}); err != nil {
		s.Logger.Warn("fsm transition failed", "error", err, "from", state.FSMState, "to", FSMStateClassifying)
	}

	userContext := o.buildUserContext(s.User)
	intentDecision, isMutation := o.resolveIntentDecisionForLaunch(s, state, text, userContext, traceID)
	if !isMutation {
		// For non-mutation turns, preserve current behavior by delegating to control path.
		o.processMessageControl(s, text, imageBase64)
		_ = transitionFSMState(state, FSMStateReplied, state.FSMPendingState, traceID, s.MessageID, map[string]interface{}{"delegated_to": "control_non_mutation"})
		return
	}
	if len(text) > 10 || imageBase64 != "" {
		o.replySafe(s, MsgThinking)
	}

	var cmd MutationCommand
	if shouldParseLogMeal(intentDecision) {
		if err := transitionFSMState(state, FSMStateParsing, state.FSMPendingState, traceID, s.MessageID, map[string]interface{}{"intent": string(intentDecision.Intent)}); err != nil {
			s.Logger.Warn("fsm transition failed", "error", err, "to", FSMStateParsing)
		}
		parsed, perr := o.parseMealMutationPayload(s, text, userContext)
		if perr != nil {
			s.Logger.Warn("meal parser unavailable in launch pipeline, falling back to control", "error_code", perr.Code, "message", perr.Message)
			o.processMessageControl(s, text, imageBase64)
			_ = transitionFSMState(state, FSMStateReplied, state.FSMPendingState, traceID, s.MessageID, map[string]interface{}{"delegated_to": "control_parser_error"})
			return
		}
		if parsed.ClarificationNeeded {
			msg := parsed.ClarificationQ
			if strings.TrimSpace(msg) == "" {
				msg = "Please clarify your meal details."
			}
			state.FSMPendingState = FSMPendingAwaitingClarification
			_ = transitionFSMState(state, FSMStatePersisting, state.FSMPendingState, traceID, s.MessageID, map[string]interface{}{"clarification": true})
			o.finalizeReply(s, state, text, msg, "meal_parser_v1_clarification", "", msg)
			_ = transitionFSMState(state, FSMStateReplied, state.FSMPendingState, traceID, s.MessageID, map[string]interface{}{"clarification": true})
			return
		}
		if len(parsed.Items) == 0 {
			s.Logger.Warn("meal parser returned no items in launch pipeline, falling back to control")
			o.processMessageControl(s, text, imageBase64)
			_ = transitionFSMState(state, FSMStateReplied, state.FSMPendingState, traceID, s.MessageID, map[string]interface{}{"delegated_to": "control_parser_empty"})
			return
		}
		cmd = buildMutationCommandFromParsed(text, s.MessageID, parsed)
	} else {
		cmd = MutationCommand{
			Intent:    intentDecision.Intent,
			ToolName:  strings.TrimSpace(intentDecision.ToolName),
			ToolArgs:  intentDecision.ToolArgs,
			RawText:   text,
			MessageID: s.MessageID,
		}
	}

	if err := transitionFSMState(state, FSMStateValidating, state.FSMPendingState, traceID, s.MessageID, map[string]interface{}{"tool": cmd.ToolName}); err != nil {
		s.Logger.Warn("fsm transition failed", "error", err, "to", FSMStateValidating)
	}
	if vErr := validateMutationCommand(text, cmd); vErr != nil {
		o.failPipelineTurn(s, state, text, traceID, vErr)
		return
	}

	if err := transitionFSMState(state, FSMStateExecuting, state.FSMPendingState, traceID, s.MessageID, map[string]interface{}{"tool": cmd.ToolName}); err != nil {
		s.Logger.Warn("fsm transition failed", "error", err, "to", FSMStateExecuting)
	}
	outcome := o.executeMutationCommand(s, cmd)
	if !outcome.OK {
		errCode := strings.TrimSpace(outcome.BusinessCode)
		if shouldClearMutationContextForError(errCode) {
			clearMutationFailureContext(state)
		}
		state.FSMLastErrorCode = errCode
		o.failPipelineTurn(s, state, text, traceID, &PipelineError{
			Code:        pickFirstNonEmpty(errCode, ErrCodePipelineStage),
			Message:     strings.TrimSpace(outcome.Reply),
			Recoverable: IsClarificationErrorCode(errCode),
		})
		return
	}

	if err := transitionFSMState(state, FSMStatePersisting, state.FSMPendingState, traceID, s.MessageID, map[string]interface{}{"tool": outcome.ToolName}); err != nil {
		s.Logger.Warn("fsm transition failed", "error", err, "to", FSMStatePersisting)
	}
	o.finalizeReply(s, state, text, outcome.Reply, string(cmd.Intent), outcome.ToolName, outcome.ToolResult)
	_ = transitionFSMState(state, FSMStateReplied, state.FSMPendingState, traceID, s.MessageID, map[string]interface{}{"tool": outcome.ToolName})
}

func (o *Orchestrator) resolveIntentDecisionForLaunch(s *Session, state *models.ConversationState, text, userContext, traceID string) (IntentDecision, bool) {
	decision := routeWhatsAppMessage(text, state)
	intent := IntentName(IntentFallback)
	if strings.TrimSpace(decision.Intent) != "" {
		intent = IntentName(strings.TrimSpace(decision.Intent))
	}
	out := IntentDecision{
		Intent:      intent,
		ToolName:    strings.TrimSpace(decision.ToolName),
		ToolArgs:    decision.Args,
		DirectReply: strings.TrimSpace(decision.DirectReply),
		Source:      "deterministic",
	}

	_, hasOverride := classifyIntentOverrides(text)
	intentClassifierMode := config.GetIntentClassifierMode()
	intentClassifierEnabled := intentClassifierMode == "launch"
	runClassifier := intentClassifierEnabled && (hasOverride || looksLikeMealCRUD(strings.ToLower(strings.TrimSpace(text))) || decision.NeedsLLM)
	if runClassifier {
		classifier := newIntentClassifier(llm.NewClient())
		classified, err := classifier.Classify(text, userContext, traceID)
		if err == nil {
			out.Intent = classified.Intent
			out.Confidence = classified.Confidence
			out.Source = "classifier"
			policy := decideIntentPolicy(classified)
			if policy.Decision == DecisionExecute {
				toolName, args, directReply, ok := mapIntentToExecution(classified)
				if ok {
					out.ToolName = toolName
					out.ToolArgs = args
				}
				if !ok && classified.Intent == IntentLogMeal {
					out.ToolName = "log_meals"
					out.ToolArgs = map[string]any{}
				}
				out.DirectReply = directReply
			}
		}
	}

	if strings.TrimSpace(out.DirectReply) != "" {
		return out, false
	}
	return out, isMutationToolName(out.ToolName) || isMutationIntent(out.Intent)
}

func isMutationIntent(intent IntentName) bool {
	switch strings.ToLower(strings.TrimSpace(string(intent))) {
	case strings.ToLower(string(IntentLogMeal)),
		strings.ToLower(string(IntentModifyMeal)),
		strings.ToLower(string(IntentDeleteMeal)):
		return true
	default:
		return false
	}
}

func isMutationToolName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "log_meals", "modify_logged_meal", "clear_all_meals_today":
		return true
	default:
		return false
	}
}

func shouldParseLogMeal(decision IntentDecision) bool {
	if strings.EqualFold(string(decision.Intent), string(IntentLogMeal)) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(decision.ToolName), "log_meals")
}

func (o *Orchestrator) parseMealMutationPayload(s *Session, text, userContext string) (MealParsePayload, *PipelineError) {
	client := llm.NewClient()
	parsed, err := client.ParseMealV1(text, userContext)
	if err != nil || parsed == nil {
		msg := "meal parser failed"
		if err != nil {
			msg = fmt.Sprintf("meal parser failed: %v", err)
		}
		return MealParsePayload{}, &PipelineError{Code: ErrCodePipelineStage, Message: msg, Recoverable: false}
	}
	out := MealParsePayload{
		MealType:            strings.TrimSpace(parsed.MealType),
		ClarificationNeeded: parsed.ClarificationNeeded,
	}
	if parsed.ClarificationQ != nil {
		out.ClarificationQ = strings.TrimSpace(*parsed.ClarificationQ)
	}
	for _, it := range parsed.ParsedItems {
		item := MealParseItem{
			DishName:      strings.TrimSpace(it.FoodName),
			RawText:       strings.TrimSpace(it.RawText),
			MealType:      canonicalMealTypeFromParsed(parsed.MealType),
			QuantityValue: it.Quantity,
			QuantityUnit:  canonicalQuantityUnit(it.Unit),
		}
		ings := buildIngredientsTextFromParsedItem(it)
		item.Ingredients = strings.TrimSpace(ings)
		if len(it.Ingredients) > 0 {
			item.IngredientsRaw = make([]map[string]interface{}, 0, len(it.Ingredients))
			for _, ing := range it.Ingredients {
				item.IngredientsRaw = append(item.IngredientsRaw, map[string]interface{}{
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
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

func buildMutationCommandFromParsed(text string, messageID string, parsed MealParsePayload) MutationCommand {
	meals := make([]interface{}, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		if strings.TrimSpace(item.DishName) == "" {
			continue
		}
		m := map[string]interface{}{
			"dish_name":      strings.TrimSpace(item.DishName),
			"food_name":      strings.TrimSpace(item.DishName),
			"ingredients":    strings.TrimSpace(item.Ingredients),
			"raw_text":       strings.TrimSpace(item.RawText),
			"meal_type":      canonicalMealTypeFromParsed(parsed.MealType),
			"quantity_value": item.QuantityValue,
			"quantity_unit":  canonicalQuantityUnit(item.QuantityUnit),
			"trace_id":       "",
			"source":         "fsm:launch",
		}
		if len(item.IngredientsRaw) > 0 {
			m["ingredients_structured"] = item.IngredientsRaw
			m["ingredients_source"] = "user_provided"
		} else {
			m["ingredients_source"] = "inferred"
		}
		meals = append(meals, m)
	}
	return MutationCommand{
		Intent:    IntentLogMeal,
		ToolName:  "log_meals",
		ToolArgs:  map[string]interface{}{"meals": meals},
		RawText:   text,
		MessageID: messageID,
	}
}

func validateMutationCommand(rawText string, cmd MutationCommand) *PipelineError {
	if !isMutationToolName(cmd.ToolName) {
		return &PipelineError{Code: ErrCodePipelineStage, Message: "missing mutation tool", Recoverable: false}
	}
	if strings.TrimSpace(cmd.ToolName) == "log_meals" {
		meals, ok := cmd.ToolArgs["meals"].([]interface{})
		if !ok || len(meals) == 0 {
			return &PipelineError{Code: ErrCodeInvalidPayload, Message: "empty meals payload", Recoverable: true}
		}
		if looksLikeMultiItemInput(rawText) && len(meals) < 2 {
			return &PipelineError{Code: ErrCodePartialParse, Message: "partial parse for multi-item meal log", Recoverable: true}
		}
		for _, item := range meals {
			m, _ := item.(map[string]interface{})
			if strings.TrimSpace(asString(m["dish_name"])) == "" {
				return &PipelineError{Code: ErrCodeInvalidPayload, Message: "dish name missing", Recoverable: true}
			}
		}
	}
	return nil
}

func looksLikeMultiItemInput(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if strings.Contains(lower, " and also ") || strings.Contains(lower, "\n2.") || strings.Contains(lower, " 2.") {
		return true
	}
	re := regexp.MustCompile(`(^|\s)1\.`)
	return re.MatchString(lower) && strings.Contains(lower, "2.")
}

func (o *Orchestrator) executeMutationCommand(s *Session, cmd MutationCommand) MutationOutcome {
	args := cmd.ToolArgs
	if args == nil {
		args = map[string]interface{}{}
	}
	traceID, _ := args["trace_id"].(string)
	actionVal, _ := args["action"].(string)
	mut := MealMutationV1{
		TraceID:        traceID,
		Source:         "orchestrator:fsm",
		Action:         MutationAction(strings.ToLower(strings.TrimSpace(actionVal))),
		MealType:       asString(args["meal_type"]),
		TargetDishName: asString(args["target_dish_name"]),
	}
	if idNum, ok := args["meal_id"].(float64); ok && idNum > 0 {
		mut.MealID = uint(idNum)
	}
	if mut.Action != "" {
		mut.IdempotencyKey = mutationIdempotencyKey(cmd.MessageID, mut)
		args["idempotency_key"] = mut.IdempotencyKey
	}

	resp, err := o.Registry.Execute(s, buildToolCall(cmd.ToolName, args))
	if err != nil {
		return MutationOutcome{
			OK:           false,
			BusinessCode: ErrCodePipelineStage,
			Reply:        "Something went wrong while saving your request. Nothing was saved. Please retry.",
			ToolName:     cmd.ToolName,
			ToolResult:   resp,
		}
	}
	parsed := parseToolResponse(resp)
	if ok, errCode := validateMutatingToolAck(cmd.ToolName, parsed); !ok {
		return MutationOutcome{
			OK:           false,
			BusinessCode: errCode,
			Reply:        "Something went wrong while saving your request. Nothing was saved. Please retry.",
			ToolName:     cmd.ToolName,
			ToolResult:   resp,
			RawParsed:    parsed,
		}
	}
	if failed, errCode := extractMutationFailure(cmd.ToolName, parsed); failed {
		result := []toolExecutionResult{{ToolName: cmd.ToolName, Response: parsed}}
		reply := deterministicToolReply(result)
		if strings.TrimSpace(reply) == "" {
			reply = "I couldn't complete that request."
		}
		return MutationOutcome{
			OK:           false,
			BusinessCode: errCode,
			Reply:        reply,
			ToolName:     cmd.ToolName,
			ToolResult:   resp,
			RawParsed:    parsed,
		}
	}
	result := []toolExecutionResult{{ToolName: cmd.ToolName, Response: parsed}}
	reply := deterministicToolReply(result)
	if strings.TrimSpace(reply) == "" || reply == "Done." {
		reply = "Done."
	}
	return MutationOutcome{
		OK:           true,
		BusinessCode: "",
		Reply:        reply,
		ToolName:     cmd.ToolName,
		ToolResult:   resp,
		RawParsed:    parsed,
	}
}

func (o *Orchestrator) failPipelineTurn(s *Session, st *models.ConversationState, userText, traceID string, perr *PipelineError) {
	if perr == nil {
		perr = &PipelineError{Code: ErrCodePipelineStage, Message: "pipeline failed", Recoverable: false}
	}
	pending := st.FSMPendingState
	if pending == "" {
		pending = FSMPendingNone
	}
	_ = transitionFSMState(st, FSMStateFailed, pending, traceID, s.MessageID, map[string]interface{}{
		"error_code":  perr.Code,
		"recoverable": perr.Recoverable,
	})
	if !perr.Recoverable {
		clearMutationFailureContext(st)
		st.FSMPendingState = FSMPendingNone
	}
	reply := strings.TrimSpace(perr.Message)
	if reply == "" {
		reply = "Something went wrong while processing your request. Please retry."
	}
	o.finalizeReply(s, st, userText, reply, "pipeline_failure", "", reply)
}
