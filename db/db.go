package db

import (
	"fmt"
	"log"

	"pateproject/entity"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// todo: make singleton
// InitDB initializes the PostgreSQL connection
func InitDB(c *entity.Config) error {
	var err error
	// Define the connection string (PostgreSQL DSN format)
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s", c.PostgresConfig.Host, c.PostgresConfig.User, c.PostgresConfig.Password, c.PostgresConfig.DBName, c.PostgresConfig.Port, c.PostgresConfig.SSLMode)

	// Open the database connection
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}
	fmt.Println("Database connection established successfully!")
	return err
}

func Close() {
	sqlDB, err := DB.DB()
	if err != nil {
		log.Printf("Failed to retrieve sql.DB: %v", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		log.Printf("Error closing the database connection: %v", err)
	}

}

func GetDBInstance() *gorm.DB {
	return DB
}
