package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/joho/godotenv"
	"github.com/pmitra96/pateproject/config"
)

func main() {
	godotenv.Load("../.env")

	accessToken := config.GetEnv("WHATSAPP_ACCESS_TOKEN", "")
	phoneNumberID := config.GetEnv("WHATSAPP_PHONE_NUMBER_ID", "")
	testRecipient := "919632810011" // Assuming this is the user's number based on context or common format

	fmt.Printf("Testing WhatsApp Send Trip...\n")
	fmt.Printf("Phone Number ID: %s\n", phoneNumberID)
	fmt.Printf("URL: https://graph.facebook.com/v18.0/%s/messages\n", phoneNumberID)

	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/messages", phoneNumberID)

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                testRecipient,
		"type":              "text",
		"text": map[string]string{
			"body": "Test message from PateProject Backend Debugger",
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Network Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status Code: %d\n", resp.StatusCode)
	fmt.Printf("Response: %s\n", string(respBody))
}
