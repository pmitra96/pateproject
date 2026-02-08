package jobs

import (
	"sync"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/logger"
	"github.com/pmitra96/pateproject/models"
	"github.com/pmitra96/pateproject/services"
)

// NutritionJob represents a job to fetch nutrition data for an item
type NutritionJob struct {
	ItemID uint
}

// NutritionWorker processes nutrition jobs in the background
type NutritionWorker struct {
	jobs         chan NutritionJob
	nutritionSvc *services.NutritionService
	subscribers  map[chan NutritionUpdate]bool
	subMux       sync.RWMutex
}

// NutritionUpdate is sent to SSE subscribers when nutrition data is updated
type NutritionUpdate struct {
	ItemID   uint    `json:"item_id"`
	Calories float64 `json:"calories"`
	Protein  float64 `json:"protein"`
	Carbs    float64 `json:"carbs"`
	Fat      float64 `json:"fat"`
	Fiber    float64 `json:"fiber"`
	Verified bool    `json:"nutrition_verified"`
}

var (
	worker     *NutritionWorker
	workerOnce sync.Once
)

// GetWorker returns the singleton NutritionWorker instance
func GetWorker() *NutritionWorker {
	workerOnce.Do(func() {
		worker = &NutritionWorker{
			jobs:         make(chan NutritionJob, 100),
			nutritionSvc: services.NewNutritionService(),
			subscribers:  make(map[chan NutritionUpdate]bool),
		}
		go worker.run()
		logger.Info("Nutrition worker started")
	})
	return worker
}

// Enqueue adds a nutrition job to the queue
func (w *NutritionWorker) Enqueue(itemID uint) {
	select {
	case w.jobs <- NutritionJob{ItemID: itemID}:
		logger.Info("Nutrition job enqueued", "item_id", itemID)
	default:
		logger.Warn("Nutrition job queue full, dropping job", "item_id", itemID)
	}
}

// Subscribe registers a channel to receive nutrition updates
func (w *NutritionWorker) Subscribe(ch chan NutritionUpdate) {
	w.subMux.Lock()
	defer w.subMux.Unlock()
	w.subscribers[ch] = true
}

// Unsubscribe removes a channel from nutrition updates
func (w *NutritionWorker) Unsubscribe(ch chan NutritionUpdate) {
	w.subMux.Lock()
	defer w.subMux.Unlock()
	delete(w.subscribers, ch)
	close(ch)
}

// run processes jobs from the queue
func (w *NutritionWorker) run() {
	for job := range w.jobs {
		w.processJob(job)
	}
}

func (w *NutritionWorker) processJob(job NutritionJob) {
	logger.Info("Processing nutrition job", "item_id", job.ItemID)

	// Fetch item from database
	var item models.Item
	if err := database.DB.Preload("Ingredient").Preload("Brand").First(&item, job.ItemID).Error; err != nil {
		logger.Error("Failed to fetch item for nutrition job", "item_id", job.ItemID, "error", err)
		return
	}

	// Skip if already verified
	if item.NutritionVerified {
		logger.Info("Item already has verified nutrition, skipping", "item_id", job.ItemID)
		return
	}

	// Fetch nutrition data
	if err := w.nutritionSvc.FetchItemNutrition(&item); err != nil {
		logger.Warn("Failed to fetch nutrition for item", "item_id", job.ItemID, "error", err)
	}

	// Update database
	if err := database.DB.Save(&item).Error; err != nil {
		logger.Error("Failed to save nutrition data", "item_id", job.ItemID, "error", err)
		return
	}

	logger.Info("Nutrition data updated", "item_id", job.ItemID, "calories", item.Calories)

	// Broadcast update to subscribers
	update := NutritionUpdate{
		ItemID:   item.ID,
		Calories: item.Calories,
		Protein:  item.Protein,
		Carbs:    item.Carbs,
		Fat:      item.Fat,
		Fiber:    item.Fiber,
		Verified: item.NutritionVerified,
	}

	w.subMux.RLock()
	for ch := range w.subscribers {
		select {
		case ch <- update:
		default:
			// Drop update if subscriber is slow
		}
	}
	w.subMux.RUnlock()
}
