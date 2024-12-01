package mapper

import (
	"pateproject/entity"
	"pateproject/model"
	"pateproject/util"
)

// ItemEntityToModel maps an Item entity to the corresponding model.
func ItemEntityToModel(entity *entity.Item) *model.Item {
	return &model.Item{
		ID:              entity.ID,
		Name:            entity.Name,
		Category:        entity.Category,
		DefaultUnitID:   entity.DefaultUnitID,
		CaloriesPerUnit: entity.CaloriesPerUnit,
	}
}

// ItemModelToEntity maps an Item model to the corresponding entity.
func ItemModelToEntity(model *model.Item) *entity.Item {
	return &entity.Item{
		ID:              model.ID,
		Name:            model.Name,
		Category:        model.Category,
		DefaultUnitID:   model.DefaultUnitID,
		CaloriesPerUnit: model.CaloriesPerUnit,
	}
}

// UnitEntityToModel maps a Unit entity to the corresponding model.
func UnitEntityToModel(entity *entity.Unit) *model.Unit {
	return &model.Unit{
		ID:               entity.ID,
		Name:             entity.Name,
		Abbreviation:     entity.Abbreviation,
		ConversionFactor: entity.ConversionFactor,
	}
}

// UnitModelToEntity maps a Unit model to the corresponding entity.
func UnitModelToEntity(model *model.Unit) *entity.Unit {
	return &entity.Unit{
		ID:               model.ID,
		Name:             model.Name,
		Abbreviation:     model.Abbreviation,
		ConversionFactor: model.ConversionFactor,
	}
}

// UserEntityToModel maps a User entity to the corresponding model.
func UserEntityToModel(entity *entity.User) *model.User {

	hashedPassword, err := util.HashPassword(entity.Password)
	if err != nil {
		return nil
	}
	return &model.User{
		ID:       entity.ID,
		Name:     entity.Name,
		Email:    entity.Email,
		Password: hashedPassword,
	}
}

// UserModelToEntity maps a User model to the corresponding entity.
func UserModelToEntity(model *model.User) *entity.User {
	return &entity.User{
		ID:        model.ID,
		Name:      model.Name,
		Email:     model.Email,
		Password:  string(model.Password),
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}

// UserInventoryEntityToModel maps a UserInventory entity to the corresponding model.
func UserInventoryEntityToModel(entity *entity.UserInventory) *model.UserInventory {
	return &model.UserInventory{
		ID:           entity.ID,
		UserID:       entity.UserID,
		IngredientID: entity.IngredientID,
		Quantity:     entity.Quantity,
		UnitID:       entity.UnitID,
		LastUpdated:  entity.LastUpdated,
	}
}

// UserInventoryModelToEntity maps a UserInventory model to the corresponding entity.
func UserInventoryModelToEntity(model *model.UserInventory) *entity.UserInventory {
	return &entity.UserInventory{
		ID:           model.ID,
		UserID:       model.UserID,
		IngredientID: model.IngredientID,
		Quantity:     model.Quantity,
		UnitID:       model.UnitID,
		LastUpdated:  model.LastUpdated,
	}
}

// RecipeEntityToModel maps a Recipe entity to the corresponding model.
func RecipeEntityToModel(entity *entity.Recipe) *model.Recipe {
	return &model.Recipe{
		ID:          entity.ID,
		Name:        entity.Name,
		Description: entity.Description,
		Calories:    entity.Calories,
	}
}

// RecipeModelToEntity maps a Recipe model to the corresponding entity.
func RecipeModelToEntity(model *model.Recipe) *entity.Recipe {
	return &entity.Recipe{
		ID:          model.ID,
		Name:        model.Name,
		Description: model.Description,
		Calories:    model.Calories,
	}
}

// RecipeIngredientEntityToModel maps a RecipeIngredient entity to the corresponding model.
func RecipeIngredientEntityToModel(entity *entity.RecipeIngredient) *model.RecipeIngredient {
	return &model.RecipeIngredient{
		ID:           entity.ID,
		RecipeID:     entity.RecipeID,
		IngredientID: entity.IngredientID,
		Quantity:     entity.Quantity,
		UnitID:       entity.UnitID,
	}
}

// RecipeIngredientModelToEntity maps a RecipeIngredient model to the corresponding entity.
func RecipeIngredientModelToEntity(model *model.RecipeIngredient) *entity.RecipeIngredient {
	return &entity.RecipeIngredient{
		ID:           model.ID,
		RecipeID:     model.RecipeID,
		IngredientID: model.IngredientID,
		Quantity:     model.Quantity,
		UnitID:       model.UnitID,
	}
}
