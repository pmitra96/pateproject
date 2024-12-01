package handler

import (
	"net/http"
	"pateproject/controller"
	"pateproject/entity"
	"strconv"

	"github.com/gin-gonic/gin"
)

type UserHandler interface {
	Create(c *gin.Context)
	GetUser(c *gin.Context)
	UpdateUser(c *gin.Context)
	DeleteUser(c *gin.Context)
}

type userHandler struct {
	userController controller.UserController
}

func NewUserHandler(userController controller.UserController) UserHandler {
	return &userHandler{
		userController: userController,
	}
}

// CreateUserHandler handles the creation of a new user
func (h *userHandler) Create(c *gin.Context) {
	var newUser entity.User

	// Bind the incoming JSON to the User struct
	if err := c.ShouldBindJSON(&newUser); err != nil {
		// If binding fails, return a 400 Bad Request response with the error
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Call the controller's method to create the user
	err := h.userController.CreateUser(c.Request.Context(), &newUser)
	if err != nil {
		// If there was an error in the controller, return a 500 Internal Server Error response
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Return the created user with a 201 Created status code
	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
	})
}

// GetUserHandler handles fetching a specific user by ID
func (h *userHandler) GetUser(c *gin.Context) {
	idStr := c.Param("id")         // Get the user ID from the URL parameter
	id, err := strconv.Atoi(idStr) // Convert the ID from string to int
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Call the controller's method to get the user by ID
	user, err := h.userController.GetUser(c.Request.Context(), id)
	if err != nil {
		// If the user wasn't found or there was an error, return a 404 Not Found or 500 Internal Server Error
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	// Return the user with a 200 OK status code
	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

// UpdateUserHandler handles updating an existing user
func (h *userHandler) UpdateUser(c *gin.Context) {
	idStr := c.Param("id")         // Get the user ID from the URL parameter
	id, err := strconv.Atoi(idStr) // Convert the ID from string to int
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Call the controller's method to get the user by ID
	user, err := h.userController.GetUser(c.Request.Context(), id)
	if err != nil {
		// If the user wasn't found or there was an error, return a 404 Not Found or 500 Internal Server Error
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var updatedUser *entity.User

	// Bind the incoming JSON to the updatedUser struct
	if err := c.ShouldBindJSON(&updatedUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Call the controller's method to update the user
	err = h.userController.UpdateUser(c.Request.Context(), updatedUser)
	if err != nil {
		// If there was an error updating the user, return a 500 Internal Server Error response
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return the updated user with a 200 OK status code
	c.JSON(http.StatusOK, gin.H{
		"message": "User updated successfully",
		"user":    user,
	})
}

// DeleteUserHandler handles deleting a user
func (h *userHandler) DeleteUser(c *gin.Context) {
	idStr := c.Param("id")         // Get the user ID from the URL parameter
	id, err := strconv.Atoi(idStr) // Convert the ID from string to int
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Call the controller's method to delete the user
	err = h.userController.DeleteUser(c.Request.Context(), id)
	if err != nil {
		// If there was an error deleting the user, return a 500 Internal Server Error response
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return a success message with a 200 OK status code
	c.JSON(http.StatusOK, gin.H{
		"message": "User deleted successfully",
	})
}
