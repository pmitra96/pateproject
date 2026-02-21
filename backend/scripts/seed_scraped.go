package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/models"
	"gorm.io/gorm/clause"
)

type ScrapedData struct {
	Brands   []ScrapedBrand   `json:"brands"`
	Products []ScrapedProduct `json:"products"`
}

type ScrapedBrand struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type ScrapedProduct struct {
	ID            string                 `json:"id"`
	BrandID       string                 `json:"brand_id"`
	Name          string                 `json:"name"`
	ZeptoID       string                 `json:"zepto_id"`
	URL           string                 `json:"url"`
	ImageURL      string                 `json:"image_url"`
	Category      string                 `json:"category"`
	NutritionInfo map[string]interface{} `json:"nutrition_info"`
	Weight        string                 `json:"weight"`
	Unit          string                 `json:"unit"`
}

func main() {
	// Initialize database
	database.InitDB()
	db := database.DB

	// Load seed data
	seedPath := "seeds/scraped_data_seed.json.gz"
	fmt.Printf("Loading seed data from %s...\n", seedPath)

	f, err := os.Open(seedPath)
	if err != nil {
		log.Fatalf("Failed to open seed file: %v", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		log.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gr.Close()

	var data ScrapedData
	if err := json.NewDecoder(gr).Decode(&data); err != nil {
		log.Fatalf("Failed to decode seed data: %v", err)
	}

	fmt.Printf("Seeding %d brands and %d products...\n", len(data.Brands), len(data.Products))

	// Map Scraped Brand ID to Backend Brand ID
	brandMap := make(map[string]uint)

	// Seed Brands
	for _, b := range data.Brands {
		brand := models.Brand{
			Name: b.Name,
		}

		err := db.Where("name = ?", b.Name).Attrs(models.Brand{Name: b.Name}).FirstOrCreate(&brand).Error
		if err != nil {
			fmt.Printf("Error seeding brand %s: %v\n", b.Name, err)
			continue
		}
		brandMap[b.ID] = brand.ID
	}

	// Seed Products (Items)
	for _, p := range data.Products {
		// 1. Ensure Ingredient exists (we'll use Category as the base ingredient name if possible)
		ingredientName := p.Category
		if ingredientName == "" {
			ingredientName = "Uncategorized"
		}

		ingredient := models.Ingredient{
			Name: ingredientName,
		}
		if err := db.Where("name = ?", ingredientName).FirstOrCreate(&ingredient).Error; err != nil {
			fmt.Printf("Error seeding ingredient %s: %v\n", ingredientName, err)
			continue
		}

		// 2. Map Brand ID
		var brandID *uint
		if bID, ok := brandMap[p.BrandID]; ok {
			brandID = &bID
		}

		// 3. Extract Nutrition
		calories := getFloat(p.NutritionInfo, "energy")
		protein := getFloat(p.NutritionInfo, "protein")
		carbs := getFloat(p.NutritionInfo, "carbohydrates")
		fat := getFloat(p.NutritionInfo, "fat")

		// 4. Create/Update Item
		item := models.Item{
			Name:              p.Name,
			IngredientID:      ingredient.ID,
			BrandID:           brandID,
			ProductName:       p.Name,
			Unit:              p.Unit,
			Calories:          calories,
			Protein:           protein,
			Carbs:             carbs,
			Fat:               fat,
			NutritionVerified: true,
		}

		// Use OnConflict to update if Name already exists
		err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}},
			UpdateAll: true,
		}).Create(&item).Error

		if err != nil {
			fmt.Printf("Error seeding item %s: %v\n", p.Name, err)
		}
	}

	fmt.Println("Seeding completed successfully.")
}

func getFloat(m map[string]interface{}, key string) float64 {
	if m == nil {
		return 0
	}
	val, ok := m[key]
	if !ok {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return 0
	}
}
