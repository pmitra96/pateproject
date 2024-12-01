package util

import (
	"errors"
	"log"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

func HandleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// HashPassword takes a plain-text password and returns a hashed password.
func HashPassword(password string) ([]byte, error) {
	// Generate a salt and hash the password
	emptyBytes := []byte{}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return emptyBytes, err
	}
	return hash, nil
}

// CheckPasswordHash compares the given password with the stored hash.
func CheckPasswordHash(password, hash string) bool {
	// Compare the plain password with the stored hashed password
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil // Returns true if passwords match, false otherwise
}

// ValidatePassword checks if a password meets the required security criteria.
func ValidatePassword(password string) error {
	// Check length: must be between 8 and 20 characters
	if len(password) < 8 || len(password) > 20 {
		err := errors.New("password must be between 8 and 20 characters")
		return err
	}

	// Check for at least one uppercase letter
	if !hasUpperCase(password) {
		err := errors.New("password must contain at least one uppercase letter")
		return err
	}

	// Check for at least one lowercase letter
	if !hasLowerCase(password) {
		err := errors.New("password must contain at least one lowercase letter")
		return err
	}

	// Check for at least one digit
	if !hasDigit(password) {
		err := errors.New("password must contain at least one digit")
		return err
	}

	// Check for at least one special character
	if !hasSpecialCharacter(password) {
		err := errors.New("password must contain at least one special character")
		return err
	}

	// Check for no obvious patterns (e.g., "password", user's email)
	if hasCommonPatterns(password) {
		return errors.New("password contains common patterns or easily guessable words")
	}

	// Check if password contains spaces
	if strings.Contains(password, " ") {
		return errors.New("password must not contain spaces")
	}

	return nil
}

// Helper function to check if password contains at least one uppercase letter
func hasUpperCase(password string) bool {
	for _, char := range password {
		if unicode.IsUpper(char) {
			return true
		}
	}
	return false
}

// Helper function to check if password contains at least one lowercase letter
func hasLowerCase(password string) bool {
	for _, char := range password {
		if unicode.IsLower(char) {
			return true
		}
	}
	return false
}

// Helper function to check if password contains at least one digit
func hasDigit(password string) bool {
	for _, char := range password {
		if unicode.IsDigit(char) {
			return true
		}
	}
	return false
}

// Helper function to check if password contains at least one special character
func hasSpecialCharacter(password string) bool {
	specialChars := `!@#$%^&*()-_=+[\]{}|;:'",<.>/?`
	for _, char := range password {
		if strings.ContainsRune(specialChars, char) {
			return true
		}
	}
	return false
}

// Helper function to check if password contains common patterns (like the word "password")
func hasCommonPatterns(password string) bool {
	// You can customize this to add other common words
	commonPatterns := []string{"password", "123456", "qwerty", "welcome", "admin"}

	for _, pattern := range commonPatterns {
		if strings.Contains(strings.ToLower(password), pattern) {
			return true
		}
	}
	return false
}
