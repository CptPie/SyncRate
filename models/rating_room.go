package models

import "time"

type RatingRoom struct {
	RoomID       string    `gorm:"primaryKey;size:8"`
	CreatorID    uint      `gorm:"not null"`
	CurrentSongID *uint    `gorm:"index"`
	CreatedAt    time.Time
	LastActive   time.Time `gorm:"index"`

	// Relationships
	Creator     User  `gorm:"foreignKey:CreatorID;references:UserID"`
	CurrentSong *Song `gorm:"foreignKey:CurrentSongID;references:SongID;constraint:OnDelete:SET NULL"`
}