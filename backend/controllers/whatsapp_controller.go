package controllers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pmitra96/pateproject/config"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/llm"
	"github.com/pmitra96/pateproject/models"
	"github.com/pmitra96/pateproject/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// VerifyWhatsAppWebhook handles the GET request from Meta to verify the webhook URL.
func VerifyWhatsAppWebhook(w http.ResponseWriter, r *http.Request) {
	verifyToken := config.GetEnv("WHATSAPP_VERIFY_TOKEN", "")
	
	// Parse query parameters
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode != "" && token != "" {
		if mode == "subscribe" && token == verifyToken {
			fmt.Println("WEBHOOK_VERIFIED")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(challenge))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusBadRequest)
}

// HandleWhatsAppMessage handles the POST request from Meta containing new messages.
func HandleWhatsAppMessage(w http.ResponseWriter, r *http.Request) {
	// 1. Decode the JSON payload
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Printf("Error decoding WhatsApp payload: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Print for debugging
	bodyBytes, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Printf("Received WhatsApp Webhook:\n%s\n", string(bodyBytes))

	// Return a 200 OK to Meta quickly so they don't retry
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("EVENT_RECEIVED"))

	// Process message in a goroutine
	go processWhatsAppPayload(payload)
}

func processWhatsAppPayload(payload map[string]interface{}) {
	// Parse the Meta Webhook payload format
	// {"object":"whatsapp_business_account","entry":[{"id":"...","changes":[{"value":{"messages":[{"from":"number","text":{"body":"..."}}]}}]}]}
	
	entries, ok := payload["entry"].([]interface{})
	if !ok || len(entries) == 0 {
		return
	}

	entry, ok := entries[0].(map[string]interface{})
	if !ok {
		return
	}

	changes, ok := entry["changes"].([]interface{})
	if !ok || len(changes) == 0 {
		return
	}

	change, ok := changes[0].(map[string]interface{})
	if !ok {
		return
	}

	value, ok := change["value"].(map[string]interface{})
	if !ok {
		return
	}

	messages, ok := value["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		// Might be a status update, ignore for now
		return
	}

	message, ok := messages[0].(map[string]interface{})
	if !ok {
		return
	}

	fromPhone, _ := message["from"].(string)
	
	var textBody string
	msgType, _ := message["type"].(string)

	if msgType == "text" {
		textObj, _ := message["text"].(map[string]interface{})
		textBody, _ = textObj["body"].(string)
	} else if msgType == "interactive" {
		interactiveObj, _ := message["interactive"].(map[string]interface{})
		if interactiveObj["type"] == "button_reply" {
			buttonReply, _ := interactiveObj["button_reply"].(map[string]interface{})
			textBody, _ = buttonReply["id"].(string) // Use the ID as the text for the intent handler
			fmt.Printf("Received button reply: %s\n", textBody)
		}
	} else if msgType == "image" {
		imageObj, _ := message["image"].(map[string]interface{})
		mediaID, _ := imageObj["id"].(string)
		caption, _ := imageObj["caption"].(string)
		
		fmt.Printf("Received image from %s with media_id: %s\n", fromPhone, mediaID)
		
		// Download 
		imageBytes, err := downloadWhatsAppMedia(mediaID)
		if err != nil {
			fmt.Printf("Error downloading media: %v\n", err)
			SendWhatsAppMessage(fromPhone, "📸 I saw your photo, but I had trouble downloading it. Can you try again?")
			return
		}
		
		SendWhatsAppMessage(fromPhone, "👀 Analyzing your photo... give me a moment.")
		textBody = caption
		imageBase64 := base64.StdEncoding.EncodeToString(imageBytes)
		
		// 1. Auto-provision User
		user, err := GetOrCreateWhatsAppUser(fromPhone)
		if err != nil {
			fmt.Printf("Error provisioning user: %v\n", err)
			SendWhatsAppMessage(fromPhone, "Sorry, there was an internal error setting up your account.")
			return
		}

		// 2. Classify intent and act (Multi-modal)
		responseMsg := handleWhatsAppIntent(user, textBody, imageBase64)

		// 3. Send Reply
		SendWhatsAppMessage(fromPhone, responseMsg)
		return
	}

	if fromPhone == "" || textBody == "" {
		return
	}

	fmt.Printf("WhatsApp Message from %s (Type: %s): %s\n", fromPhone, msgType, textBody)
	
	// 1. Auto-provision User
	user, err := GetOrCreateWhatsAppUser(fromPhone)
	if err != nil {
		fmt.Printf("Error provisioning user: %v\n", err)
		SendWhatsAppMessage(fromPhone, "Sorry, there was an internal error setting up your account.")
		return
	}

	// 2. Classify intent and act
	responseMsg := handleWhatsAppIntent(user, textBody, "")

	// 3. Send Reply
	SendWhatsAppMessage(fromPhone, responseMsg)
}

func downloadWhatsAppMedia(mediaID string) ([]byte, error) {
	accessToken := config.GetEnv("WHATSAPP_ACCESS_TOKEN", "")
	if accessToken == "" {
		return nil, fmt.Errorf("WHATSAPP_ACCESS_TOKEN not configured")
	}
	
	// 1. Get media URL
	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s", mediaID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get media metadata (status %d): %s", resp.StatusCode, string(body))
	}
	
	var meta struct {
		URL string `json:"url"`
	}
	json.NewDecoder(resp.Body).Decode(&meta)
	
	// 2. Download the actual media
	req, _ = http.NewRequest("GET", meta.URL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to download media bits (status %d)", resp.StatusCode)
	}
	
	return io.ReadAll(resp.Body)
}

// GetOrCreateWhatsAppUser finds a user by phone or creates a new one
func GetOrCreateWhatsAppUser(phone string) (*models.User, error) {
	var identity models.UserIdentity
	err := database.DB.Where("provider = ? AND external_id = ?", "whatsapp", phone).First(&identity).Error
	if err == nil {
		var existingUser models.User
		if err := database.DB.Where("id = ?", identity.UserID).First(&existingUser).Error; err == nil {
			return &existingUser, nil
		}
	}

	// Create user
	user := models.User{
		Name:  "WhatsApp User",
		Email: fmt.Sprintf("%s@whatsapp.local", phone), // Dummy email since unique is required
	}
	if err := database.DB.Create(&user).Error; err != nil {
		return nil, err
	}

	// Create identity
	identity = models.UserIdentity{
		UserID:     user.ID,
		Provider:   "whatsapp",
		ExternalID: phone,
	}
	if err := database.DB.Create(&identity).Error; err != nil {
		return nil, err
	}

	// Assign default goal (2000 cal, 150g protein) for new WhatsApp users
	goal := models.Goal{
		UserID:      user.ID,
		Title:       "WhatsApp Default Goal",
		Description: "Auto-provisioned via WhatsApp",
		IsActive:    true,
	}
	database.DB.Create(&goal)

	profile := models.GoalMacroProfile{
		GoalID:                     goal.ID,
		DailyCalorieTarget:         2000,
		DailyProteinTarget:         150,
		DailyFatTarget:             65,
		DailyCarbsTarget:           200,
		MacroPriorityOrder:         "[\"protein\", \"calories\"]",
		DamageControlFloorCalories: 300,
	}
	database.DB.Create(&profile)

	return &user, nil
}

func handleWhatsAppIntent(user *models.User, text string, imageBase64 string) string {
	// 0. Rate Limit Check (50 messages per day)
	var count int64
	todayStart := time.Now().Truncate(24 * time.Hour)
	database.DB.Model(&models.LLMUsageLog{}).Where("user_id = ? AND created_at >= ?", user.ID, todayStart).Count(&count)
	if count >= 50 {
		return "You've reached your daily limit of 50 AI messages. Please try again tomorrow! 🌙"
	}

	llmClient := llm.NewClient()

	// 0. Gather Context for Coaching (Current + Recent History)
	now := time.Now()
	state, _ := ComputeRemainingDayState(user.ID, now)
	userContext := "No active goal set."
	if state != nil {
		userContext = fmt.Sprintf("TODAY: Goal %.0f kcal. Consumed %.0f/%.0f kcal. Remaining: %.0f kcal, %.1fg protein, %.1fg carbs, %.1fg fat. Mode: %s.",
			state.TargetCalories, (state.TargetCalories - state.RemainingCalories), state.TargetCalories, state.RemainingCalories, state.RemainingProtein, state.RemainingCarbs, state.RemainingFat, state.ControlMode)
	}

	// Add summaries for the last 3 days
	recentHistory := "\nRECENT HISTORY:"
	hasHistory := false
	for i := 1; i <= 3; i++ {
		pastDate := now.AddDate(0, 0, -i)
		pState, _ := ComputeRemainingDayState(user.ID, pastDate)
		if pState != nil && (pState.TargetCalories-pState.RemainingCalories) > 0 {
			recentHistory += fmt.Sprintf("\n- %s: Consumed %.0f/%.0f kcal.", pastDate.Format("Mon Jan 02"), pState.TargetCalories-pState.RemainingCalories, pState.TargetCalories)
			hasHistory = true
		}
	}
	if hasHistory {
		userContext += recentHistory
	}

	// 1. Process via LLM Tool Calling within a transaction to avoid history races
	var assistantMsg *llm.Message
	var usage llm.Usage
	var err error
	
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var conv models.Conversation
		// Use Clause FOR UPDATE to lock the conversation row for this user
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", user.ID).First(&conv).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		var history []llm.Message
		if conv.Messages != "" {
			json.Unmarshal([]byte(conv.Messages), &history)
		}

		// Process via LLM
		assistantMsg, usage, err = llmClient.ProcessWhatsAppConversation(text, imageBase64, history, userContext)
		if err != nil {
			return err
		}

		// Update History
		updatedHistory := append(history, llm.Message{Role: "user", Content: text})
		if len(assistantMsg.ToolCalls) == 0 && assistantMsg.Content != "" {
			updatedHistory = append(updatedHistory, llm.Message{Role: "assistant", Content: assistantMsg.Content})
		} else if len(assistantMsg.ToolCalls) > 0 {
			updatedHistory = append(updatedHistory, llm.Message{Role: "assistant", Content: fmt.Sprintf("[Action: %s]", assistantMsg.ToolCalls[0].Function.Name)})
		}

		if len(updatedHistory) > 20 {
			updatedHistory = updatedHistory[len(updatedHistory)-20:]
		}
		historyJSON, _ := json.Marshal(updatedHistory)
		
		if conv.ID == 0 {
			return tx.Create(&models.Conversation{UserID: user.ID, Messages: string(historyJSON)}).Error
		} else {
			return tx.Model(&conv).Update("messages", string(historyJSON)).Error
		}
	})

	if err != nil {
		fmt.Println("Processing Error:", err)
		return "I'm having trouble thinking right now. Please try again later."
	}

	// 1.1 Log Usage (Outside transaction)
	database.DB.Create(&models.LLMUsageLog{
		UserID:           user.ID,
		Model:            "gpt-4o-mini",
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		Feature:          "whatsapp",
		CreatedAt:        time.Now(),
	})

	// 2. If it's a conversational reply (no tools called), return the text
	if len(assistantMsg.ToolCalls) == 0 {
		content, _ := assistantMsg.Content.(string)
		return content
	}

	// 3. Handle Tool Calls
	var toolResponses []string
	for _, toolCall := range assistantMsg.ToolCalls {
		toolResponses = append(toolResponses, handleWhatsAppToolCall(user, toolCall))
	}
	
	if len(toolResponses) > 0 {
		return strings.Join(toolResponses, "\n\n")
	}
	return "I'm not sure how to respond to that."
}

func handleWhatsAppToolCall(user *models.User, toolCall llm.ToolCall) string {
	ns := services.NewNutritionService()
	llmClient := llm.NewClient()
	
	switch toolCall.Function.Name {
	case "log_meals":
		var args map[string]interface{}
		json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		meals, ok := args["meals"].([]interface{})
		if !ok || len(meals) == 0 {
			return "I couldn't identify any specific meals in your message. Could you please specify what you ate?"
		}

		var summary []string
		for _, m := range meals {
			mealData, _ := m.(map[string]interface{})
			dishName, _ := mealData["dish_name"].(string)
			ingredients, _ := mealData["ingredients"].(string)
			mealType, _ := mealData["meal_type"].(string)

			estimated, err := ns.EstimateNutritionFromQuery(user.ID, ingredients)
			if err != nil || estimated == nil {
				continue
			}

			preState, _ := ComputeRemainingDayState(user.ID, time.Now())
			controlMode := "NORMAL"
			if preState != nil {
				controlMode = preState.ControlMode
			}

			mealLog := models.MealLog{
				UserID:           user.ID,
				Name:             dishName,
				MealType:         mealType,
				Ingredients:      ingredients,
				Calories:         estimated.Calories,
				Protein:          estimated.Protein,
				Carbs:            estimated.Carbs,
				Fat:              estimated.Fat,
				LoggedAt:         time.Now(),
				ControlModeAtLog: controlMode,
			}
			database.DB.Create(&mealLog)
			summary = append(summary, fmt.Sprintf("- %s (%.0f kcal)", dishName, estimated.Calories))
		}

		if len(summary) == 0 {
			return "I had some trouble estimating the nutrition for those items. Could you provide more details?"
		}

		newState, _ := ComputeRemainingDayState(user.ID, time.Now())
		modeStr := ""
		if newState != nil && newState.ControlMode == "DAMAGE_CONTROL" {
			modeStr = "\n⚠️ WARNING: You are now in DAMAGE CONTROL mode!"
		}

		resp := "✅ *Logged:*\n" + strings.Join(summary, "\n")
		if newState != nil {
			resp += fmt.Sprintf("\n\n*Remaining today:* %.0f kcal, %.1fg protein.%s",
				newState.RemainingCalories, newState.RemainingProtein, modeStr)
		}
		return resp

	case "set_daily_goal":
		var args map[string]interface{}
		json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		calories := int(args["calories"].(float64))
		if calories <= 0 {
			return "Please specify a valid calorie number above zero."
		}

		var goal models.Goal
		if err := database.DB.Where("user_id = ? AND is_active = ?", user.ID, true).First(&goal).Error; err != nil {
			goal = models.Goal{UserID: user.ID, Title: "My Health Goal", IsActive: true}
			database.DB.Create(&goal)
		}

		var profile models.GoalMacroProfile
		if err := database.DB.Where("goal_id = ?", goal.ID).First(&profile).Error; err != nil {
			// Create default profile with 30/30/40 macro split
			profile = models.GoalMacroProfile{
				GoalID:             goal.ID,
				DailyCalorieTarget: calories,
				DailyProteinTarget: float64(calories) * 0.30 / 4,
				DailyFatTarget:     float64(calories) * 0.30 / 9,
				DailyCarbsTarget:   float64(calories) * 0.40 / 4,
			}
			database.DB.Create(&profile)
		} else {
			database.DB.Model(&profile).Updates(map[string]interface{}{
				"daily_calorie_target": calories,
				"updated_at":           time.Now(),
			})
		}
		return fmt.Sprintf("🎯 *Goal Updated!* Your daily target is now %d calories. I've updated your budget accordingly. Let's go!", calories)

	case "update_meal":
		var args map[string]interface{}
		json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		newIngredients, _ := args["full_new_description"].(string)
		mealType, _ := args["meal_type"].(string)

		var meal models.MealLog
		query := database.DB.Where("user_id = ?", user.ID)
		if mealType != "" {
			query = query.Where("meal_type = ?", mealType)
		}
		if err := query.Order("logged_at desc").First(&meal).Error; err != nil {
			return fmt.Sprintf("I couldn't find a recent %s to update. Should I log this as a new meal instead?", mealType)
		}

		// Re-estimate based on full new description
		estimated, err := ns.EstimateNutritionFromQuery(user.ID, newIngredients)
		if err != nil || estimated == nil {
			return fmt.Sprintf("I tried to update your %s, but I'm having trouble recalculating the nutrition.", meal.Name)
		}

		oldIngredients := meal.Ingredients
		database.DB.Model(&meal).Updates(map[string]interface{}{
			"ingredients": newIngredients,
			"calories":    estimated.Calories,
			"protein":     estimated.Protein,
			"carbs":       estimated.Carbs,
			"fat":         estimated.Fat,
			"updated_at":  time.Now(),
		})

		newState, _ := ComputeRemainingDayState(user.ID, time.Now())
		resp := fmt.Sprintf("✅ *Updated your %s (%s)!*\nPrevious: %s\nNow: %s\n\nNew Total: %.0f kcal, %.1fg protein.",
			meal.MealType, meal.Name, oldIngredients, newIngredients, estimated.Calories, estimated.Protein)
		if newState != nil {
			resp += fmt.Sprintf("\n\n*Remaining today:* %.0f kcal.", newState.RemainingCalories)
		}
		return resp

	case "get_past_day_summary":
		var args map[string]interface{}
		json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		dateStr, _ := args["date"].(string)
		
		targetDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return "Please provide the date in YYYY-MM-DD format."
		}

		pState, _ := ComputeRemainingDayState(user.ID, targetDate)
		if pState == nil {
			return fmt.Sprintf("I don't have any goal data for %s.", dateStr)
		}

		startOfDay := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location())
		var logs []models.MealLog
		database.DB.Where("user_id = ? AND logged_at >= ? AND logged_at < ?", user.ID, startOfDay, startOfDay.Add(24*time.Hour)).Find(&logs)

		summary := fmt.Sprintf("📊 *Summary for %s*:\n", targetDate.Format("Monday, Jan 02"))
		summary += fmt.Sprintf("Total Consumed: %.0f / %.0f kcal\n", pState.TargetCalories-pState.RemainingCalories, pState.TargetCalories)
		summary += "\nMeals:\n"
		if len(logs) == 0 {
			summary += "- No meals logged."
		} else {
			for _, l := range logs {
				summary += fmt.Sprintf("- %s (%.0f kcal)\n", l.Name, l.Calories)
			}
		}
		return summary

	case "ask_advice":
		var args map[string]interface{}
		json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		foodName, _ := args["food_description"].(string)

		state, _ := ComputeRemainingDayState(user.ID, time.Now())
		if state == nil {
			return "I couldn't find your daily goals. Please set them up in the web app!"
		}
		
		estimated, err := ns.EstimateNutritionFromQuery(user.ID, foodName)
		if err != nil || estimated == nil {
			return "I'm having trouble estimating the nutrition for that right now."
		}

		result := services.CheckFoodPermission(state, *estimated)
		decision := "❌ No, you shouldn't eat this."
		if result.Status == "APPROVED" || result.Status == "WARNING" {
			decision = "✅ Yes, you can eat this!"
		}
		
		return fmt.Sprintf("%s\n\n%s\n\nNutrition Est: %.0f kcal, %.1fg protein.", decision, result.Reason, estimated.Calories, estimated.Protein)

	case "get_leftover_budget":
		state, _ := ComputeRemainingDayState(user.ID, time.Now())
		if state == nil {
			return "You haven't set up your goals yet! Please configure them in the web dashboard."
		}
		return fmt.Sprintf("📊 *Your Daily Budget:*\n\nRemaining Calories: %.0f kcal\nRemaining Protein: %.1f g\nRemaining Carbs: %.1f g\nRemaining Fat: %.1f g\n\nCurrent Mode: %s",
			state.RemainingCalories, state.RemainingProtein, state.RemainingCarbs, state.RemainingFat, state.ControlMode)

	case "get_daily_summary":
		now := time.Now()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		var logs []models.MealLog
		database.DB.Where("user_id = ? AND logged_at >= ?", user.ID, startOfDay).Find(&logs)

		if len(logs) == 0 {
			return "You haven't logged any meals today!"
		}

		summary := "🍽️ *Today's Meals:*\n\n"
		var tCals, tProt float64
		for _, l := range logs {
			summary += fmt.Sprintf("- %s (%.0f kcal, %.1fg prot)\n", l.Name, l.Calories, l.Protein)
			tCals += l.Calories
			tProt += l.Protein
		}
		summary += fmt.Sprintf("\n*Total:* %.0f kcal | %.1fg prot", tCals, tProt)
		return summary

	case "update_pantry":
		var args map[string]interface{}
		json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		items, ok := args["items"].([]interface{})
		if !ok {
			return "I couldn't understand the pantry items. Please try again."
		}

		var updatedNames []string
		
		// Group items by ingredient to handle multiple items mapping to same ingredient (e.g. 2 types of Kombucha)
		type pantryChange struct {
			quantity float64
			unit     string
			action   string
			brand    *string
			rawName  string
		}
		ingredientChanges := make(map[uint]*pantryChange)
		ingredientNames := make(map[uint]string)
		
		// 1. Batch AI Ingredient Extraction (with Cache Check)
		var rawNames []string
		var results = make([]llm.PantryItemExtraction, len(items))
		var itemsToExtract []int // indices of items that need LLM extraction
		
		for i, it := range items {
			itemMap, _ := it.(map[string]interface{})
			name, ok := itemMap["name"].(string)
			if !ok { continue }
			rawNames = append(rawNames, name)
			
			// Check Cache
			var mapping models.IngredientMapping
			if err := database.DB.Where("LOWER(raw_name) = ?", strings.ToLower(name)).First(&mapping).Error; err == nil {
				results[i] = llm.PantryItemExtraction{
					Ingredient: mapping.IngredientName,
					Brand:      mapping.Brand,
					Product:    mapping.Product,
				}
				fmt.Printf("[PANTRY_DEBUG] Cache Hit for %s -> %s\n", name, mapping.IngredientName)
			} else {
				itemsToExtract = append(itemsToExtract, i)
			}
		}

		if len(itemsToExtract) > 0 {
			var namesToCall []string
			for _, idx := range itemsToExtract {
				namesToCall = append(namesToCall, rawNames[idx])
			}
			
			fmt.Printf("[PANTRY_DEBUG] Calling Batch Extraction for %d items\n", len(namesToCall))
			extractions, err := llmClient.ExtractPantryItemsBatch(namesToCall)
			if err == nil {
				for i, ext := range extractions {
					originalIdx := itemsToExtract[i]
					results[originalIdx] = ext
					
					// Save to Cache if valid
					if ext.Ingredient != "" {
						var ing models.Ingredient
						database.DB.Where("LOWER(name) = ?", strings.ToLower(ext.Ingredient)).FirstOrCreate(&ing, models.Ingredient{Name: ext.Ingredient})
						
						database.DB.Create(&models.IngredientMapping{
							RawName:        rawNames[originalIdx],
							IngredientID:   ing.ID,
							IngredientName: ing.Name,
							Brand:          ext.Brand,
							Product:        ext.Product,
						})
					}
				}
			}
		}

		// 2. Aggregate Changes
		for i, it := range items {
			itemMap, _ := it.(map[string]interface{})
			quantity, _ := itemMap["quantity"].(float64)
			unit, _ := itemMap["unit"].(string)
			action, _ := itemMap["action"].(string)
			
			var ingredientName string
			if results[i].Ingredient != "" {
				ingredientName = results[i].Ingredient
			} else {
				// Fallback to heuristic
				ingredientName = normalizeIngredientName(rawNames[i])
			}

			// 2.1 Find/Create Ingredient
			var ingredient models.Ingredient
			database.DB.Where("LOWER(name) = ?", strings.ToLower(ingredientName)).FirstOrCreate(&ingredient, models.Ingredient{Name: ingredientName})
			
			// 2.2 Aggregate
			if _, exists := ingredientChanges[ingredient.ID]; !exists {
				ingredientChanges[ingredient.ID] = &pantryChange{
					quantity: quantity,
					unit:     unit,
					action:   action,
					brand:    results[i].Brand,
					rawName:  rawNames[i],
				}
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

		// 4. Apply Aggregated Changes to DB
		for ingID, change := range ingredientChanges {
			var pi models.PantryItem
			err := database.DB.Where("user_id = ? AND ingredient_id = ?", user.ID, ingID).First(&pi).Error
			
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					// Find a representative ItemID (best match for this ingredient)
					var sampleItem models.Item
					database.DB.Where("ingredient_id = ?", ingID).First(&sampleItem)
					
					if sampleItem.ID == 0 {
						// Create a generic item for this ingredient to satisfy the NOT NULL constraint
						var brandID *uint
						if change.brand != nil && *change.brand != "" {
							var b models.Brand
							database.DB.Where("LOWER(name) = ?", strings.ToLower(*change.brand)).FirstOrCreate(&b, models.Brand{Name: *change.brand})
							brandID = &b.ID
						}

						sampleItem = models.Item{
							Name:         change.rawName, // Use the full name from receipt as the item name
							IngredientID: ingID,
							BrandID:      brandID,
							Unit:         change.unit,
						}
						database.DB.Create(&sampleItem)
						fmt.Printf("[PANTRY_DEBUG] Created generic item for %s (Brand: %v)\n", ingredientNames[ingID], change.brand)
					}
					
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

			// Update Stock
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

		fmt.Printf("[PANTRY_DEBUG] Final Updated List: %v\n", updatedNames)
		return fmt.Sprintf("📦 *Pantry Updated!*\nUpdated: %s\nI've noted these changes in your inventory.", strings.Join(updatedNames, ", "))

	case "create_recipe":
		var args map[string]interface{}
		json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		name, _ := args["name"].(string)
		sourceURL, _ := args["source_url"].(string)
		ingredientsRaw, _ := args["ingredients"].([]interface{})
		instructionsRaw, _ := args["instructions"].([]interface{})

		var totalCals, totalProt, totalFat, totalCarbs float64
		var ingredientList []string
		
		ns := services.NewNutritionService()
		
		for _, ing := range ingredientsRaw {
			ingMap := ing.(map[string]interface{})
			ingName, _ := ingMap["name"].(string)
			qty, _ := ingMap["quantity"].(float64)
			unit, _ := ingMap["unit"].(string)
			
			ingDesc := fmt.Sprintf("%.1f %s %s", qty, unit, ingName)
			ingredientList = append(ingredientList, ingDesc)
			
			// Use the new Pantry-First AI estimation logic
			est, _ := ns.EstimateNutritionFromQuery(user.ID, ingDesc)
			if est != nil {
				totalCals += est.Calories
				totalProt += est.Protein
				totalFat += est.Fat
				totalCarbs += est.Carbs
			}
		}

		ingredientsJSON, _ := json.Marshal(ingredientList)
		instructionsJSON, _ := json.Marshal(instructionsRaw)

		recipe := models.Recipe{
			UserID:        user.ID,
			Name:          name,
			Ingredients:   string(ingredientsJSON),
			Instructions:  string(instructionsJSON),
			TotalCalories: totalCals,
			TotalProtein:  totalProt,
			TotalFat:      totalFat,
			TotalCarbs:    totalCarbs,
			SourceURL:     sourceURL,
		}
		database.DB.Create(&recipe)

		resp := fmt.Sprintf("📖 *Recipe Saved: %s*\n\n", name)
		resp += fmt.Sprintf("🔥 *Nutrition Info (Full Batch):*\n")
		resp += fmt.Sprintf("- Calories: %.0f kcal\n- Protein: %.1fg\n- Carbs: %.1fg\n- Fat: %.1fg\n\n", totalCals, totalProt, totalCarbs, totalFat)
		resp += "I've saved this to your collection. You can log it anytime by name!"
		return resp

	case "get_pantry":
		var items []models.PantryItem
		database.DB.Preload("Ingredient").Preload("Item").Where("user_id = ?", user.ID).Find(&items)
		
		validCount := 0
		msg := "📦 *Your Pantry Inventory:*\n\n"
		for _, it := range items {
			qty := it.EffectiveQuantity()
			if qty > 0 {
				unit := "units"
				if it.Item.Unit != "" {
					unit = it.Item.Unit
				}
				msg += fmt.Sprintf("- %s: %.1f %s\n", it.Ingredient.Name, qty, unit)
				validCount++
			}
		}

		if validCount == 0 {
			return "Your pantry is empty! Add items by saying things like 'Add 2kg rice to my pantry'."
		}
		return msg

	case "get_recipes":
		var recipes []models.Recipe
		database.DB.Where("user_id = ?", user.ID).Find(&recipes)
		
		if len(recipes) == 0 {
			return "You haven't saved any recipes yet! Share a photo or link to create one."
		}
		
		msg := "📖 *Your Saved Recipes:*\n\n"
		for _, r := range recipes {
			msg += fmt.Sprintf("- *%s*: %.0f kcal\n", r.Name, r.TotalCalories)
		}
		msg += "\nSay 'Show recipe for [name]' for details."
		return msg

	case "delete_recipe":
		var args map[string]interface{}
		json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		name, _ := args["name"].(string)
		
		err := database.DB.Where("user_id = ? AND LOWER(name) = ?", user.ID, strings.ToLower(name)).Delete(&models.Recipe{}).Error
		if err != nil {
			return fmt.Sprintf("Failed to delete recipe '%s'.", name)
		}
		return fmt.Sprintf("🗑️ Deleted recipe: *%s*", name)

	default:
		return "I'm not quite sure how to handle that yet."
	}
}

// SendWhatsAppMessage sends a text message back to the user
func SendWhatsAppMessage(to string, text string) error {
	accessToken := config.GetEnv("WHATSAPP_ACCESS_TOKEN", "")
	phoneNumberID := config.GetEnv("WHATSAPP_PHONE_NUMBER_ID", "")

	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/messages", phoneNumberID)

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              "text",
		"text": map[string]string{
			"preview_url": "false",
			"body":        text,
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending WhatsApp message: %v\n", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		fmt.Printf("WhatsApp API Error: %s\n", string(respBody))
		return fmt.Errorf("API error: %s", string(respBody))
	}

	return nil
}
func SendWhatsAppInteractiveButtons(to string, text string, buttons []map[string]string) error {
	accessToken := config.GetEnv("WHATSAPP_ACCESS_TOKEN", "")
	phoneNumberID := config.GetEnv("WHATSAPP_PHONE_NUMBER_ID", "")

	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/messages", phoneNumberID)

	var buttonRows []map[string]interface{}
	for _, b := range buttons {
		buttonRows = append(buttonRows, map[string]interface{}{
			"type": "reply",
			"reply": map[string]string{
				"id":    b["id"],
				"title": b["title"],
			},
		})
	}

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              "interactive",
		"interactive": map[string]interface{}{
			"type": "button",
			"body": map[string]string{
				"text": text,
			},
			"action": map[string]interface{}{
				"buttons": buttonRows,
			},
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		fmt.Printf("WhatsApp Interactive Error: %s\n", string(respBody))
		return fmt.Errorf("API error: %s", string(respBody))
	}

	return nil
}
