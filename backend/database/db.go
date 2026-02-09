package database

import (
	"fmt"
	"log"

	"github.com/pmitra96/pateproject/config"
	"github.com/pmitra96/pateproject/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	host := config.GetEnv("DB_HOST", "localhost")
	user := config.GetEnv("DB_USER", "postgres")
	password := config.GetEnv("DB_PASSWORD", "password")
	dbname := config.GetEnv("DB_NAME", "pateproject")
	port := config.GetEnv("DB_PORT", "5432")
	sslmode := config.GetEnv("DB_SSLMODE", "disable")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		host, user, password, dbname, port, sslmode)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	log.Println("Database connection established")

	// Migration logic
	log.Println("Running migrations...")
	err = DB.AutoMigrate(
		&models.User{},
		&models.UserIdentity{},
		&models.Ingredient{},
		&models.Brand{},
		&models.Item{},
		&models.Order{},
		&models.OrderItem{},
		&models.PantryItem{},
		&models.Goal{},
		&models.MealLog{},
		&models.Conversation{},
		&models.UserPreferences{},
		&models.DishSample{},
	)
	if err != nil {
		log.Fatal("Failed to migrate database: ", err)
	}
	log.Println("Migrations completed")
}
