package whatsapp

func decideIntentPolicy(result IntentResult) PolicyDecision {
	if len(result.MissingSlots) > 0 {
		if result.Intent == IntentDeleteMeal {
			return PolicyDecision{Decision: DecisionConfirm, Reason: "destructive_ambiguous"}
		}
		return PolicyDecision{Decision: DecisionClarify, Reason: "missing_slots"}
	}

	switch result.Intent {
	case IntentDeleteMeal:
		if result.Confidence < 0.85 {
			return PolicyDecision{Decision: DecisionClarify, Reason: "low_confidence_destructive"}
		}
		return PolicyDecision{Decision: DecisionConfirm, Reason: "destructive_requires_confirmation"}
	case IntentLogMeal, IntentModifyMeal, IntentSetGoal, IntentUpdatePantry, IntentUpdateProfile:
		if result.Confidence < 0.75 {
			return PolicyDecision{Decision: DecisionClarify, Reason: "low_confidence_write"}
		}
		return PolicyDecision{Decision: DecisionExecute}
	case IntentGetSummary, IntentGetBudget, IntentAdvice, IntentHelp:
		if result.Confidence < 0.50 {
			return PolicyDecision{Decision: DecisionClarify, Reason: "low_confidence_read"}
		}
		return PolicyDecision{Decision: DecisionExecute}
	default:
		return PolicyDecision{Decision: DecisionClarify, Reason: "fallback"}
	}
}
