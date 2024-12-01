package handler

import (
	"net/http"
	"pateproject/controller"
	"pateproject/entity"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ItemHandler interface {
	Create(c *gin.Context)
	GetItem(c *gin.Context)
	UpdateItem(c *gin.Context)
	DeleteItem(c *gin.Context)
}

type itemHandler struct {
	itemController controller.ItemController
}

func NewItemHandler(itemController controller.ItemController) ItemHandler {
	return &itemHandler{
		itemController: itemController,
	}
}

// CreateItemHandler handles the creation of a new item
func (h *itemHandler) Create(c *gin.Context) {
	var newItem entity.Item

	// Bind the incoming JSON to the Item struct
	if err := c.ShouldBindJSON(&newItem); err != nil {
		// If binding fails, return a 400 Bad Request response with the error
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Call the controller's method to create the item
	err := h.itemController.CreateItem(c.Request.Context(), &newItem)
	if err != nil {
		// If there was an error in the controller, return a 500 Internal Server Error response
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Return the created item with a 201 Created status code
	c.JSON(http.StatusCreated, gin.H{
		"message": "Item created successfully",
	})
}

// GetItemHandler handles fetching a specific item by ID
func (h *itemHandler) GetItem(c *gin.Context) {
	idStr := c.Param("id")         // Get the item ID from the URL parameter
	id, err := strconv.Atoi(idStr) // Convert the ID from string to int
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Call the controller's method to get the item by ID
	item, err := h.itemController.GetItem(c.Request.Context(), id)
	if err != nil {
		// If the item wasn't found or there was an error, return a 404 Not Found or 500 Internal Server Error
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	// Return the item with a 200 OK status code
	c.JSON(http.StatusOK, gin.H{
		"item": item,
	})
}

// UpdateItemHandler handles updating an existing item
func (h *itemHandler) UpdateItem(c *gin.Context) {
	idStr := c.Param("id")         // Get the item ID from the URL parameter
	id, err := strconv.Atoi(idStr) // Convert the ID from string to int
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Call the controller's method to get the item by ID
	item, err := h.itemController.GetItem(c.Request.Context(), id)
	if err != nil {
		// If the item wasn't found or there was an error, return a 404 Not Found or 500 Internal Server Error
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var updatedItem *entity.Item

	// Bind the incoming JSON to the updatedItem struct
	if err := c.ShouldBindJSON(&updatedItem); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Call the controller's method to update the item
	err = h.itemController.UpdateItem(c.Request.Context(), updatedItem)
	if err != nil {
		// If there was an error updating the item, return a 500 Internal Server Error response
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return the updated item with a 200 OK status code
	c.JSON(http.StatusOK, gin.H{
		"message": "Item updated successfully",
		"item":    item,
	})
}

// DeleteItemHandler handles deleting an item
func (h *itemHandler) DeleteItem(c *gin.Context) {
	idStr := c.Param("id")         // Get the item ID from the URL parameter
	id, err := strconv.Atoi(idStr) // Convert the ID from string to int
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Call the controller's method to delete the item
	err = h.itemController.DeleteItem(c.Request.Context(), id)
	if err != nil {
		// If there was an error deleting the item, return a 500 Internal Server Error response
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return a success message with a 200 OK status code
	c.JSON(http.StatusOK, gin.H{
		"message": "Item deleted successfully",
	})
}
