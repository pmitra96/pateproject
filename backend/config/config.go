package config

import (
	"os"
	"strconv"
	"strings"
)

func GetEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func GetEnvInt(key string, defaultValue int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultValue
	}
	return val
}

// WhatsApp Config Defaults
func GetWhatsAppDailyLimit() int {
	return GetEnvInt("WHATSAPP_DAILY_LIMIT", 50)
}

func GetWhatsAppHistoryWindow() int {
	return GetEnvInt("WHATSAPP_HISTORY_WINDOW", 20)
}

func GetPreferredLLMModel() string {
	return GetEnv("PREFERRED_LLM_MODEL", "gpt-4o-mini")
}

func GetNutritionEstimatorV2Mode() string {
	mode := strings.ToLower(strings.TrimSpace(GetEnv("NUTRITION_ESTIMATOR_V2_MODE", "control")))
	switch mode {
	case "control", "shadow", "launch":
		return mode
	default:
		return "control"
	}
}

func GetNutritionShadowTimeoutMs() int {
	return GetEnvInt("NUTRITION_SHADOW_TIMEOUT_MS", 1500)
}

func GetIntentClassifierMode() string {
	mode := strings.ToLower(strings.TrimSpace(GetEnv("INTENT_CLASSIFIER_MODE", "control")))
	switch mode {
	case "control", "launch":
		return mode
	default:
		return "control"
	}
}
