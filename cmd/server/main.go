package main

import (
	"my-gin-app/routes"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	routes.SetupRoutes(r) // Setup routes for your app
	r.Run(":8080")        // Start the server on port 8080
}
