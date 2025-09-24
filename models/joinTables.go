package models

type SongArtist struct {
	SongID   uint `gorm:"primaryKey"`
	ArtistID uint `gorm:"primaryKey"`
}

type AlbumSong struct {
	AlbumID uint `gorm:"primaryKey"`
	SongID  uint `gorm:"primaryKey"`
}

type SongUnit struct {
	SongID uint `gorm:"primaryKey"`
	UnitID uint `gorm:"primaryKey"`
}

type ArtistUnit struct {
	ArtistID uint `gorm:"primaryKey"`
	UnitID   uint `gorm:"primaryKey"`
}
