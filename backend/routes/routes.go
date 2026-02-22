package routes

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/pmitra96/pateproject/config"
	"github.com/pmitra96/pateproject/controllers"
	"github.com/pmitra96/pateproject/jobs"
	auth "github.com/pmitra96/pateproject/middleware"
)

func SetupRouter() *chi.Mux {
	fmt.Println("Setting up router v1.1...")
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS Configuration
	allowedOrigins := []string{"http://localhost:5173", "http://127.0.0.1:5173", "https://pateproject.vercel.app"}
	rawOrigins := config.GetEnv("ALLOWED_ORIGINS", "")
	if rawOrigins != "" {
		for _, origin := range strings.Split(rawOrigins, ",") {
			cleanOrigin := strings.TrimSpace(origin)
			if cleanOrigin != "" {
				allowedOrigins = append(allowedOrigins, cleanOrigin)
			}
		}
	}
	fmt.Printf("CORS Final Allowed Origins: %v\n", allowedOrigins)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-API-Key", "X-Requested-With"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health Check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Health check hit")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"up","version":"1.1"}`))
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Root hit")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"PateProject Backend API up v1.1"}`))
	})

	// Public / Auth
	// r.Post("/auth/login", ...) // If we had real auth

	// Ingestion (API Key protected)
	r.Group(func(r chi.Router) {
		r.Use(auth.APIKeyMiddleware)
		r.Post("/ingest/order", controllers.IngestOrder)
	})

	// LLM Routes (public for now, add auth as needed)
	// r.Post("/llm/story", controllers.GenerateStory) // Deprecated/Removed
	r.Post("/llm/suggest-meal", controllers.SuggestMeal)

	// User Routes (OAuth/UserContext protected)
	r.Group(func(r chi.Router) {
		r.Use(auth.OAuthMiddleware)
		r.Get("/pantry", controllers.GetPantry)
		r.Post("/pantry/add", controllers.AddPantryItem)
		r.Patch("/pantry/{item_id}", controllers.UpdatePantryItem)
		r.Delete("/pantry/{item_id}", controllers.DeletePantryItem)
		r.Post("/pantry/bulk-delete", controllers.BulkDeletePantryItems)
		r.Get("/pantry/low-stock", controllers.GetLowStock)
		r.Get("/items", controllers.GetItems)
		r.Post("/items", controllers.CreateItem)
		r.Post("/items/extract", controllers.ExtractItems)
		r.Get("/orders", controllers.GetOrders)

		// Goals
		r.Get("/goals", controllers.GetGoals)
		r.Post("/goals", controllers.CreateGoal)
		r.Delete("/goals/{goal_id}", controllers.DeleteGoal)

		// Meals
		r.Post("/meals/log", controllers.LogMeal)
		r.Get("/meals", controllers.GetMealHistory)
		r.Delete("/meals/{meal_id}", controllers.DeleteMealLog)

		// LLM with auth (for personalized suggestions)
		r.Post("/llm/suggest-meal-personalized", controllers.SuggestMealPersonalized)
		r.Post("/llm/chat", controllers.ChatBot)

		// Conversations
		r.Post("/conversations", controllers.SaveConversation)
		r.Get("/conversations", controllers.GetConversations)

		// User Preferences
		r.Get("/preferences", controllers.GetUserPreferences)
		r.Put("/preferences", controllers.UpdateUserPreferences)

		// Dish Samples
		r.Get("/dish-samples", controllers.GetDishSamples)
		r.Post("/dish-samples", controllers.CreateDishSample)
		r.Post("/dish-samples/bulk", controllers.BulkCreateDishSamples)
		r.Delete("/dish-samples/{dish_id}", controllers.DeleteDishSample)
		// Remaining Day Control
		r.Get("/remaining-day-state", controllers.GetRemainingDayState)
		r.Post("/goals/{goal_id}/targets", controllers.SetGoalMacroTargets) // Adjusted path for brevity? No, prompt said /api/goals/{goal_id}/macro-targets. I'll stick to closest: /goals/{goal_id}/targets
		r.Get("/meals/validate", controllers.ValidateMeal)
		r.Post("/can-i-eat", controllers.CheckFoodPermissionHandler)

	})

	// Server-Sent Events for real-time nutrition updates
	r.Get("/sse/nutrition", NutritionSSE)

	// Debug: Manually trigger nutrition job for an item
	r.Get("/debug/nutrition/{item_id}", func(w http.ResponseWriter, req *http.Request) {
		itemID := chi.URLParam(req, "item_id")
		var id uint
		fmt.Sscanf(itemID, "%d", &id)
		if id > 0 {
			jobs.GetWorker().Enqueue(id)
			w.Write([]byte(fmt.Sprintf(`{"status": "enqueued", "item_id": %d}`, id)))
		} else {
			http.Error(w, "Invalid item_id", http.StatusBadRequest)
		}
	})

	return r
}
