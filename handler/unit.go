package handler

import (
	"net/http"
	"pateproject/controller"
	"pateproject/entity"
	"strconv"

	"github.com/gin-gonic/gin"
)

type UnitHandler interface {
	Create(c *gin.Context)
	GetUnit(c *gin.Context)
	UpdateUnit(c *gin.Context)
	DeleteUnit(c *gin.Context)
}

type unitHandler struct {
	unitController controller.UnitController
}

func NewUnitHandler(unitController controller.UnitController) UnitHandler {
	return &unitHandler{
		unitController: unitController,
	}
}

// CreateUnitHandler handles the creation of a new unit
func (h *unitHandler) Create(c *gin.Context) {
	var newUnit entity.Unit

	// Bind the incoming JSON to the Unit struct
	if err := c.ShouldBindJSON(&newUnit); err != nil {
		// If binding fails, return a 400 Bad Request response with the error
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Call the controller's method to create the unit
	err := h.unitController.CreateUnit(c.Request.Context(), &newUnit)
	if err != nil {
		// If there was an error in the controller, return a 500 Internal Server Error response
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Return the created unit with a 201 Created status code
	c.JSON(http.StatusCreated, gin.H{
		"message": "Unit created successfully",
	})
}

func (h *unitHandler) GetUnit(c *gin.Context) {
	idStr := c.Param("id")         // Get the unit ID from the URL parameter
	id, err := strconv.Atoi(idStr) // Convert the ID from string to int
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Call the controller's method to get the unit by ID
	unit, err := h.unitController.GetUnit(c.Request.Context(), id)
	if err != nil {
		// If there was an error in the controller, return a 500 Internal Server Error response
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Return the unit with a 200 OK status code
	c.JSON(http.StatusOK, unit)
}

func (h *unitHandler) UpdateUnit(c *gin.Context) {
	idStr := c.Param("id")         // Get the unit ID from the URL parameter
	id, err := strconv.Atoi(idStr) // Convert the ID from string to int
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var updatedUnit entity.Unit
	if err := c.ShouldBindJSON(&updatedUnit); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updatedUnit.ID = uint(id)
	err = h.unitController.UpdateUnit(c.Request.Context(), &updatedUnit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Unit updated successfully",
	})
}

func (h *unitHandler) DeleteUnit(c *gin.Context) {
	idStr := c.Param("id")         // Get the unit ID from the URL parameter
	id, err := strconv.Atoi(idStr) // Convert the ID from string to int
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.unitController.DeleteUnit(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Unit deleted successfully",
	})
}
