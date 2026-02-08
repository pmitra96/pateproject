package routes

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pmitra96/pateproject/jobs"
	"github.com/pmitra96/pateproject/logger"
)

// NutritionSSE handles Server-Sent Events for nutrition updates
func NutritionSSE(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get flusher to send data immediately
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Subscribe to nutrition updates
	updateCh := make(chan jobs.NutritionUpdate, 10)
	worker := jobs.GetWorker()
	worker.Subscribe(updateCh)

	logger.Info("SSE client connected")

	// Send initial connection message
	fmt.Fprintf(w, "event: connected\ndata: {\"status\": \"connected\"}\n\n")
	flusher.Flush()

	// Handle client disconnect
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			logger.Info("SSE client disconnected")
			worker.Unsubscribe(updateCh)
			return
		case update := <-updateCh:
			data, err := json.Marshal(update)
			if err != nil {
				logger.Error("Failed to marshal nutrition update", "error", err)
				continue
			}
			fmt.Fprintf(w, "event: nutrition_update\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}
