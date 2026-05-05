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

func normalizeIngredientName(name string) string {
	return strings.Title(strings.ToLower(name))
}

func main() {
	// Setup Test DB
	db, err := gorm.Open(sqlite.Open("test_pantry_final.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	database.DB = db
	db.AutoMigrate(&models.Ingredient{}, &models.PantryItem{}, &models.Item{}, &models.Brand{}, &models.User{}, &models.IngredientMapping{})

	// Create Mock User
	user := models.User{Name: "Test User", Email: "test_final@example.com"}
	db.Create(&user)

	// Image Path
	imagePath := "/Users/pushya/Documents/pushya_projects/pateproject/backend/tests/1000174823.jpg"
	imageBytes, err := os.ReadFile(imagePath)
	if err != nil {
		fmt.Printf("Error reading image: %v\n", err)
		return
	}
	imageBase64 := base64.StdEncoding.EncodeToString(imageBytes)

	client := llm.NewClient()
	
	fmt.Println("--- STEP 1: Multi-modal NIRA (One Step) ---")
	userContext := fmt.Sprintf("User ID: %d, Name: %s", user.ID, user.Name)
	// Passing imageBase64 directly to ProcessWhatsAppConversation
	assistantMsg, _, err := client.ProcessWhatsAppConversation("Analyze this order screenshot and update my pantry.", imageBase64, nil, userContext)
	if err != nil {
		fmt.Printf("NIRA Error: %v\n", err)
		return
	}

	if len(assistantMsg.ToolCalls) == 0 {
		fmt.Printf("No tools called. Response: %v\n", assistantMsg.Content)
		return
	}

	fmt.Printf("NIRA called %d tools.\n", len(assistantMsg.ToolCalls))

	for _, toolCall := range assistantMsg.ToolCalls {
		fmt.Printf("Tool: %s, Args: %s\n", toolCall.Function.Name, toolCall.Function.Arguments)
		
		if toolCall.Function.Name == "update_pantry" {
			// Executing the same logic as in controller
			var args map[string]interface{}
			json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			items := args["items"].([]interface{})

			var updatedNames []string
			type pantryChange struct {
				quantity float64
				unit     string
				action   string
			}
			ingredientChanges := make(map[uint]*pantryChange)
			ingredientNames := make(map[uint]string)
			
			// --- LOGIC FROM CONTROLLER ---
			var rawNames []string
			var results = make([]llm.PantryItemExtraction, len(items))
			var itemsToExtract []int 
			
			for i, it := range items {
				itemMap, _ := it.(map[string]interface{})
				name, _ := itemMap["name"].(string)
				rawNames = append(rawNames, name)
				
				var mapping models.IngredientMapping
				if err := database.DB.Where("LOWER(raw_name) = ?", strings.ToLower(name)).First(&mapping).Error; err == nil {
					results[i] = llm.PantryItemExtraction{Ingredient: mapping.IngredientName}
				} else {
					itemsToExtract = append(itemsToExtract, i)
				}
			}

			if len(itemsToExtract) > 0 {
				var namesToCall []string
				for _, idx := range itemsToExtract {
					namesToCall = append(namesToCall, rawNames[idx])
				}
				extractions, _ := client.ExtractPantryItemsBatch(namesToCall)
				for i, ext := range extractions {
					results[itemsToExtract[i]] = ext
				}
			}

			for i, it := range items {
				itemMap, _ := it.(map[string]interface{})
				quantity, _ := itemMap["quantity"].(float64)
				unit, _ := itemMap["unit"].(string)
				action, _ := itemMap["action"].(string)
				
				ingredientName := results[i].Ingredient
				if ingredientName == "" { ingredientName = normalizeIngredientName(rawNames[i]) }

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
				var pi models.PantryItem
				database.DB.Where("user_id = ? AND ingredient_id = ?", user.ID, ingID).FirstOrCreate(&pi, models.PantryItem{UserID: user.ID, IngredientID: ingID})

				current := pi.EffectiveQuantity()
				var newQty float64
				if change.action == "add" { newQty = current + change.quantity } else { newQty = change.quantity }

				database.DB.Model(&pi).Updates(map[string]interface{}{"manual_quantity": &newQty, "last_updated": time.Now()})
				updatedNames = append(updatedNames, fmt.Sprintf("%s (%.1f %s)", ingredientNames[ingID], newQty, change.unit))
			}
			fmt.Printf("\nFINAL RESULT: Updated %s\n", strings.Join(updatedNames, ", "))
		}
	}
}
