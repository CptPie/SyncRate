package models

import (
	"time"
)

type Vote struct {
	VoteID  uint `gorm:"primaryKey"`
	UserID  uint `gorm:"uniqueIndex:idx_user_song,priority:1"`
	SongID  uint `gorm:"uniqueIndex:idx_user_song,priority:2"`
	Rating  int  `gorm:"check:rating >= 1 AND rating <= 10"`
	Comment string

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}
