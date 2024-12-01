package repository

import (
	"context"
	"pateproject/entity"
	"pateproject/mapper"
	"pateproject/model"

	"gorm.io/gorm"
)

// ItemRepository is a struct that holds the database connection.
type ItemRepository struct {
	DB *gorm.DB
}

// NewItemRepository creates and returns a new ItemRepository.
func NewItemRepository(db *gorm.DB) *ItemRepository {
	return &ItemRepository{
		DB: db,
	}
}

// CreateItem creates a new item in the database.
func (r *ItemRepository) CreateItem(ctx context.Context, itemEntity *entity.Item) error {
	// Convert entity to model
	itemModel := mapper.ItemEntityToModel(itemEntity)

	// Store in the database using GORM
	if err := r.DB.WithContext(ctx).Create(itemModel).Error; err != nil {
		return err
	}
	return nil
}

// GetItemByID fetches an item from the database by ID.
func (r *ItemRepository) GetItemByID(ctx context.Context, id int) (*entity.Item, error) {
	var itemModel model.Item
	if err := r.DB.WithContext(ctx).First(&itemModel, id).Error; err != nil {
		return nil, err
	}
	// Convert model back to entity
	itemEntity := mapper.ItemModelToEntity(&itemModel)
	return itemEntity, nil
}

// UpdateItem updates an existing item in the database.
func (r *ItemRepository) UpdateItem(ctx context.Context, itemEntity *entity.Item) error {
	// Convert entity to model
	itemModel := mapper.ItemEntityToModel(itemEntity)

	// Update the record in the database using GORM
	if err := r.DB.WithContext(ctx).Save(itemModel).Error; err != nil {
		return err
	}
	return nil
}

// DeleteItem deletes an item from the database by ID.
func (r *ItemRepository) DeleteItem(ctx context.Context, id int) error {
	if err := r.DB.WithContext(ctx).Delete(&model.Item{}, id).Error; err != nil {
		return err
	}
	return nil
}
