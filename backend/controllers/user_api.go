package controllers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/jobs"
	"github.com/pmitra96/pateproject/logger"
	"github.com/pmitra96/pateproject/middleware"
	"github.com/pmitra96/pateproject/models"
)

func getUserID(r *http.Request) (uint, error) {
	val := r.Context().Value(middleware.UserContextKey)
	if val == nil {
		return 0, http.ErrNoCookie
	}
	idStr, ok := val.(string)
	if !ok {
		return 0, http.ErrNoCookie
	}

	searchIDs := []string{idStr}

	// If it looks like a JWT, we need the "sub" claim
	if strings.Contains(idStr, ".") {
		parts := strings.Split(idStr, ".")
		if len(parts) >= 2 {
			payload, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err == nil {
				var claims map[string]interface{}
				if err := json.Unmarshal(payload, &claims); err == nil {
					if sub, ok := claims["sub"].(string); ok {
						searchIDs = append(searchIDs, sub)
					}
				}
			}
		}
	}

	// 1. Try UserIdentities table (exact match or extracted sub)
	var identity models.UserIdentity
	for _, sid := range searchIDs {
		if err := database.DB.Where("external_id = ?", sid).First(&identity).Error; err == nil {
			return identity.UserID, nil
		}
	}

	// 2. Try numeric ID (legacy/internal)
	if id, err := strconv.Atoi(idStr); err == nil && len(idStr) < 10 {
		return uint(id), nil
	}

	return 0, http.ErrNoCookie
}

func GetPantry(w http.ResponseWriter, r *http.Request) {
	userID, _ := getUserID(r)

	var pantryItems []models.PantryItem
	if err := database.DB.Preload("Ingredient").Preload("Item.Brand").Where("user_id = ?", userID).Find(&pantryItems).Error; err != nil {
		http.Error(w, "Failed to fetch pantry", http.StatusInternalServerError)
		return
	}

	// We can enhance this to return effective quantity explicitly if needed,
	// but the frontend can calculate it from ManualQuantity ?? DerivedQuantity.
	// Or we create a response struct.
	type PantryResponse struct {
		models.PantryItem
		EffectiveQuantity float64 `json:"effective_quantity"`
	}

	res := make([]PantryResponse, len(pantryItems))
	for i, p := range pantryItems {
		res[i] = PantryResponse{
			PantryItem:        p,
			EffectiveQuantity: p.EffectiveQuantity(),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func UpdatePantryItem(w http.ResponseWriter, r *http.Request) {
	userID, _ := getUserID(r)
	itemIDStr := chi.URLParam(r, "item_id") // Using item_id (PantryItem ID or Item ID?)
	// The path is /pantry/{item_id}. Usually implies PantryItem ID or Item ID.
	// Since pantry is per user, Item ID is better for uniqueness per user.
	// But let's assume valid ID.

	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	var req struct {
		ManualQuantity *float64 `json:"manual_quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	var pantryItem models.PantryItem
	// We first try to find by ItemID (as the frontend sends it)
	// But since ItemID can change (representative item updates), we also allow ID.
	if err := database.DB.Where("user_id = ? AND (item_id = ? OR id = ?)", userID, itemID, itemID).First(&pantryItem).Error; err != nil {
		http.Error(w, "Item not found in pantry", http.StatusNotFound)
		return
	}

	pantryItem.ManualQuantity = req.ManualQuantity
	if err := database.DB.Save(&pantryItem).Error; err != nil {
		http.Error(w, "Failed to update", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func GetItems(w http.ResponseWriter, r *http.Request) {
	var items []models.Item
	database.DB.Find(&items)
	json.NewEncoder(w).Encode(items)
}

func CreateItem(w http.ResponseWriter, r *http.Request) {
	var item models.Item
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	if err := database.DB.Create(&item).Error; err != nil {
		http.Error(w, "Failed to create item", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

func GetOrders(w http.ResponseWriter, r *http.Request) {
	userID, _ := getUserID(r)
	var orders []models.Order
	database.DB.Preload("OrderItems.Item.Ingredient").Preload("OrderItems.Item.Brand").Where("user_id = ?", userID).Find(&orders)
	json.NewEncoder(w).Encode(orders)
}

func GetLowStock(w http.ResponseWriter, r *http.Request) {
	userID, _ := getUserID(r)

	// Complex logic typically, doing simple iteration for now or SQL query.
	// "Reorder suggestions are computed dynamically"
	// Let's say low stock is < 2 units.

	var pantryItems []models.PantryItem
	database.DB.Preload("Item").Where("user_id = ?", userID).Find(&pantryItems)

	var lowStock []models.PantryItem
	for _, p := range pantryItems {
		if p.EffectiveQuantity() < 2.0 {
			lowStock = append(lowStock, p)
		}
	}

	json.NewEncoder(w).Encode(lowStock)
}

func DeletePantryItem(w http.ResponseWriter, r *http.Request) {
	userID, _ := getUserID(r)
	itemIDStr := chi.URLParam(r, "item_id")

	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	logger.Info("Deleting pantry item", "user_id", userID, "item_id", itemID)

	var pantryItem models.PantryItem
	if err := database.DB.Where("user_id = ? AND item_id = ?", userID, itemID).First(&pantryItem).Error; err != nil {
		http.Error(w, "Item not found in pantry", http.StatusNotFound)
		return
	}

	if err := database.DB.Delete(&pantryItem).Error; err != nil {
		http.Error(w, "Failed to delete item", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func BulkDeletePantryItems(w http.ResponseWriter, r *http.Request) {
	userID, _ := getUserID(r)

	var req struct {
		ItemIDs []uint `json:"item_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	if len(req.ItemIDs) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	logger.Info("Bulk deleting pantry items", "user_id", userID, "count", len(req.ItemIDs))

	if err := database.DB.Where("user_id = ? AND item_id IN ?", userID, req.ItemIDs).Delete(&models.PantryItem{}).Error; err != nil {
		http.Error(w, "Failed to delete items", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func AddPantryItem(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Name     string  `json:"name"`
		Quantity float64 `json:"quantity"`
		Unit     string  `json:"unit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Quantity <= 0 {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	// 1. Find or Create Ingredient
	var ingredient models.Ingredient
	if err := database.DB.Where("name = ?", req.Name).First(&ingredient).Error; err != nil {
		ingredient = models.Ingredient{Name: req.Name}
		database.DB.Create(&ingredient)
	}

	// 2. Find or Create Item (simple default item for manual entry)
	var item models.Item
	if err := database.DB.Where("name = ?", req.Name).First(&item).Error; err != nil {
		// Create new item if not exists
		item = models.Item{
			Name:         req.Name,
			IngredientID: ingredient.ID,
			Unit:         req.Unit,
		}
		if err := database.DB.Create(&item).Error; err != nil {
			http.Error(w, "Failed to create item", http.StatusInternalServerError)
			return
		}
	}

	// 3. Update or Create PantryItem
	var pantryItem models.PantryItem
	if err := database.DB.Where("user_id = ? AND ingredient_id = ?", userID, ingredient.ID).First(&pantryItem).Error; err == nil {
		// Update existing
		newQty := req.Quantity
		if pantryItem.ManualQuantity != nil {
			newQty += *pantryItem.ManualQuantity
		} else {
			newQty += pantryItem.DerivedQuantity
		}
		pantryItem.ManualQuantity = &newQty
		database.DB.Save(&pantryItem)
	} else {
		// Create new
		qty := req.Quantity
		pantryItem = models.PantryItem{
			UserID:         userID,
			IngredientID:   ingredient.ID,
			ItemID:         item.ID,
			ManualQuantity: &qty,
		}
		database.DB.Create(&pantryItem)
	}

	// Trigger nutrition worker for the item
	go func() {
		// Small delay to ensure DB commit if transaction was used (here it's auto-commit but good practice)
		// and mostly to not block response
		jobs.GetWorker().Enqueue(item.ID)
	}()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(pantryItem)
}
