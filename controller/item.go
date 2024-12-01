package controller

import (

	// Update the import path to reflect the correct package path
	"pateproject/entity"
	"pateproject/repository"

	"context"
)

type ItemController interface {
	GetItem(ctx context.Context, id int) (*entity.Item, error)
	CreateItem(ctx context.Context, item *entity.Item) error
	UpdateItem(ctx context.Context, item *entity.Item) error
	DeleteItem(ctx context.Context, id int) error
}

// itemController struct
type itemController struct {
	itemRepository repository.ItemRepository
}

func NewitemController(itemRepository *repository.ItemRepository) itemController {
	return itemController{
		itemRepository: *itemRepository,
	}
}

// GetItem retrieves a single item by ID
func (c itemController) GetItem(ctx context.Context, id int) (*entity.Item, error) {
	item, err := c.itemRepository.GetItemByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// CreateItem adds a new item to the inventory
func (c itemController) CreateItem(ctx context.Context, item *entity.Item) error {
	err := c.itemRepository.CreateItem(ctx, item)
	if err != nil {
		return err
	}
	return nil
}

// UpdateItem modifies an existing item
func (c itemController) UpdateItem(ctx context.Context, item *entity.Item) error {
	err := c.itemRepository.UpdateItem(ctx, item)
	if err != nil {
		return err
	}
	return nil
}

// DeleteItem removes an item by ID
func (c itemController) DeleteItem(ctx context.Context, id int) error {
	err := c.itemRepository.DeleteItem(ctx, id)
	return err
}
