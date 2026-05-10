package whatsapp

import (
	"fmt"
	"strings"
)

type MutationAction string

const (
	MutationAdd    MutationAction = "add"
	MutationUpdate MutationAction = "update"
	MutationDelete MutationAction = "delete"
	MutationMove   MutationAction = "move"
)

type MealInputV1 struct {
	TraceID           string  `json:"trace_id"`
	Source            string  `json:"source"`
	RawText           string  `json:"raw_text"`
	MealType          string  `json:"meal_type"`
	ItemName          string  `json:"item_name"`
	QuantityValue     float64 `json:"quantity_value"`
	QuantityUnit      string  `json:"quantity_unit"`
	QuantityBaseValue float64 `json:"quantity_base_value"`
	QuantityBaseUnit  string  `json:"quantity_base_unit"`
	Confidence        float64 `json:"confidence"`
	Assumptions       string  `json:"assumptions"`
}

type MealMutationV1 struct {
	TraceID        string         `json:"trace_id"`
	Source         string         `json:"source"`
	Action         MutationAction `json:"action"`
	MealType       string         `json:"meal_type"`
	TargetDishName string         `json:"target_dish_name"`
	MealID         uint           `json:"meal_id"`
	Input          *MealInputV1   `json:"input,omitempty"`
	IdempotencyKey string         `json:"idempotency_key"`
}

type MutationResultV1 struct {
	OK        bool   `json:"ok"`
	ErrorCode string `json:"error_code,omitempty"`
	Message   string `json:"message,omitempty"`
}

func (m *MealInputV1) Validate(requireQuantity bool) error {
	if m == nil {
		return fmt.Errorf(ErrCodeInvalidPayload)
	}
	if strings.TrimSpace(m.MealType) == "" {
		return fmt.Errorf(ErrCodeInvalidPayload)
	}
	if strings.TrimSpace(m.ItemName) == "" {
		return fmt.Errorf(ErrCodeInvalidPayload)
	}
	if requireQuantity {
		if m.QuantityValue <= 0 || strings.TrimSpace(m.QuantityUnit) == "" {
			return fmt.Errorf(ErrCodeInvalidPayload)
		}
	}
	return nil
}

func (m *MealMutationV1) Validate() error {
	if m == nil {
		return fmt.Errorf(ErrCodeInvalidPayload)
	}
	switch m.Action {
	case MutationAdd, MutationUpdate:
		if m.Input == nil {
			return fmt.Errorf(ErrCodeInvalidPayload)
		}
		if err := m.Input.Validate(true); err != nil {
			return err
		}
	case MutationDelete:
		if m.MealID == 0 && strings.TrimSpace(m.TargetDishName) == "" {
			return fmt.Errorf(ErrCodeInvalidPayload)
		}
	case MutationMove:
		if m.MealID == 0 || strings.TrimSpace(m.MealType) == "" {
			return fmt.Errorf(ErrCodeInvalidPayload)
		}
	default:
		return fmt.Errorf(ErrCodeInvalidPayload)
	}
	return nil
}
