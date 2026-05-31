package models

import (
	"time"
)

type User struct {
	UserID       uint   `gorm:"primaryKey"`
	Username     string `gorm:"uniqueIndex;size:50;not null"`
	PasswordHash string `json:"-"`
	Email        string
	IsAdmin      bool `gorm:"not null;default:false"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}
