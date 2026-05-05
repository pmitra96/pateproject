package controllers

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pmitra96/pateproject/config"
	"github.com/pmitra96/pateproject/database"
	"github.com/pmitra96/pateproject/models"
)

type OTPRequest struct {
	Phone string `json:"phone"`
}

type OTPVerifyRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

// RequestWhatsAppOTP generates an OTP and sends it via WhatsApp
func RequestWhatsAppOTP(w http.ResponseWriter, r *http.Request) {
	var req OTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "Invalid request body"})
		return
	}

	if req.Phone == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "Phone number is required"})
		return
	}

	// Generate 6-digit OTP
	n, _ := rand.Int(rand.Reader, big.NewInt(900000))
	code := fmt.Sprintf("%06d", n.Int64()+100000)

	// Save to DB
	otp := models.OTP{
		Phone:     req.Phone,
		Code:      code,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	database.DB.Create(&otp)

	// Send via WhatsApp
	msg := fmt.Sprintf("Your PateProject login code is: *%s*\n\nThis code expires in 10 minutes.", code)
	err := SendWhatsAppMessage(req.Phone, msg)
	if err != nil {
		fmt.Printf("Failed to send OTP to %s: %v\n", req.Phone, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Failed to send OTP message. Is your number registered?"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "OTP sent successfully",
	})
}

// VerifyWhatsAppOTP verifies the OTP and returns a JWT
func VerifyWhatsAppOTP(w http.ResponseWriter, r *http.Request) {
	var req OTPVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "Invalid request body"})
		return
	}

	if req.Phone == "" || req.Code == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "Phone and code are required"})
		return
	}

	// Verify OTP
	var otp models.OTP
	err := database.DB.Where("phone = ? AND code = ? AND expires_at > ?", req.Phone, req.Code, time.Now()).Order("created_at desc").First(&otp).Error
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"message": "Invalid or expired OTP"})
		return
	}

	// Delete used OTP
	database.DB.Delete(&otp)

	// Get or Create User
	user, err := GetOrCreateWhatsAppUser(req.Phone)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Failed to retrieve user account"})
		return
	}

	// Generate JWT
	secret := config.GetEnv("JWT_SECRET", "default-dev-secret-key-12345")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   req.Phone, // Uses the phone as the subject
		"name":  user.Name,
		"exp":   time.Now().Add(24 * 7 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Failed to generate token"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"token":  tokenString,
		"user": map[string]interface{}{
			"id":    user.ID,
			"name":  user.Name,
			"phone": req.Phone,
		},
	})
}
