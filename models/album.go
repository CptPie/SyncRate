package models

import "time"

type Album struct {
	AlbumID      uint   `gorm:"primaryKey"`
	NameOriginal string `gorm:"size:255;not null"`
	NameEnglish  string `gorm:"size:255"`
	AlbumArtURL  string
	Type         string `gorm:"size:20;check:type IN ('Album','Single','EP')"`
	CategoryID   *uint
	Category     *Category `gorm:"foreignKey:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	Songs        []AlbumSong `gorm:"foreignKey:AlbumID"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}
