package main

import (
	"pateproject/db"
	"pateproject/logger"
	"pateproject/route"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	err := route.SetupRoutes(r) // Setup routes for your app
	if err != nil {
		panic(err)
	}
	logger.InitializeLogger() // Initialize the logger
	defer logger.Close()      // Close the logger when the main function exits
	defer db.Close()          // Close the database connection when the main function exits
	r.Run(":8080")            // Start the server on port 8080
}
