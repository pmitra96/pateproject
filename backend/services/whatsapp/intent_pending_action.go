package whatsapp

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/models"
)

const pendingActionMetaKey = "pending_action_v2"

func getPendingAction(st *models.ConversationState) (*PendingAction, bool) {
	meta := getPendingSelectionMeta(st)
	raw, ok := meta[pendingActionMetaKey]
	if !ok {
		return nil, false
	}
	blob, err := json.Marshal(raw)
	if err != nil {
		return nil, false
	}
	var p PendingAction
	if err := json.Unmarshal(blob, &p); err != nil {
		return nil, false
	}
	if p.ExpiresAtUnix > 0 && time.Now().Unix() > p.ExpiresAtUnix {
		clearPendingAction(st)
		return nil, false
	}
	if p.CreatedAtUnix > 0 && p.ExpiresAtUnix > 0 && p.CreatedAtUnix >= p.ExpiresAtUnix {
		clearPendingAction(st)
		return nil, false
	}
	return &p, true
}

func setPendingAction(st *models.ConversationState, p PendingAction) {
	meta := getPendingSelectionMeta(st)
	meta[pendingActionMetaKey] = p
	persistPendingMeta(st, meta)
}

func clearPendingAction(st *models.ConversationState) {
	meta := getPendingSelectionMeta(st)
	delete(meta, pendingActionMetaKey)
	persistPendingMeta(st, meta)
}

func persistPendingMeta(st *models.ConversationState, meta map[string]any) {
	raw, _ := json.Marshal(meta)
	st.PendingSlots = string(raw)
	st.UpdatedAt = time.Now()
	updateConversationStateMeta(st)
}

func resolvePendingActionChoice(st *models.ConversationState, incomingMessageID string, text string) (llmToolName string, llmToolArgs map[string]any, handled bool, reply string) {
	p, ok := getPendingAction(st)
	if !ok {
		return "", nil, false, ""
	}
	if p.SourceMessageID != "" && incomingMessageID != "" && p.SourceMessageID == incomingMessageID {
		return "", nil, true, "I need a new reply message to continue this action."
	}
	choice := strings.ToLower(strings.TrimSpace(text))

	if p.RequiredSlot == "confirmation" {
		if strings.TrimSpace(p.ProposedToolName) == "" {
			clearPendingAction(st)
			return "", nil, true, "That pending action is no longer valid. Please retry the command."
		}
		if isPlainAffirmation(choice) {
			clearPendingAction(st)
			return p.ProposedToolName, p.ProposedToolArgs, true, ""
		}
		if isPlainNegation(choice) || choice == "cancel" {
			clearPendingAction(st)
			return "", nil, true, "Okay, cancelled."
		}
		return "", nil, true, "Reply yes/no, or cancel."
	}

	if p.RequiredSlot == "delete_scope_choice" {
		if strings.Contains(choice, "all") {
			args := map[string]any{}
			if mt, ok := p.Scope["meal_type"].(string); ok && mt != "" {
				args["meal_type"] = mt
			}
			args["delete_scope"] = "all"
			p.RequiredSlot = "confirmation"
			p.ProposedToolName = "modify_logged_meal"
			p.ProposedToolArgs = map[string]any{"action": "delete", "meal_type": args["meal_type"], "target_dish_name": "*"}
			setPendingAction(st, *p)
			return "", nil, true, "Please confirm: delete all entries for that meal type? (yes/no)"
		}
		if strings.Contains(choice, "one") || strings.Contains(choice, "item") {
			return "", nil, true, "Reply with the exact dish name to delete from that meal type."
		}
		return "", nil, true, "Reply 'all' to delete all entries, or 'one' to delete a single item."
	}

	if p.RequiredSlot == "selection" {
		if n, err := strconv.Atoi(strings.Trim(choice, ".,!?")); err == nil {
			args := p.ProposedToolArgs
			args["selection_index"] = n
			clearPendingAction(st)
			return p.ProposedToolName, args, true, ""
		}
		return "", nil, true, "Please reply with a valid number."
	}

	return "", nil, false, ""
}
