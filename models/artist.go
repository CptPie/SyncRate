package models

import "time"

type Artist struct {
	ArtistID       uint   `gorm:"primaryKey"`
	NameOriginal   string `gorm:"size:255;not null"`
	NameEnglish    string `gorm:"size:255"`
	PrimaryColor   string `gorm:"size:7"`
	SecondaryColor string `gorm:"size:7"`
	CategoryID     *uint
	Category       *Category `gorm:"foreignKey:CategoryID;references:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	Units          []Unit `gorm:"many2many:artist_units;joinForeignKey:ArtistID;joinReferences:UnitID"`
	Songs          []Song `gorm:"many2many:song_artists;joinForeignKey:ArtistID;joinReferences:SongID"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}

func (a *Artist) GetNameOriginal() string {
	return a.NameOriginal
}

func (a *Artist) GetNameEnglish() string {
	return a.NameEnglish
}

func (a *Artist) GetPrimaryColor() string {
	return a.PrimaryColor
}

func (a *Artist) GetSecondaryColor() string {
	return a.SecondaryColor
}

func (a *Artist) GetEntityType() string {
	return "artist"
}
