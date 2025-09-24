package models

type SongArtist struct {
	SongID   uint   `gorm:"primaryKey"`
	ArtistID uint   `gorm:"primaryKey"`
	Song     Song   `gorm:"foreignKey:SongID;references:SongID"`
	Artist   Artist `gorm:"foreignKey:ArtistID;references:ArtistID"`
}

type AlbumSong struct {
	AlbumID uint  `gorm:"primaryKey"`
	SongID  uint  `gorm:"primaryKey"`
	Album   Album `gorm:"foreignKey:AlbumID;references:AlbumID"`
	Song    Song  `gorm:"foreignKey:SongID;references:SongID"`
}

type SongUnit struct {
	SongID uint `gorm:"primaryKey"`
	UnitID uint `gorm:"primaryKey"`
	Song   Song `gorm:"foreignKey:SongID;references:SongID"`
	Unit   Unit `gorm:"foreignKey:UnitID;references:UnitID"`
}

type ArtistUnit struct {
	ArtistID uint   `gorm:"primaryKey"`
	UnitID   uint   `gorm:"primaryKey"`
	Artist   Artist `gorm:"foreignKey:ArtistID;references:ArtistID"`
	Unit     Unit   `gorm:"foreignKey:UnitID;references:UnitID"`
}
