package models

import "time"

type RatingRoom struct {
	RoomID          string    `gorm:"primaryKey;size:8"`
	CreatorID       uint      `gorm:"not null"`
	CurrentSongID   *uint     `gorm:"index"`
	CategoryID      *uint     `gorm:"index"`
	CoversOnly      bool      `gorm:"default:false"`
	VideoSyncEnabled *bool    `gorm:"default:true"`
	CreatedAt       time.Time
	LastActive      time.Time `gorm:"index"`

	// Relationships
	Creator     User      `gorm:"foreignKey:CreatorID;references:UserID"`
	CurrentSong *Song     `gorm:"foreignKey:CurrentSongID;references:SongID;constraint:OnDelete:SET NULL"`
	Category    *Category `gorm:"foreignKey:CategoryID;references:CategoryID;constraint:OnDelete:SET NULL"`
}