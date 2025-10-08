package models

import (
	"time"
)

type User struct {
	UserID       uint   `gorm:"primaryKey"`
	Username     string `gorm:"uniqueIndex;size:50;not null"`
	PasswordHash string
	Email        string

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}
