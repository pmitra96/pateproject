package whatsapp

import "time"

type IntentName string

const (
	IntentLogMeal       IntentName = "log_meal"
	IntentModifyMeal    IntentName = "modify_meal"
	IntentDeleteMeal    IntentName = "delete_meal"
	IntentGetSummary    IntentName = "get_summary"
	IntentGetBudget     IntentName = "get_budget"
	IntentSetGoal       IntentName = "set_goal"
	IntentUpdateProfile IntentName = "update_profile"
	IntentUpdatePantry  IntentName = "update_pantry"
	IntentAdvice        IntentName = "advice"
	IntentHelp          IntentName = "help"
	IntentFallback      IntentName = "fallback"
)

type IntentResult struct {
	Intent       IntentName     `json:"intent"`
	Confidence   float64        `json:"confidence"`
	Entities     map[string]any `json:"entities"`
	MissingSlots []string       `json:"missing_slots"`
	TraceID      string         `json:"trace_id"`
	Reason       string         `json:"reason"`
}

type Decision string

const (
	DecisionExecute Decision = "execute"
	DecisionClarify Decision = "clarify"
	DecisionConfirm Decision = "confirm"
	DecisionRefuse  Decision = "refuse"
)

type PolicyDecision struct {
	Decision Decision
	Reason   string
}

type PendingAction struct {
	Type             string         `json:"type"`
	RequiredSlot     string         `json:"required_slot"`
	Scope            map[string]any `json:"scope"`
	ProposedToolName string         `json:"proposed_tool_name"`
	ProposedToolArgs map[string]any `json:"proposed_tool_args"`
	TraceID          string         `json:"trace_id"`
	ExpiresAtUnix    int64          `json:"expires_at_unix"`
	CreatedAtUnix    int64          `json:"created_at_unix"`
	SourceMessageID  string         `json:"source_message_id"`
}

func newPendingAction(actionType, requiredSlot string, scope map[string]any, toolName string, toolArgs map[string]any, traceID, sourceMessageID string) PendingAction {
	now := time.Now()
	return PendingAction{
		Type:             actionType,
		RequiredSlot:     requiredSlot,
		Scope:            scope,
		ProposedToolName: toolName,
		ProposedToolArgs: toolArgs,
		TraceID:          traceID,
		CreatedAtUnix:    now.Unix(),
		SourceMessageID:  sourceMessageID,
		ExpiresAtUnix:    now.Add(15 * time.Minute).Unix(),
	}
}
