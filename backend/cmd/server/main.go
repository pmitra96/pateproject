package main

import (
	"net/http"

	"github.com/joho/godotenv"
	"github.com/pmitra96/pateproject/config"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/jobs"
	"github.com/pmitra96/pateproject/logger"
	"github.com/pmitra96/pateproject/routes"
)

func main() {
	// Initialize Structured Logger
	logger.Init()

	// Load .env
	if err := godotenv.Load(); err != nil {
		logger.Warn("No .env file found, using system env vars")
	}

	// Initialize DB
	logger.Info("Checking Configuration...")
	logger.Info("DATABASE_URL", "length", len(config.GetEnv("DATABASE_URL", "")))
	logger.Info("PYTHON_EXTRACTOR_URL", "url", config.GetEnv("PYTHON_EXTRACTOR_URL", ""))
	logger.Info("ALLOWED_ORIGINS", "origins", config.GetEnv("ALLOWED_ORIGINS", ""))

	database.InitDB()

	// Start background nutrition worker
	jobs.GetWorker()

	// Setup Router
	r := routes.SetupRouter()

	port := config.GetEnv("PORT", "8080")
	logger.Info("PateProject Backend v1.1 Starting", "port", port, "revision", "67ff30a+")
	logger.Info("Listening on", "addr", ":"+port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		logger.Error("Server failed to start", "error", err)
	}
}
