package controllers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/logger"
	"github.com/pmitra96/pateproject/models"
	"gorm.io/gorm"
)

type IngestOrderRequest struct {
	UserID          string    `json:"user_id"`
	ExternalOrderID string    `json:"external_order_id"`
	EmailMessageID  string    `json:"email_message_id"`
	Provider        string    `json:"provider"`
	OrderDate       time.Time `json:"order_date"`
	Items           []struct {
		RawName  string  `json:"raw_name"`
		Quantity float64 `json:"quantity"` // Assumed in base unit for now, or we need unit parsing
		Unit     string  `json:"unit"`
	} `json:"items"`
}

func IngestOrder(w http.ResponseWriter, r *http.Request) {

	var req IngestOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("Invalid request payload", "error", err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	logger.Info("Received ingestion request", "user_id", req.UserID, "provider", req.Provider)

	// Resolve User via Identity Table
	var identity models.UserIdentity
	var user models.User

	// Search for existing identity
	err := database.DB.Where("provider = ? AND external_id = ?", req.Provider, req.UserID).First(&identity).Error
	if err == nil {
		// Found existing identity, find the user
		if err := database.DB.First(&user, identity.UserID).Error; err != nil {
			logger.Error("Identity exists but user missing", "user_id", identity.UserID)
			http.Error(w, "Inconsistent database state", http.StatusInternalServerError)
			return
		}
	} else if err == gorm.ErrRecordNotFound {
		// No identity found. Check if user already exists by email
		email := "user-" + req.UserID + "@example.com"
		if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// No user exists, create one
				user = models.User{
					Email:    email,
					Name:     "External User (" + req.Provider + ")",
					Password: "",
				}

				tx := database.DB.Begin()
				if err := tx.Create(&user).Error; err != nil {
					tx.Rollback()
					logger.Error("Failed to auto-create user during ingestion", "error", err)
					http.Error(w, "Failed to create user", http.StatusInternalServerError)
					return
				}

				newIdentity := models.UserIdentity{
					UserID:     user.ID,
					Provider:   req.Provider,
					ExternalID: req.UserID,
				}
				if err := tx.Create(&newIdentity).Error; err != nil {
					tx.Rollback()
					logger.Error("Failed to create user identity", "error", err)
					http.Error(w, "Failed to create identity", http.StatusInternalServerError)
					return
				}
				tx.Commit()
				logger.Info("Created new user and identity for ingestion", "user_id", user.ID, "external_id", req.UserID)
			} else {
				logger.Error("Database error during user email lookup", "error", err)
				http.Error(w, "Database error", http.StatusInternalServerError)
				return
			}
		} else {
			// User exists by email, just link new identity
			newIdentity := models.UserIdentity{
				UserID:     user.ID,
				Provider:   req.Provider,
				ExternalID: req.UserID,
			}
			if err := database.DB.Create(&newIdentity).Error; err != nil {
				logger.Error("Failed to link existing user to new identity", "error", err)
				http.Error(w, "Failed to link identity", http.StatusInternalServerError)
				return
			}
			logger.Info("Linked existing user to new identity", "user_id", user.ID, "external_id", req.UserID)
		}
	} else {
		logger.Error("Database error during identity lookup", "error", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Idempotency Check
	var existingOrder models.Order
	query := database.DB.Where("provider = ?", req.Provider)
	if req.ExternalOrderID != "" {
		query = query.Where("external_order_id = ?", req.ExternalOrderID)
	} else if req.EmailMessageID != "" {
		query = query.Where("email_message_id = ?", req.EmailMessageID)
	} else {
		http.Error(w, "Either external_order_id or email_message_id is required", http.StatusBadRequest)
		return
	}

	if err := query.First(&existingOrder).Error; err == nil {
		// Already exists
		logger.Warn("Duplicate order skipped", "provider", req.Provider, "order_id", req.ExternalOrderID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "skipped", "reason": "duplicate"}`))
		return
	} else if err != gorm.ErrRecordNotFound {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Start Transaction
	tx := database.DB.Begin()

	// Create Order
	order := models.Order{
		UserID:          user.ID,
		ExternalOrderID: req.ExternalOrderID,
		EmailMessageID:  req.EmailMessageID,
		Provider:        req.Provider,
		OrderDate:       req.OrderDate,
		Status:          "processed",
	}

	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		http.Error(w, "Failed to create order: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logger.Info("Order created successfully", "order_id", order.ID, "user_id", user.ID)

	for _, reqItem := range req.Items {
		// Unit Normalization (Base Units: g, ml)
		quantity := reqItem.Quantity
		unit := strings.ToLower(reqItem.Unit)

		if unit == "kg" {
			quantity *= 1000
			unit = "g"
		} else if unit == "l" {
			quantity *= 1000
			unit = "ml"
		}

		// 1. Find or Create Canonical Item
		var item models.Item
		// transform to lower case for canonical check
		canonicalName := strings.TrimSpace(strings.ToLower(reqItem.RawName))

		// Simple canonical logic: find by name.
		if err := tx.Where("LOWER(name) = ?", canonicalName).First(&item).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Create new Item
				item = models.Item{
					Name: reqItem.RawName, // Use original casing for display
					Unit: unit,
				}
				if err := tx.Create(&item).Error; err != nil {
					tx.Rollback()
					http.Error(w, "Failed to create item: "+err.Error(), http.StatusInternalServerError)
					return
				}
			} else {
				tx.Rollback()
				http.Error(w, "Database error looking up item", http.StatusInternalServerError)
				return
			}
		}

		// 2. Create Order Item
		orderItem := models.OrderItem{
			OrderID:  order.ID,
			ItemID:   item.ID,
			RawName:  reqItem.RawName,
			Quantity: quantity,
		}
		if err := tx.Create(&orderItem).Error; err != nil {
			tx.Rollback()
			http.Error(w, "Failed to create order item", http.StatusInternalServerError)
			return
		}

		// 3. Update Pantry State
		// Find existing pantry item
		var pantryItem models.PantryItem
		if err := tx.Where("user_id = ? AND item_id = ?", user.ID, item.ID).First(&pantryItem).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				pantryItem = models.PantryItem{
					UserID:          user.ID,
					ItemID:          item.ID,
					DerivedQuantity: 0,
				}
				if err := tx.Create(&pantryItem).Error; err != nil {
					tx.Rollback()
					http.Error(w, "Failed to create pantry item", http.StatusInternalServerError)
					return
				}
			} else {
				tx.Rollback()
				http.Error(w, "Database error pantry item", http.StatusInternalServerError)
				return
			}
		}

		// Update derived quantity
		pantryItem.DerivedQuantity += quantity
		if err := tx.Save(&pantryItem).Error; err != nil {
			tx.Rollback()
			http.Error(w, "Failed to update pantry state", http.StatusInternalServerError)
			return
		}
	}

	tx.Commit()
	logger.Info("Order ingested successfully", "order_id", order.ID, "items_count", len(req.Items))

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"status": "created"}`))
}
