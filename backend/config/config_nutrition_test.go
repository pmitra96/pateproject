package config

import "testing"

func TestGetNutritionEstimatorV2ModeDefaultAndInvalid(t *testing.T) {
	t.Setenv("NUTRITION_ESTIMATOR_V2_MODE", "")
	if got := GetNutritionEstimatorV2Mode(); got != "control" {
		t.Fatalf("expected control default, got %q", got)
	}
	t.Setenv("NUTRITION_ESTIMATOR_V2_MODE", "INVALID")
	if got := GetNutritionEstimatorV2Mode(); got != "control" {
		t.Fatalf("expected control for invalid value, got %q", got)
	}
	t.Setenv("NUTRITION_ESTIMATOR_V2_MODE", "launch")
	if got := GetNutritionEstimatorV2Mode(); got != "launch" {
		t.Fatalf("expected launch, got %q", got)
	}
	t.Setenv("NUTRITION_ESTIMATOR_V2_MODE", "shadow")
	if got := GetNutritionEstimatorV2Mode(); got != "shadow" {
		t.Fatalf("expected shadow, got %q", got)
	}
}

func TestGetIntentClassifierModeDefaultAndInvalid(t *testing.T) {
	t.Setenv("INTENT_CLASSIFIER_MODE", "")
	if got := GetIntentClassifierMode(); got != "control" {
		t.Fatalf("expected control default, got %q", got)
	}
	t.Setenv("INTENT_CLASSIFIER_MODE", "INVALID")
	if got := GetIntentClassifierMode(); got != "control" {
		t.Fatalf("expected control for invalid value, got %q", got)
	}
	t.Setenv("INTENT_CLASSIFIER_MODE", "launch")
	if got := GetIntentClassifierMode(); got != "launch" {
		t.Fatalf("expected launch, got %q", got)
	}
}
