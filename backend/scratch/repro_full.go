package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/llm"
	"github.com/pmitra96/pateproject/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Mocked helper from controllers
func normalizeIngredientName(name string) string {
	return strings.Title(strings.ToLower(name))
}

func main() {
	// Setup Test DB
	db, err := gorm.Open(sqlite.Open("test_pantry.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	database.DB = db
	db.AutoMigrate(&models.Ingredient{}, &models.PantryItem{}, &models.Item{}, &models.Brand{}, &models.User{})

	// Create Mock User
	user := models.User{Name: "Test User", Email: "test@example.com"}
	db.Create(&user)

	// Image Path
	imagePath := "/Users/pushya/Documents/pushya_projects/pateproject/backend/tests/1000174823.jpg"
	imageBytes, err := os.ReadFile(imagePath)
	if err != nil {
		fmt.Printf("Error reading image: %v\n", err)
		return
	}

	client := llm.NewClient()
	
	fmt.Println("--- STEP 1: Vision Analysis ---")
	analysis, err := client.AnalyzeMealImage(base64.StdEncoding.EncodeToString(imageBytes))
	if err != nil {
		fmt.Printf("Vision Error: %v\n", err)
		return
	}
	fmt.Printf("Vision Output:\n%s\n\n", analysis)

	fmt.Println("--- STEP 2: NIRA Processing ---")
	userContext := fmt.Sprintf("User ID: %d, Name: %s", user.ID, user.Name)
	assistantMsg, _, err := client.ProcessWhatsAppConversation(analysis, nil, userContext)
	if err != nil {
		fmt.Printf("NIRA Error: %v\n", err)
		return
	}

	if len(assistantMsg.ToolCalls) == 0 {
		fmt.Println("No tools called by NIRA.")
		return
	}

	fmt.Println("--- STEP 3: Executing Pantry Update Logic ---")
	for _, toolCall := range assistantMsg.ToolCalls {
		if toolCall.Function.Name == "update_pantry" {
			var args map[string]interface{}
			json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			items := args["items"].([]interface{})

			var updatedNames []string
			
			// --- COPIED FROM whatsapp_controller.go ---
			type pantryChange struct {
				quantity float64
				unit     string
				action   string
			}
			ingredientChanges := make(map[uint]*pantryChange)
			ingredientNames := make(map[uint]string)
			
			for _, it := range items {
				itemMap := it.(map[string]interface{})
				name, _ := itemMap["name"].(string)
				quantity, _ := itemMap["quantity"].(float64)
				unit, _ := itemMap["unit"].(string)
				action, _ := itemMap["action"].(string)

				fmt.Printf("[DEBUG] Aggregating item: %s\n", name)
				
				extraction, err := client.ExtractPantryItemInfo(name)
				var ingredientName string
				if err == nil && extraction.Ingredient != "" {
					ingredientName = extraction.Ingredient
				} else {
					ingredientName = normalizeIngredientName(name)
				}

				var ingredient models.Ingredient
				database.DB.Where("LOWER(name) = ?", strings.ToLower(ingredientName)).FirstOrCreate(&ingredient, models.Ingredient{Name: ingredientName})
				
				if _, exists := ingredientChanges[ingredient.ID]; !exists {
					ingredientChanges[ingredient.ID] = &pantryChange{quantity: quantity, unit: unit, action: action}
					ingredientNames[ingredient.ID] = ingredientName
				} else {
					if action == "add" && ingredientChanges[ingredient.ID].action == "add" {
						ingredientChanges[ingredient.ID].quantity += quantity
					} else {
						ingredientChanges[ingredient.ID].quantity = quantity
						ingredientChanges[ingredient.ID].action = action
					}
				}
			}

			for ingID, change := range ingredientChanges {
				fmt.Printf("[DEBUG] Updating DB for Ingredient ID %d (%s)\n", ingID, ingredientNames[ingID])
				var pi models.PantryItem
				err := database.DB.Where("user_id = ? AND ingredient_id = ?", user.ID, ingID).First(&pi).Error
				
				if err != nil {
					if err == gorm.ErrRecordNotFound {
						var sampleItem models.Item
						database.DB.Where("ingredient_id = ?", ingID).First(&sampleItem)
						
						pi = models.PantryItem{
							UserID:       user.ID,
							IngredientID: ingID,
							ItemID:       sampleItem.ID,
						}
						database.DB.Create(&pi)
					} else {
						continue
					}
				}

				current := pi.EffectiveQuantity()
				var newQty float64
				switch change.action {
				case "add":
					newQty = current + change.quantity
				case "set":
					newQty = change.quantity
				case "remove":
					newQty = 0
				}

				database.DB.Model(&pi).Updates(map[string]interface{}{
					"manual_quantity": &newQty,
					"last_updated":    time.Now(),
				})
				updatedNames = append(updatedNames, fmt.Sprintf("%s (%.1f %s)", ingredientNames[ingID], newQty, change.unit))
			}
			// --- END COPIED ---

			fmt.Printf("\nFINAL RESPONSE:\nUpdated: %s\n", strings.Join(updatedNames, ", "))
		}
	}
}
