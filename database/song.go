package database

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/CptPie/SyncRate/models"
)

func (db *Database) validateSong(song *models.Song, isUpdate bool) error {
	if song == nil {
		return errors.New("song cannot be nil")
	}

	// NameOriginal validation
	if strings.TrimSpace(song.NameOriginal) == "" {
		return errors.New("original name cannot be empty")
	}
	if len(song.NameOriginal) > 255 {
		return errors.New("original name cannot exceed 255 characters")
	}

	// NameEnglish validation
	if len(song.NameEnglish) > 255 {
		return errors.New("english name cannot exceed 255 characters")
	}

	// SourceURL validation
	if strings.TrimSpace(song.SourceURL) == "" {
		return errors.New("source URL cannot be empty")
	}
	if _, err := url.ParseRequestURI(song.SourceURL); err != nil {
		return errors.New("source URL must be a valid URL")
	}

	// ThumbnailURL validation
	if strings.TrimSpace(song.ThumbnailURL) == "" {
		return errors.New("thumbnail URL cannot be empty")
	}
	if _, err := url.ParseRequestURI(song.ThumbnailURL); err != nil {
		return errors.New("thumbnail URL must be a valid URL")
	}

	// CategoryID validation (if provided)
	if song.CategoryID != nil && *song.CategoryID != 0 {
		exists, err := db.CategoryExists(*song.CategoryID)
		if err != nil {
			return fmt.Errorf("failed to check if category exists: %w", err)
		}
		if !exists {
			return errors.New("specified category does not exist")
		}
	}

	return nil
}

func (db *Database) CreateSong(song *models.Song) error {
	if err := db.validateSong(song, false); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := db.DB.Create(song).Error; err != nil {
		return fmt.Errorf("failed to create song: %w", err)
	}
	return nil
}

func (db *Database) GetSongByID(songID uint) (*models.Song, error) {
	if songID == 0 {
		return nil, errors.New("song ID cannot be zero")
	}

	var song models.Song
	if err := db.DB.Preload("Units").Preload("Category").Preload("Artists").Preload("Albums").First(&song, songID).Error; err != nil {
		return nil, fmt.Errorf("failed to get song: %w", err)
	}
	return &song, nil
}

func (db *Database) GetSongsByName(name string) ([]models.Song, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("name cannot be empty")
	}

	var songs []models.Song
	searchPattern := "%" + name + "%"
	if err := db.DB.Preload("Units").Preload("Category").Preload("Artists").Preload("Albums").
		Where("name_original ILIKE ? OR name_english ILIKE ?", searchPattern, searchPattern).
		Find(&songs).Error; err != nil {
		return nil, fmt.Errorf("failed to search songs by name: %w", err)
	}
	return songs, nil
}

func (db *Database) GetSongsByCategory(categoryID uint) ([]models.Song, error) {
	if categoryID == 0 {
		return nil, errors.New("category ID cannot be zero")
	}

	// Verify category exists
	exists, err := db.CategoryExists(categoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if category exists: %w", err)
	}
	if !exists {
		return nil, errors.New("category does not exist")
	}

	var songs []models.Song
	if err := db.DB.Preload("Units").Preload("Category").Preload("Artists").Preload("Albums").
		Where("category_id = ?", categoryID).Find(&songs).Error; err != nil {
		return nil, fmt.Errorf("failed to get songs by category: %w", err)
	}
	return songs, nil
}


func (db *Database) GetCoverSongs() ([]models.Song, error) {
	var songs []models.Song
	if err := db.DB.Preload("Units").Preload("Category").Preload("Artists").Preload("Albums").
		Where("is_cover = ?", true).Find(&songs).Error; err != nil {
		return nil, fmt.Errorf("failed to get cover songs: %w", err)
	}
	return songs, nil
}

func (db *Database) GetOriginalSongs() ([]models.Song, error) {
	var songs []models.Song
	if err := db.DB.Preload("Units").Preload("Category").Preload("Artists").Preload("Albums").
		Where("is_cover = ?", false).Find(&songs).Error; err != nil {
		return nil, fmt.Errorf("failed to get original songs: %w", err)
	}
	return songs, nil
}

func (db *Database) GetAllSongs() ([]models.Song, error) {
	var songs []models.Song
	if err := db.DB.Preload("Units").Preload("Category").Preload("Artists").Preload("Albums").Find(&songs).Error; err != nil {
		return nil, fmt.Errorf("failed to get all songs: %w", err)
	}
	return songs, nil
}

func (db *Database) UpdateSong(song *models.Song) error {
	if song == nil {
		return errors.New("song cannot be nil")
	}
	if song.SongID == 0 {
		return errors.New("song ID cannot be zero")
	}

	// Check if song exists
	exists, err := db.SongExists(song.SongID)
	if err != nil {
		return fmt.Errorf("failed to check if song exists: %w", err)
	}
	if !exists {
		return errors.New("song does not exist")
	}

	// Validate the updated song data
	if err := db.validateSong(song, true); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := db.DB.Save(song).Error; err != nil {
		return fmt.Errorf("failed to update song: %w", err)
	}
	return nil
}

func (db *Database) DeleteSong(songID uint) error {
	if songID == 0 {
		return errors.New("song ID cannot be zero")
	}

	// Check if song exists
	exists, err := db.SongExists(songID)
	if err != nil {
		return fmt.Errorf("failed to check if song exists: %w", err)
	}
	if !exists {
		return errors.New("song does not exist")
	}

	if err := db.DB.Delete(&models.Song{}, songID).Error; err != nil {
		return fmt.Errorf("failed to delete song: %w", err)
	}
	return nil
}

func (db *Database) SongExists(songID uint) (bool, error) {
	if songID == 0 {
		return false, nil
	}

	var count int64
	if err := db.DB.Model(&models.Song{}).Where("song_id = ?", songID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check if song exists: %w", err)
	}
	return count > 0, nil
}

func (db *Database) GetSongsBySourceURL(sourceURL string) (*models.Song, error) {
	if strings.TrimSpace(sourceURL) == "" {
		return nil, errors.New("source URL cannot be empty")
	}

	var song models.Song
	if err := db.DB.Preload("Units").Preload("Category").Preload("Artists").Preload("Albums").
		Where("source_url = ?", sourceURL).First(&song).Error; err != nil {
		return nil, fmt.Errorf("failed to get song by source URL: %w", err)
	}
	return &song, nil
}