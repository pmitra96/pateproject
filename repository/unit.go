package repository

import (
	"context"
	"pateproject/entity"
	"pateproject/mapper"
	"pateproject/model"

	"gorm.io/gorm"
)

// UnitRepository is a struct that holds the database connection for Unit.
type UnitRepository struct {
	DB *gorm.DB
}

// NewUnitRepository creates and returns a new UnitRepository.
func NewUnitRepository(db *gorm.DB) *UnitRepository {
	return &UnitRepository{
		DB: db,
	}
}

// CreateUnit creates a new unit in the database.
func (r *UnitRepository) CreateUnit(ctx context.Context, unitEntity *entity.Unit) error {
	unitModel := mapper.UnitEntityToModel(unitEntity)
	if err := r.DB.WithContext(ctx).Create(unitModel).Error; err != nil {
		return err
	}
	return nil
}

// GetUnitByID fetches a unit from the database by ID.
func (r *UnitRepository) GetUnitByID(ctx context.Context, id int) (*entity.Unit, error) {
	var unitModel model.Unit
	if err := r.DB.WithContext(ctx).First(&unitModel, id).Error; err != nil {
		return nil, err
	}
	unitEntity := mapper.UnitModelToEntity(&unitModel)
	return unitEntity, nil
}

// UpdateUnit updates an existing unit in the database.
func (r *UnitRepository) UpdateUnit(ctx context.Context, unitEntity *entity.Unit) error {
	unitModel := mapper.UnitEntityToModel(unitEntity)
	if err := r.DB.WithContext(ctx).Save(unitModel).Error; err != nil {
		return err
	}
	return nil
}

func (r *UnitRepository) DeleteUnit(ctx context.Context, id int) error {
	if err := r.DB.WithContext(ctx).Delete(&model.Unit{}, id).Error; err != nil {
		return err
	}
	return nil
}
