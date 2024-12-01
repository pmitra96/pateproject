package handler

import (
	"net/http"
	"pateproject/entity"
	"pateproject/service"

	"github.com/gin-gonic/gin"
)

// AuthHandler interface
type AuthHandler interface {
	Login(c *gin.Context)
}

// authHandler struct
type authHandler struct {
	authService service.AuthService
}

// NewAuthHandler creates and returns a new AuthHandler
func NewAuthHandler(authService service.AuthService) AuthHandler {
	return &authHandler{
		authService: authService,
	}
}

// Login handles user authentication
func (h *authHandler) Login(c *gin.Context) {
	// Bind the incoming JSON to the loginRequest struct
	var loginRequest entity.LoginRequest
	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Call the AuthService's Login method
	user, token, err := h.authService.Login(c.Request.Context(), loginRequest.Email, loginRequest.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Return the user and token with a 200 OK status code
	c.JSON(http.StatusOK, gin.H{
		"user":  user,
		"token": token,
	})
}
