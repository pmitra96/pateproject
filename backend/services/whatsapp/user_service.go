package whatsapp

import (
	"fmt"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/models"
)

// GetOrCreateWhatsAppUser identifies an existing WhatsApp user or provisions a new one with default goals/profile.
func GetOrCreateWhatsAppUser(phone string) (*models.User, error) {
	var identity models.UserIdentity
	err := database.DB.Where("provider = ? AND external_id = ?", "whatsapp", phone).First(&identity).Error
	if err == nil {
		var existingUser models.User
		if err := database.DB.Preload("Identities").Where("id = ?", identity.UserID).First(&existingUser).Error; err == nil {
			return &existingUser, nil
		}
	}

	user := models.User{
		Name:  "WhatsApp User",
		Email: fmt.Sprintf("%s@whatsapp.local", phone),
	}
	if err := database.DB.Create(&user).Error; err != nil {
		return nil, err
	}

	identity = models.UserIdentity{UserID: user.ID, Provider: "whatsapp", ExternalID: phone}
	database.DB.Create(&identity)
	user.Identities = []models.UserIdentity{identity}

	goal := models.Goal{UserID: user.ID, Title: "WhatsApp Default Goal", IsActive: true}
	database.DB.Create(&goal)

	profile := models.GoalMacroProfile{
		GoalID:             goal.ID,
		DailyCalorieTarget: 2000,
		DailyProteinTarget: 150,
		DailyFatTarget:     65,
		DailyCarbsTarget:   200,
	}
	database.DB.Create(&profile)

	return &user, nil
}
