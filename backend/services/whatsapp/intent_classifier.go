package whatsapp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pmitra96/pateproject/llm"
)

type IntentClassifier interface {
	Classify(text string, userContext string, traceID string) (IntentResult, error)
}

type llmIntentClassifier struct {
	client *llm.Client
}

func newIntentClassifier(client *llm.Client) IntentClassifier {
	return &llmIntentClassifier{client: client}
}

func (c *llmIntentClassifier) Classify(text string, userContext string, traceID string) (IntentResult, error) {
	prompt := fmt.Sprintf(`Classify this WhatsApp nutrition message into one intent.
Return ONLY JSON with keys: intent, confidence, entities, missing_slots, reason.

Allowed intents: log_meal, modify_meal, delete_meal, get_summary, get_budget, set_goal, update_profile, update_pantry, advice, help, fallback.
Rules:
- confidence must be 0 to 1.
- Use fallback when unsure.
- For destructive requests with ambiguity (like "delete breakfast"), set intent=delete_meal and include missing_slots with delete_scope_choice.
- Keep entities minimal and factual.

User context: %s
Message: %s`, userContext, text)

	out, _, err := c.client.Chat([]llm.Message{{Role: "system", Content: "You are a strict JSON classifier. Output JSON only."}, {Role: "user", Content: prompt}})
	if err != nil {
		return IntentResult{}, err
	}

	parsed := strings.TrimSpace(out)
	parsed = strings.TrimPrefix(parsed, "```json")
	parsed = strings.TrimPrefix(parsed, "```")
	parsed = strings.TrimSuffix(parsed, "```")
	parsed = strings.TrimSpace(parsed)

	var r IntentResult
	if err := json.Unmarshal([]byte(parsed), &r); err != nil {
		return IntentResult{}, fmt.Errorf("intent classifier json parse: %w", err)
	}
	if r.Entities == nil {
		r.Entities = map[string]any{}
	}
	if r.Confidence < 0 {
		r.Confidence = 0
	}
	if r.Confidence > 1 {
		r.Confidence = 1
	}
	if r.Intent == "" {
		r.Intent = IntentFallback
	}
	r.TraceID = traceID
	return r, nil
}
