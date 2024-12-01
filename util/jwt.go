package util

import (
	"time"

	"github.com/dgrijalva/jwt-go"
)

// Claims defines the structure of the JWT payload.
type Claims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	jwt.StandardClaims
}

// GenerateJWT creates a JWT token with the given user information.
func GenerateJWT(userID uint, email string, jwtSecretKey []byte) (string, error) {
	// Set expiration time for the JWT token
	expirationTime := time.Now().Add(24 * time.Hour) // 24 hours expiration

	// Create the JWT claims
	claims := &Claims{
		UserID: userID,
		Email:  email,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
			IssuedAt:  time.Now().Unix(),
			Issuer:    "pateproject", // Issuer information
		},
	}

	// Create the JWT token using the claims and signing with the secret key
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token
	tokenString, err := token.SignedString(jwtSecretKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateJWT verifies the JWT token and returns the claims if valid.
func ValidateJWT(tokenString string, jwtSecretKey []byte) (*Claims, error) {
	// Parse and validate the token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Return the secret key to validate the signature
		return jwtSecretKey, nil
	})

	if err != nil {
		return nil, err
	}

	// Check if the token is valid
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, err
}
