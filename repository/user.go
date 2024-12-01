package repository

import (
	"context"
	"pateproject/entity"
	"pateproject/mapper"
	"pateproject/model"

	"gorm.io/gorm"
)

// UserRepository is a struct that holds the database connection.
type UserRepository struct {
	DB *gorm.DB
}

// NewUserRepository creates and returns a new UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		DB: db,
	}
}

// CreateUser creates a new user in the database.
func (r *UserRepository) CreateUser(ctx context.Context, userEntity *entity.User) error {
	// Convert entity to model
	userModel := mapper.UserEntityToModel(userEntity)

	// Store in the database using GORM
	if err := r.DB.WithContext(ctx).Create(userModel).Error; err != nil {
		return err
	}
	return nil
}

// GetUserByID fetches a user from the database by ID.
func (r *UserRepository) GetUserByID(ctx context.Context, id int) (*entity.User, error) {
	var userModel model.User
	if err := r.DB.WithContext(ctx).First(&userModel, id).Error; err != nil {
		return nil, err
	}
	// Convert model back to entity
	userEntity := mapper.UserModelToEntity(&userModel)
	return userEntity, nil
}

// UpdateUser updates an existing user in the database.
func (r *UserRepository) UpdateUser(ctx context.Context, userEntity *entity.User) error {
	// Convert entity to model
	userModel := mapper.UserEntityToModel(userEntity)

	// Update the record in the database using GORM
	if err := r.DB.WithContext(ctx).Model(&model.User{}).Where("id = ?", userModel.ID).Updates(userModel).Error; err != nil {
		return err
	}
	return nil
}

// DeleteUser deletes a user from the database by ID.
func (r *UserRepository) DeleteUser(ctx context.Context, id int) error {
	if err := r.DB.WithContext(ctx).Delete(&model.User{}, id).Error; err != nil {
		return err
	}
	return nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	var userModel model.User
	if err := r.DB.WithContext(ctx).Where("email = ?", email).First(&userModel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // or return a custom error if you prefer
		}
		return nil, err
	}
	// Convert model back to entity
	userEntity := mapper.UserModelToEntity(&userModel)
	return userEntity, nil
}
