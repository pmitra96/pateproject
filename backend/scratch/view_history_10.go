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

	var conv models.Conversation
	database.DB.Where("user_id = ?", 10).Order("updated_at desc").First(&conv)

	fmt.Println("Conversation History for Test User 10:")
	fmt.Println(conv.Messages)
}
