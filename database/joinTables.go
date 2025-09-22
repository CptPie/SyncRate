package database

import (
	"errors"
	"fmt"

	"github.com/CptPie/SyncRate/models"
)

// SongArtist operations

func (db *Database) validateSongArtist(songArtist *models.SongArtist) error {
	if songArtist == nil {
		return errors.New("song artist cannot be nil")
	}

	if songArtist.SongID == 0 {
		return errors.New("song ID cannot be zero")
	}

	if songArtist.ArtistID == 0 {
		return errors.New("artist ID cannot be zero")
	}

	// Check if song exists
	songExists, err := db.SongExists(songArtist.SongID)
	if err != nil {
		return fmt.Errorf("failed to check if song exists: %w", err)
	}
	if !songExists {
		return errors.New("song does not exist")
	}

	// Check if artist exists
	artistExists, err := db.ArtistExists(songArtist.ArtistID)
	if err != nil {
		return fmt.Errorf("failed to check if artist exists: %w", err)
	}
	if !artistExists {
		return errors.New("artist does not exist")
	}

	return nil
}

func (db *Database) CreateSongArtist(songArtist *models.SongArtist) error {
	if err := db.validateSongArtist(songArtist); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check if relationship already exists
	exists, err := db.SongArtistExists(songArtist.SongID, songArtist.ArtistID)
	if err != nil {
		return fmt.Errorf("failed to check if song artist relationship exists: %w", err)
	}
	if exists {
		return errors.New("song artist relationship already exists")
	}

	if err := db.DB.Create(songArtist).Error; err != nil {
		return fmt.Errorf("failed to create song artist relationship: %w", err)
	}
	return nil
}

func (db *Database) GetSongArtist(songID, artistID uint) (*models.SongArtist, error) {
	if songID == 0 {
		return nil, errors.New("song ID cannot be zero")
	}
	if artistID == 0 {
		return nil, errors.New("artist ID cannot be zero")
	}

	var songArtist models.SongArtist
	if err := db.DB.Preload("Song").Preload("Artist").
		Where("song_id = ? AND artist_id = ?", songID, artistID).First(&songArtist).Error; err != nil {
		return nil, fmt.Errorf("failed to get song artist relationship: %w", err)
	}
	return &songArtist, nil
}

func (db *Database) GetArtistsBySong(songID uint) ([]models.Artist, error) {
	if songID == 0 {
		return nil, errors.New("song ID cannot be zero")
	}

	// Check if song exists
	exists, err := db.SongExists(songID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if song exists: %w", err)
	}
	if !exists {
		return nil, errors.New("song does not exist")
	}

	var artists []models.Artist
	if err := db.DB.Table("artists").
		Joins("JOIN song_artists ON artists.artist_id = song_artists.artist_id").
		Where("song_artists.song_id = ?", songID).
		Find(&artists).Error; err != nil {
		return nil, fmt.Errorf("failed to get artists by song: %w", err)
	}
	return artists, nil
}

func (db *Database) GetSongsByArtist(artistID uint) ([]models.Song, error) {
	if artistID == 0 {
		return nil, errors.New("artist ID cannot be zero")
	}

	// Check if artist exists
	exists, err := db.ArtistExists(artistID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if artist exists: %w", err)
	}
	if !exists {
		return nil, errors.New("artist does not exist")
	}

	var songs []models.Song
	if err := db.DB.Table("songs").
		Joins("JOIN song_artists ON songs.song_id = song_artists.song_id").
		Where("song_artists.artist_id = ?", artistID).
		Find(&songs).Error; err != nil {
		return nil, fmt.Errorf("failed to get songs by artist: %w", err)
	}
	return songs, nil
}

func (db *Database) DeleteSongArtist(songID, artistID uint) error {
	if songID == 0 {
		return errors.New("song ID cannot be zero")
	}
	if artistID == 0 {
		return errors.New("artist ID cannot be zero")
	}

	// Check if relationship exists
	exists, err := db.SongArtistExists(songID, artistID)
	if err != nil {
		return fmt.Errorf("failed to check if song artist relationship exists: %w", err)
	}
	if !exists {
		return errors.New("song artist relationship does not exist")
	}

	if err := db.DB.Where("song_id = ? AND artist_id = ?", songID, artistID).Delete(&models.SongArtist{}).Error; err != nil {
		return fmt.Errorf("failed to delete song artist relationship: %w", err)
	}
	return nil
}

func (db *Database) SongArtistExists(songID, artistID uint) (bool, error) {
	if songID == 0 || artistID == 0 {
		return false, nil
	}

	var count int64
	if err := db.DB.Model(&models.SongArtist{}).
		Where("song_id = ? AND artist_id = ?", songID, artistID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check if song artist relationship exists: %w", err)
	}
	return count > 0, nil
}

// AlbumSong operations

func (db *Database) validateAlbumSong(albumSong *models.AlbumSong) error {
	if albumSong == nil {
		return errors.New("album song cannot be nil")
	}

	if albumSong.AlbumID == 0 {
		return errors.New("album ID cannot be zero")
	}

	if albumSong.SongID == 0 {
		return errors.New("song ID cannot be zero")
	}

	// Check if album exists
	albumExists, err := db.AlbumExists(albumSong.AlbumID)
	if err != nil {
		return fmt.Errorf("failed to check if album exists: %w", err)
	}
	if !albumExists {
		return errors.New("album does not exist")
	}

	// Check if song exists
	songExists, err := db.SongExists(albumSong.SongID)
	if err != nil {
		return fmt.Errorf("failed to check if song exists: %w", err)
	}
	if !songExists {
		return errors.New("song does not exist")
	}

	return nil
}

func (db *Database) CreateAlbumSong(albumSong *models.AlbumSong) error {
	if err := db.validateAlbumSong(albumSong); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check if relationship already exists
	exists, err := db.AlbumSongExists(albumSong.AlbumID, albumSong.SongID)
	if err != nil {
		return fmt.Errorf("failed to check if album song relationship exists: %w", err)
	}
	if exists {
		return errors.New("album song relationship already exists")
	}

	if err := db.DB.Create(albumSong).Error; err != nil {
		return fmt.Errorf("failed to create album song relationship: %w", err)
	}
	return nil
}

func (db *Database) GetAlbumSong(albumID, songID uint) (*models.AlbumSong, error) {
	if albumID == 0 {
		return nil, errors.New("album ID cannot be zero")
	}
	if songID == 0 {
		return nil, errors.New("song ID cannot be zero")
	}

	var albumSong models.AlbumSong
	if err := db.DB.Preload("Album").Preload("Song").
		Where("album_id = ? AND song_id = ?", albumID, songID).First(&albumSong).Error; err != nil {
		return nil, fmt.Errorf("failed to get album song relationship: %w", err)
	}
	return &albumSong, nil
}

func (db *Database) GetSongsByAlbum(albumID uint) ([]models.Song, error) {
	if albumID == 0 {
		return nil, errors.New("album ID cannot be zero")
	}

	// Check if album exists
	exists, err := db.AlbumExists(albumID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if album exists: %w", err)
	}
	if !exists {
		return nil, errors.New("album does not exist")
	}

	var songs []models.Song
	if err := db.DB.Table("songs").
		Joins("JOIN album_songs ON songs.song_id = album_songs.song_id").
		Where("album_songs.album_id = ?", albumID).
		Find(&songs).Error; err != nil {
		return nil, fmt.Errorf("failed to get songs by album: %w", err)
	}
	return songs, nil
}

func (db *Database) GetAlbumsBySong(songID uint) ([]models.Album, error) {
	if songID == 0 {
		return nil, errors.New("song ID cannot be zero")
	}

	// Check if song exists
	exists, err := db.SongExists(songID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if song exists: %w", err)
	}
	if !exists {
		return nil, errors.New("song does not exist")
	}

	var albums []models.Album
	if err := db.DB.Table("albums").
		Joins("JOIN album_songs ON albums.album_id = album_songs.album_id").
		Where("album_songs.song_id = ?", songID).
		Find(&albums).Error; err != nil {
		return nil, fmt.Errorf("failed to get albums by song: %w", err)
	}
	return albums, nil
}

func (db *Database) DeleteAlbumSong(albumID, songID uint) error {
	if albumID == 0 {
		return errors.New("album ID cannot be zero")
	}
	if songID == 0 {
		return errors.New("song ID cannot be zero")
	}

	// Check if relationship exists
	exists, err := db.AlbumSongExists(albumID, songID)
	if err != nil {
		return fmt.Errorf("failed to check if album song relationship exists: %w", err)
	}
	if !exists {
		return errors.New("album song relationship does not exist")
	}

	if err := db.DB.Where("album_id = ? AND song_id = ?", albumID, songID).Delete(&models.AlbumSong{}).Error; err != nil {
		return fmt.Errorf("failed to delete album song relationship: %w", err)
	}
	return nil
}

func (db *Database) AlbumSongExists(albumID, songID uint) (bool, error) {
	if albumID == 0 || songID == 0 {
		return false, nil
	}

	var count int64
	if err := db.DB.Model(&models.AlbumSong{}).
		Where("album_id = ? AND song_id = ?", albumID, songID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check if album song relationship exists: %w", err)
	}
	return count > 0, nil
}

// ArtistUnit operations

func (db *Database) validateArtistUnit(artistUnit *models.ArtistUnit) error {
	if artistUnit == nil {
		return errors.New("artist unit cannot be nil")
	}

	if artistUnit.ArtistID == 0 {
		return errors.New("artist ID cannot be zero")
	}

	if artistUnit.UnitID == 0 {
		return errors.New("unit ID cannot be zero")
	}

	// Check if artist exists
	artistExists, err := db.ArtistExists(artistUnit.ArtistID)
	if err != nil {
		return fmt.Errorf("failed to check if artist exists: %w", err)
	}
	if !artistExists {
		return errors.New("artist does not exist")
	}

	// Check if unit exists
	unitExists, err := db.UnitExists(artistUnit.UnitID)
	if err != nil {
		return fmt.Errorf("failed to check if unit exists: %w", err)
	}
	if !unitExists {
		return errors.New("unit does not exist")
	}

	return nil
}

func (db *Database) CreateArtistUnit(artistUnit *models.ArtistUnit) error {
	if err := db.validateArtistUnit(artistUnit); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check if relationship already exists
	exists, err := db.ArtistUnitExists(artistUnit.ArtistID, artistUnit.UnitID)
	if err != nil {
		return fmt.Errorf("failed to check if artist unit relationship exists: %w", err)
	}
	if exists {
		return errors.New("artist unit relationship already exists")
	}

	if err := db.DB.Create(artistUnit).Error; err != nil {
		return fmt.Errorf("failed to create artist unit relationship: %w", err)
	}
	return nil
}

func (db *Database) GetArtistUnit(artistID, unitID uint) (*models.ArtistUnit, error) {
	if artistID == 0 {
		return nil, errors.New("artist ID cannot be zero")
	}
	if unitID == 0 {
		return nil, errors.New("unit ID cannot be zero")
	}

	var artistUnit models.ArtistUnit
	if err := db.DB.Preload("Artist").Preload("Unit").
		Where("artist_id = ? AND unit_id = ?", artistID, unitID).First(&artistUnit).Error; err != nil {
		return nil, fmt.Errorf("failed to get artist unit relationship: %w", err)
	}
	return &artistUnit, nil
}

func (db *Database) GetArtistsByUnit(unitID uint) ([]models.Artist, error) {
	if unitID == 0 {
		return nil, errors.New("unit ID cannot be zero")
	}

	// Check if unit exists
	exists, err := db.UnitExists(unitID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if unit exists: %w", err)
	}
	if !exists {
		return nil, errors.New("unit does not exist")
	}

	var artists []models.Artist
	if err := db.DB.Table("artists").
		Joins("JOIN artist_units ON artists.artist_id = artist_units.artist_id").
		Where("artist_units.unit_id = ?", unitID).
		Find(&artists).Error; err != nil {
		return nil, fmt.Errorf("failed to get artists by unit: %w", err)
	}
	return artists, nil
}

func (db *Database) GetUnitsByArtist(artistID uint) ([]models.Unit, error) {
	if artistID == 0 {
		return nil, errors.New("artist ID cannot be zero")
	}

	// Check if artist exists
	exists, err := db.ArtistExists(artistID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if artist exists: %w", err)
	}
	if !exists {
		return nil, errors.New("artist does not exist")
	}

	var units []models.Unit
	if err := db.DB.Table("units").
		Joins("JOIN artist_units ON units.unit_id = artist_units.unit_id").
		Where("artist_units.artist_id = ?", artistID).
		Find(&units).Error; err != nil {
		return nil, fmt.Errorf("failed to get units by artist: %w", err)
	}
	return units, nil
}

func (db *Database) DeleteArtistUnit(artistID, unitID uint) error {
	if artistID == 0 {
		return errors.New("artist ID cannot be zero")
	}
	if unitID == 0 {
		return errors.New("unit ID cannot be zero")
	}

	// Check if relationship exists
	exists, err := db.ArtistUnitExists(artistID, unitID)
	if err != nil {
		return fmt.Errorf("failed to check if artist unit relationship exists: %w", err)
	}
	if !exists {
		return errors.New("artist unit relationship does not exist")
	}

	if err := db.DB.Where("artist_id = ? AND unit_id = ?", artistID, unitID).Delete(&models.ArtistUnit{}).Error; err != nil {
		return fmt.Errorf("failed to delete artist unit relationship: %w", err)
	}
	return nil
}

func (db *Database) ArtistUnitExists(artistID, unitID uint) (bool, error) {
	if artistID == 0 || unitID == 0 {
		return false, nil
	}

	var count int64
	if err := db.DB.Model(&models.ArtistUnit{}).
		Where("artist_id = ? AND unit_id = ?", artistID, unitID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check if artist unit relationship exists: %w", err)
	}
	return count > 0, nil
}

// SongUnit operations

func (db *Database) validateSongUnit(songUnit *models.SongUnit) error {
	if songUnit == nil {
		return errors.New("song unit cannot be nil")
	}

	if songUnit.SongID == 0 {
		return errors.New("song ID cannot be zero")
	}

	if songUnit.UnitID == 0 {
		return errors.New("unit ID cannot be zero")
	}

	// Check if song exists
	songExists, err := db.SongExists(songUnit.SongID)
	if err != nil {
		return fmt.Errorf("failed to check if song exists: %w", err)
	}
	if !songExists {
		return errors.New("song does not exist")
	}

	// Check if unit exists
	unitExists, err := db.UnitExists(songUnit.UnitID)
	if err != nil {
		return fmt.Errorf("failed to check if unit exists: %w", err)
	}
	if !unitExists {
		return errors.New("unit does not exist")
	}

	return nil
}

func (db *Database) CreateSongUnit(songUnit *models.SongUnit) error {
	if err := db.validateSongUnit(songUnit); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check if relationship already exists
	exists, err := db.SongUnitExists(songUnit.SongID, songUnit.UnitID)
	if err != nil {
		return fmt.Errorf("failed to check if song unit relationship exists: %w", err)
	}
	if exists {
		return errors.New("song unit relationship already exists")
	}

	if err := db.DB.Create(songUnit).Error; err != nil {
		return fmt.Errorf("failed to create song unit relationship: %w", err)
	}
	return nil
}

func (db *Database) GetSongUnit(songID, unitID uint) (*models.SongUnit, error) {
	if songID == 0 {
		return nil, errors.New("song ID cannot be zero")
	}
	if unitID == 0 {
		return nil, errors.New("unit ID cannot be zero")
	}

	var songUnit models.SongUnit
	if err := db.DB.Preload("Song").Preload("Unit").
		Where("song_id = ? AND unit_id = ?", songID, unitID).First(&songUnit).Error; err != nil {
		return nil, fmt.Errorf("failed to get song unit relationship: %w", err)
	}
	return &songUnit, nil
}

func (db *Database) GetUnitsBySong(songID uint) ([]models.Unit, error) {
	if songID == 0 {
		return nil, errors.New("song ID cannot be zero")
	}

	// Check if song exists
	exists, err := db.SongExists(songID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if song exists: %w", err)
	}
	if !exists {
		return nil, errors.New("song does not exist")
	}

	var units []models.Unit
	if err := db.DB.Table("units").
		Joins("JOIN song_units ON units.unit_id = song_units.unit_id").
		Where("song_units.song_id = ?", songID).
		Find(&units).Error; err != nil {
		return nil, fmt.Errorf("failed to get units by song: %w", err)
	}
	return units, nil
}

func (db *Database) GetSongsByUnit(unitID uint) ([]models.Song, error) {
	if unitID == 0 {
		return nil, errors.New("unit ID cannot be zero")
	}

	// Check if unit exists
	exists, err := db.UnitExists(unitID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if unit exists: %w", err)
	}
	if !exists {
		return nil, errors.New("unit does not exist")
	}

	var songs []models.Song
	if err := db.DB.Table("songs").
		Joins("JOIN song_units ON songs.song_id = song_units.song_id").
		Where("song_units.unit_id = ?", unitID).
		Find(&songs).Error; err != nil {
		return nil, fmt.Errorf("failed to get songs by unit: %w", err)
	}
	return songs, nil
}

func (db *Database) DeleteSongUnit(songID, unitID uint) error {
	if songID == 0 {
		return errors.New("song ID cannot be zero")
	}
	if unitID == 0 {
		return errors.New("unit ID cannot be zero")
	}

	// Check if relationship exists
	exists, err := db.SongUnitExists(songID, unitID)
	if err != nil {
		return fmt.Errorf("failed to check if song unit relationship exists: %w", err)
	}
	if !exists {
		return errors.New("song unit relationship does not exist")
	}

	if err := db.DB.Where("song_id = ? AND unit_id = ?", songID, unitID).Delete(&models.SongUnit{}).Error; err != nil {
		return fmt.Errorf("failed to delete song unit relationship: %w", err)
	}
	return nil
}

func (db *Database) SongUnitExists(songID, unitID uint) (bool, error) {
	if songID == 0 || unitID == 0 {
		return false, nil
	}

	var count int64
	if err := db.DB.Model(&models.SongUnit{}).
		Where("song_id = ? AND unit_id = ?", songID, unitID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check if song unit relationship exists: %w", err)
	}
	return count > 0, nil
}