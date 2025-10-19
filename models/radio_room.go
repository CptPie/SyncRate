package models

import "time"

type RadioRoom struct {
	RoomID         string    `gorm:"primaryKey;size:8"`
	CreatorID      uint      `gorm:"not null"`
	CurrentSongID  *uint     `gorm:"index"`
	CategoryID     *uint     `gorm:"index"`
	IncludeCovers  bool      `gorm:"default:false"`
	MinRating      *int      `gorm:"default:null"` // Null means no rating filter
	CreatedAt      time.Time
	LastActive     time.Time `gorm:"index"`

	// Relationships
	Creator     User      `gorm:"foreignKey:CreatorID;references:UserID"`
	CurrentSong *Song     `gorm:"foreignKey:CurrentSongID;references:SongID;constraint:OnDelete:SET NULL"`
	Category    *Category `gorm:"foreignKey:CategoryID;references:CategoryID;constraint:OnDelete:SET NULL"`
}
