package whatsapp

import (
	"log/slog"
	"time"

	"github.com/pmitra96/pateproject/models"
)

// Session encapsulates all context for a single WhatsApp interaction
type Session struct {
	User      *models.User
	MessageID string
	Logger    *slog.Logger
	StartTime time.Time
	Client    WhatsAppClient
}

// NewSession creates a new session with an initialized logger
func NewSession(user *models.User, msgID string, logger *slog.Logger) *Session {
	return &Session{
		User:      user,
		MessageID: msgID,
		Logger:    logger,
		StartTime: time.Now(),
		Client:    NewClient(),
	}
}

// Reply sends a text response back to the user in this session
func (s *Session) Reply(text string) {
	err := s.Client.SendTextMessage(s.User.Identities[0].ExternalID, text)
	if err != nil {
		s.Logger.Error("Failed to send reply", "error", err)
	}
}
