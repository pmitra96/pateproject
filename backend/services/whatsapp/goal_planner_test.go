package whatsapp

import (
	"testing"

	"github.com/pmitra96/pateproject/models"
	"github.com/stretchr/testify/assert"
)

func TestParseWeightGoal_LoseAndGain(t *testing.T) {
	loss := parseWeightLossGoal("I want to lose 2 kg in 1 month")
	if assert.NotNil(t, loss) {
		assert.Equal(t, "lose", loss.Mode)
		assert.Equal(t, 2.0, loss.Kg)
		assert.Equal(t, 30, loss.Days)
	}

	gain := parseWeightLossGoal("I want to gain 3kg in 6 weeks")
	if assert.NotNil(t, gain) {
		assert.Equal(t, "gain", gain.Mode)
		assert.Equal(t, 3.0, gain.Kg)
		assert.Equal(t, 42, gain.Days)
	}
}

func TestCalculateGoalFromProfile_GainHigherThanLoss(t *testing.T) {
	p := models.UserPreferences{
		Height:        175,
		Weight:        70,
		Age:           30,
		Gender:        "male",
		ActivityLevel: "moderate",
	}
	lossTarget, _, okLoss := calculateGoalFromProfile(p, weightLossGoal{Kg: 2, Days: 30, Mode: "lose"})
	gainTarget, _, okGain := calculateGoalFromProfile(p, weightLossGoal{Kg: 2, Days: 30, Mode: "gain"})
	assert.True(t, okLoss)
	assert.True(t, okGain)
	assert.Greater(t, gainTarget, lossTarget)
}
