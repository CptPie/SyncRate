package models

type SongArtist struct {
	SongID   uint   `gorm:"primaryKey"`
	ArtistID uint   `gorm:"primaryKey"`
	Song     Song   `gorm:"foreignKey:SongID"`
	Artist   Artist `gorm:"foreignKey:ArtistID"`
}

type AlbumSong struct {
	AlbumID uint  `gorm:"primaryKey"`
	SongID  uint  `gorm:"primaryKey"`
	Album   Album `gorm:"foreignKey:AlbumID"`
	Song    Song  `gorm:"foreignKey:SongID"`
}

type SongUnit struct {
	SongID uint `gorm:"primaryKey"`
	UnitID uint `gorm:"primaryKey"`
	Song   Song `gorm:"foreignKey:SongID"`
	Unit   Unit `gorm:"foreignKey:UnitID"`
}

type ArtistUnit struct {
	ArtistID uint   `gorm:"primaryKey"`
	UnitID   uint   `gorm:"primaryKey"`
	Artist   Artist `gorm:"foreignKey:ArtistID"`
	Unit     Unit   `gorm:"foreignKey:UnitID"`
}
