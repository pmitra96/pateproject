package controller

import (
	"pateproject/entity"
	"pateproject/repository"

	"context"
)

type UnitController interface {
	GetUnit(ctx context.Context, id int) (*entity.Unit, error)
	CreateUnit(ctx context.Context, unit *entity.Unit) error
	UpdateUnit(ctx context.Context, unit *entity.Unit) error
	DeleteUnit(ctx context.Context, id int) error
}

type unitController struct {
	unitRepository repository.UnitRepository
}

func NewUnitController(unitRepository *repository.UnitRepository) UnitController {
	return &unitController{
		unitRepository: *unitRepository,
	}
}

func (c unitController) GetUnit(ctx context.Context, id int) (*entity.Unit, error) {
	unit, err := c.unitRepository.GetUnitByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return unit, nil
}

func (c unitController) CreateUnit(ctx context.Context, unit *entity.Unit) error {
	err := c.unitRepository.CreateUnit(ctx, unit)
	if err != nil {
		return err
	}
	return nil
}

func (c unitController) UpdateUnit(ctx context.Context, unit *entity.Unit) error {
	err := c.unitRepository.UpdateUnit(ctx, unit)
	if err != nil {
		return err
	}
	return nil
}

func (c unitController) DeleteUnit(ctx context.Context, id int) error {
	err := c.unitRepository.DeleteUnit(ctx, id)
	return err
}
