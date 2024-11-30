package routes

import (
	"my-gin-app/controllers"
	"my-gin-app/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	r.Use(middleware.CORS()) // Add CORS middleware globally

	// Define routes
	r.GET("/recipes", controllers.GenerateRecipe)
}
