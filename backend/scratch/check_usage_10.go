package main

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/models"
)

func main() {
	godotenv.Load("../.env")
	database.InitDB()

	var identity models.UserIdentity
	database.DB.Where("external_id = ?", "916666666666").First(&identity)

	var logs []models.LLMUsageLog
	database.DB.Where("user_id = ?", identity.UserID).Order("created_at asc").Find(&logs)

	fmt.Println("LLM Usage Logs for User 10:")
	for _, l := range logs {
		fmt.Printf("- [%s] %s: %d tokens\n", l.CreatedAt.Format("15:04:05"), l.Feature, l.TotalTokens)
	}
}
