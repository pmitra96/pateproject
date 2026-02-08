package controllers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/jobs"
	"github.com/pmitra96/pateproject/llm"
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

	llmClient := llm.NewClient()
	nutritionWorker := jobs.GetWorker()

	// 1. Batch Cache Lookup
	rawNames := make([]string, 0, len(req.Items))
	rawNamesMap := make(map[string]bool)
	for _, it := range req.Items {
		if !rawNamesMap[it.RawName] {
			rawNames = append(rawNames, it.RawName)
			rawNamesMap[it.RawName] = true
		}
	}

	var existingItems []models.Item
	tx.Preload("Ingredient").Preload("Brand").Where("name IN ?", rawNames).Find(&existingItems)
	itemMap := make(map[string]models.Item)
	for _, it := range existingItems {
		itemMap[it.Name] = it
	}

	// 2. Identify missing items and extract in batch
	var missingNames []string
	for _, name := range rawNames {
		if _, ok := itemMap[name]; !ok {
			missingNames = append(missingNames, name)
		}
	}

	if len(missingNames) > 0 {
		logger.Info("Performing batch LLM extraction", "count", len(missingNames))
		extractions, err := llmClient.ExtractPantryItemsBatch(missingNames)
		if err != nil || len(extractions) != len(missingNames) {
			logger.Warn("Batch LLM extraction failed or returned mismatched count, using heuristics", "error", err)
			extractions = make([]llm.PantryItemExtraction, len(missingNames))
			for i, name := range missingNames {
				extractions[i] = *llmClient.ExtractHeuristic(name)
			}
		}

		// Process extractions and create missing items
		for i, name := range missingNames {
			ext := extractions[i]

			// Resolve Ingredient
			var ingredient models.Ingredient
			tx.Where("LOWER(name) = ?", strings.ToLower(ext.Ingredient)).FirstOrCreate(&ingredient, models.Ingredient{Name: ext.Ingredient})

			// Resolve Brand
			var brandID *uint
			if ext.Brand != nil && *ext.Brand != "" {
				var brand models.Brand
				tx.Where("LOWER(name) = ?", strings.ToLower(*ext.Brand)).FirstOrCreate(&brand, models.Brand{Name: *ext.Brand})
				brandID = &brand.ID
			}

			// Create Item
			productName := ""
			if ext.Product != nil {
				productName = *ext.Product
			}

			unit := "pcs"
			for _, ri := range req.Items {
				if ri.RawName == name {
					unit = strings.ToLower(ri.Unit)
					if unit == "kg" {
						unit = "g"
					} else if unit == "l" {
						unit = "ml"
					}
					break
				}
			}

			newItem := models.Item{
				Name:         name,
				IngredientID: ingredient.ID,
				BrandID:      brandID,
				ProductName:  productName,
				Unit:         unit,
			}

			// Preload ingredient for context
			newItem.Ingredient = ingredient

			tx.Create(&newItem)
			itemMap[name] = newItem
		}
	}

	// 3. Process all items (OrderItems and Pantry aggregation)
	for _, reqItem := range req.Items {
		item, ok := itemMap[reqItem.RawName]
		if !ok {
			logger.Error("Item mapping missing for raw name", "raw_name", reqItem.RawName)
			continue
		}

		// Unit Normalization (Base Units: g, ml)
		quantity := reqItem.Quantity
		unit := strings.ToLower(reqItem.Unit)
		if unit == "kg" {
			quantity *= 1000
		} else if unit == "l" {
			quantity *= 1000
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

		// 3. Update Pantry State (Aggregated by Ingredient)
		var pantryItem models.PantryItem
		if err := tx.Where("user_id = ? AND ingredient_id = ?", user.ID, item.IngredientID).First(&pantryItem).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				pantryItem = models.PantryItem{
					UserID:          user.ID,
					IngredientID:    item.IngredientID,
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

		// Update Aggregated state
		pantryItem.DerivedQuantity += quantity
		pantryItem.ItemID = item.ID // Update to the most recent specific item (brand/product)
		if err := tx.Save(&pantryItem).Error; err != nil {
			tx.Rollback()
			http.Error(w, "Failed to update pantry state", http.StatusInternalServerError)
			return
		}
	}

	tx.Commit()
	logger.Info("Order ingested successfully", "order_id", order.ID, "items_count", len(req.Items))

	// Enqueue nutrition jobs AFTER commit so items are visible to the worker
	for _, name := range missingNames {
		if item, ok := itemMap[name]; ok {
			nutritionWorker.Enqueue(item.ID)
		}
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"status": "created"}`))
}
