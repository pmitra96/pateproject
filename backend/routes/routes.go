package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/pmitra96/pateproject/controllers"
	auth "github.com/pmitra96/pateproject/middleware"
)

func SetupRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS Configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-API-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Public / Auth
	// r.Post("/auth/login", ...) // If we had real auth

	// Ingestion (API Key protected)
	r.Group(func(r chi.Router) {
		r.Use(auth.APIKeyMiddleware)
		r.Post("/ingest/order", controllers.IngestOrder)
	})

	// LLM Routes (public for now, add auth as needed)
	r.Post("/llm/story", controllers.GenerateStory)
	r.Post("/llm/suggest-meal", controllers.SuggestMeal)

	// User Routes (OAuth/UserContext protected)
	r.Group(func(r chi.Router) {
		r.Use(auth.OAuthMiddleware)
		r.Get("/pantry", controllers.GetPantry)
		r.Patch("/pantry/{item_id}", controllers.UpdatePantryItem)
		r.Delete("/pantry/{item_id}", controllers.DeletePantryItem)
		r.Get("/pantry/low-stock", controllers.GetLowStock)
		r.Get("/items", controllers.GetItems)
		r.Post("/items", controllers.CreateItem)
		r.Post("/items/extract", controllers.ExtractItems)
		r.Get("/orders", controllers.GetOrders)

		// Goals
		r.Get("/goals", controllers.GetGoals)
		r.Post("/goals", controllers.CreateGoal)
		r.Delete("/goals/{goal_id}", controllers.DeleteGoal)

		// LLM with auth (for personalized suggestions)
		r.Post("/llm/suggest-meal-personalized", controllers.SuggestMealPersonalized)
	})

	return r
}
