package models

import "time"

type Song struct {
	SongID       uint   `gorm:"primaryKey"`
	NameOriginal string `gorm:"size:255;not null"`
	NameEnglish  string `gorm:"size:255"`
	SourceURL    string `gorm:"not null"`
	ThumbnailURL string `gorm:"not null"`
	CategoryID   *uint
	Category     *Category    `gorm:"foreignKey:CategoryID;references:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	IsCover      bool
	Units        []SongUnit   `gorm:"foreignKey:SongID"`
	Artists      []SongArtist `gorm:"foreignKey:SongID"`
	Albums       []AlbumSong  `gorm:"foreignKey:SongID"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}
