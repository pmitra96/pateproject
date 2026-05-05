package whatsapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pmitra96/pateproject/config"
	"github.com/pmitra96/pateproject/logger"
)

var sharedHTTPClient = &http.Client{
	Timeout: 45 * time.Second,
}

// WhatsAppClient defines the operations for interacting with Meta's API
type WhatsAppClient interface {
	SendTextMessage(to string, text string) error
	MarkAsRead(msgID string) error
	DownloadMedia(mediaID string) ([]byte, error)
}

// Client handles direct communication with the Meta WhatsApp Graph API
type Client struct {
	AccessToken   string
	PhoneNumberID string
	BaseURL       string
}

func NewClient() *Client {
	return &Client{
		AccessToken:   config.GetEnv("WHATSAPP_ACCESS_TOKEN", ""),
		PhoneNumberID: config.GetEnv("WHATSAPP_PHONE_NUMBER_ID", ""),
		BaseURL:       "https://graph.facebook.com/v18.0",
	}
}

// SendTextMessage sends a plain text message to a user
func (c *Client) SendTextMessage(to string, text string) error {
	url := fmt.Sprintf("%s/%s/messages", c.BaseURL, c.PhoneNumberID)
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              "text",
		"text": map[string]string{
			"preview_url": "false",
			"body":        text,
		},
	}

	return c.post(url, payload)
}

// MarkAsRead sends a read receipt for a specific message
func (c *Client) MarkAsRead(msgID string) error {
	url := fmt.Sprintf("%s/%s/messages", c.BaseURL, c.PhoneNumberID)
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"status":            "read",
		"message_id":        msgID,
	}

	return c.post(url, payload)
}

// DownloadMedia fetches the raw bytes of a media item from Meta
func (c *Client) DownloadMedia(mediaID string) ([]byte, error) {
	// 1. Get media URL
	url := fmt.Sprintf("%s/%s", c.BaseURL, mediaID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get media metadata (status %d): %s", resp.StatusCode, string(body))
	}

	var meta struct {
		URL string `json:"url"`
	}
	json.NewDecoder(resp.Body).Decode(&meta)

	// 2. Download the actual media
	req, _ = http.NewRequest("GET", meta.URL, nil)
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	resp, err = sharedHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to download media bits (status %d)", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (c *Client) post(url string, payload interface{}) error {
	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		logger.Error("WhatsApp API Error", "status", resp.StatusCode, "body", string(respBody))
		return fmt.Errorf("whatsapp api error: %s", string(respBody))
	}

	return nil
}

// MockClient for testing
type MockClient struct {
	LastMessage string
	LastRecipient string
}

func (m *MockClient) SendTextMessage(to string, text string) error {
	m.LastRecipient = to
	m.LastMessage = text
	return nil
}

func (m *MockClient) MarkAsRead(msgID string) error { return nil }
func (m *MockClient) DownloadMedia(mediaID string) ([]byte, error) { return nil, nil }
