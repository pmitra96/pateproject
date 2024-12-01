package controller

import (
	"context"
	"pateproject/entity"
	"pateproject/repository"
	"pateproject/util"
)

// UserController interface
type UserController interface {
	GetUser(ctx context.Context, id int) (*entity.User, error)
	CreateUser(ctx context.Context, user *entity.User) error
	UpdateUser(ctx context.Context, user *entity.User) error
	DeleteUser(ctx context.Context, id int) error
	GetUserByEmail(ctx context.Context, email string) (*entity.User, error)
}

// userController struct
type userController struct {
	userRepository repository.UserRepository
}

// NewUserController creates and returns a new UserController
func NewUserController(userRepository *repository.UserRepository) UserController {
	return &userController{
		userRepository: *userRepository,
	}
}

// GetUser retrieves a single user by ID
func (c *userController) GetUser(ctx context.Context, id int) (*entity.User, error) {
	user, err := c.userRepository.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// CreateUser adds a new user to the database
func (c *userController) CreateUser(ctx context.Context, user *entity.User) error {
	if err := util.ValidatePassword(user.Password); err != nil {
		return err
	}
	err := c.userRepository.CreateUser(ctx, user)
	if err != nil {
		return err
	}
	return nil
}

// UpdateUser modifies an existing user
func (c *userController) UpdateUser(ctx context.Context, user *entity.User) error {
	err := c.userRepository.UpdateUser(ctx, user)
	if err != nil {
		return err
	}
	return nil
}

// DeleteUser removes a user by ID
func (c *userController) DeleteUser(ctx context.Context, id int) error {
	err := c.userRepository.DeleteUser(ctx, id)
	return err
}

func (c *userController) GetUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	user, err := c.userRepository.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return user, nil
}
