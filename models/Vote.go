package models

import (
	"time"
)

type Vote struct {
	UserID  uint `gorm:"primaryKey"`
	SongID  uint `gorm:"primaryKey"`
	Rating  int  `gorm:"check:rating >= 1 AND rating <= 10"`
	Comment string
	User    User `gorm:"foreignKey:UserID"`
	Song    Song `gorm:"foreignKey:SongID"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}
