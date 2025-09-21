package models

import "time"

type Category struct {
	CategoryID uint   `gorm:"primaryKey"`
	Name       string `gorm:"size:100;not null;uniqueIndex"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}