package service

import (
	"context"
	"pateproject/controller"
	"pateproject/entity"
	"pateproject/util"

	"golang.org/x/crypto/bcrypt"
)

// AuthController interface
type AuthService interface {
	Login(ctx context.Context, email, password string) (*entity.User, string, error)
}

// authController struct
type authService struct {
	userController controller.UserController
	jwtSecretKey   []byte
}

// NewAuthController creates and returns a new AuthController
func NewAuthService(userController controller.UserController, config *entity.Config) AuthService {
	return &authService{
		userController: userController,
		jwtSecretKey:   config.JWTSecretKey,
	}
}

// Login handles user authentication
func (a *authService) Login(ctx context.Context, email, password string) (*entity.User, string, error) {
	// Fetch the user by email
	user, err := a.userController.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, "", err
	}

	// Compare the provided password with the stored hashed password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, "", err
	}

	// Generate a JWT token
	token, err := util.GenerateJWT(user.ID, user.Email, a.jwtSecretKey)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}
