package models

import "time"

type Unit struct {
	UnitID         uint   `gorm:"primaryKey"`
	NameOriginal   string `gorm:"size:255;not null"`
	NameEnglish    string `gorm:"size:255"`
	PrimaryColor   string `gorm:"size:7"`
	SecondaryColor string `gorm:"size:7"`
	CategoryID     *uint
	Category       *Category `gorm:"foreignKey:CategoryID"`
	Artists        []ArtistUnit `gorm:"foreignKey:UnitID"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}

func (u *Unit) GetNameOriginal() string {
	return u.NameOriginal
}

func (u *Unit) GetNameEnglish() string {
	return u.NameEnglish
}

func (u *Unit) GetPrimaryColor() string {
	return u.PrimaryColor
}

func (u *Unit) GetSecondaryColor() string {
	return u.SecondaryColor
}

func (u *Unit) GetEntityType() string {
	return "unit"
}
