package controllers

import (
	"my-gin-app/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GenerateRecipe(c *gin.Context) {
	recipe := services.GenerateRecipe("spaghetti", "vegetarian")
	c.JSON(http.StatusOK, recipe)
}
