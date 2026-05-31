package models

import (
	"time"
)

// PasswordResetToken backs the admin-generated reset-link flow. The raw token
// is handed to the user via the link; only its SHA-256 hash is stored, so a
// database leak doesn't expose usable tokens. Tokens are single-use (UsedAt)
// and time-limited (ExpiresAt).
type PasswordResetToken struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"not null;index"`
	TokenHash string    `gorm:"not null;uniqueIndex;size:64"`
	ExpiresAt time.Time `gorm:"not null"`
	UsedAt    *time.Time

	CreatedAt time.Time
}
