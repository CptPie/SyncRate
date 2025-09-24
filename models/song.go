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
	Units        []Unit   `gorm:"many2many:song_units;joinForeignKey:SongID;joinReferences:UnitID"`
	Artists      []Artist `gorm:"many2many:song_artists;joinForeignKey:SongID;joinReferences:ArtistID"`
	Albums       []Album  `gorm:"many2many:album_songs;joinForeignKey:SongID;joinReferences:AlbumID"`
	Votes        []Vote       `gorm:"foreignKey:SongID;references:SongID"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}
