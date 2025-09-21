package models

import "time"

type Song struct {
	SongID       uint   `gorm:"primaryKey"`
	NameOriginal string `gorm:"size:255;not null"`
	NameEnglish  string `gorm:"size:255"`
	SourceURL    string `gorm:"not null"`
	ThumbnailURL string `gorm:"not null"`
	Category     string
	IsCover      bool
	UnitID       *uint
	Unit         *Unit        `gorm:"foreignKey:UnitID"`
	Artists      []SongArtist `gorm:"foreignKey:SongID"`
	Albums       []AlbumSong  `gorm:"foreignKey:SongID"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}
