package database

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/CptPie/SyncRate/models"
)

func (db *Database) validateAlbum(album *models.Album, isUpdate bool) error {
	if album == nil {
		return errors.New("album cannot be nil")
	}

	// NameOriginal validation
	if strings.TrimSpace(album.NameOriginal) == "" {
		return errors.New("original name cannot be empty")
	}
	if len(album.NameOriginal) > 255 {
		return errors.New("original name cannot exceed 255 characters")
	}

	// NameEnglish validation
	if len(album.NameEnglish) > 255 {
		return errors.New("english name cannot exceed 255 characters")
	}

	// AlbumArtURL validation (if provided)
	if album.AlbumArtURL != "" {
		if _, err := url.ParseRequestURI(album.AlbumArtURL); err != nil {
			return errors.New("album art URL must be a valid URL")
		}
	}

	// Type validation (must be one of the allowed values)
	if album.Type != "" {
		validTypes := map[string]bool{
			"Album":  true,
			"Single": true,
			"EP":     true,
		}
		if !validTypes[album.Type] {
			return errors.New("album type must be one of: Album, Single, EP")
		}
	}

	// CategoryID validation (if provided)
	if album.CategoryID != nil && *album.CategoryID != 0 {
		exists, err := db.CategoryExists(*album.CategoryID)
		if err != nil {
			return fmt.Errorf("failed to check if category exists: %w", err)
		}
		if !exists {
			return errors.New("specified category does not exist")
		}
	}

	return nil
}

func (db *Database) CreateAlbum(album *models.Album) error {
	if err := db.validateAlbum(album, false); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := db.DB.Create(album).Error; err != nil {
		return fmt.Errorf("failed to create album: %w", err)
	}
	return nil
}

func (db *Database) GetAlbumByID(albumID uint) (*models.Album, error) {
	if albumID == 0 {
		return nil, errors.New("album ID cannot be zero")
	}

	var album models.Album
	if err := db.DB.Preload("Category").Preload("Songs").First(&album, albumID).Error; err != nil {
		return nil, fmt.Errorf("failed to get album: %w", err)
	}
	return &album, nil
}

func (db *Database) GetAlbumsByName(name string) ([]models.Album, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("name cannot be empty")
	}

	var albums []models.Album
	searchPattern := "%" + name + "%"
	if err := db.DB.Preload("Category").Preload("Songs").
		Where("name_original ILIKE ? OR name_english ILIKE ?", searchPattern, searchPattern).
		Find(&albums).Error; err != nil {
		return nil, fmt.Errorf("failed to search albums by name: %w", err)
	}
	return albums, nil
}

func (db *Database) GetAlbumsByType(albumType string) ([]models.Album, error) {
	if strings.TrimSpace(albumType) == "" {
		return nil, errors.New("album type cannot be empty")
	}

	// Validate album type
	validTypes := map[string]bool{
		"Album":  true,
		"Single": true,
		"EP":     true,
	}
	if !validTypes[albumType] {
		return nil, errors.New("album type must be one of: Album, Single, EP")
	}

	var albums []models.Album
	if err := db.DB.Preload("Category").Preload("Songs").
		Where("type = ?", albumType).Find(&albums).Error; err != nil {
		return nil, fmt.Errorf("failed to get albums by type: %w", err)
	}
	return albums, nil
}

func (db *Database) GetAlbumsByCategory(categoryID uint) ([]models.Album, error) {
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

	var albums []models.Album
	if err := db.DB.Preload("Category").Preload("Songs").
		Where("category_id = ?", categoryID).Find(&albums).Error; err != nil {
		return nil, fmt.Errorf("failed to get albums by category: %w", err)
	}
	return albums, nil
}

func (db *Database) GetAllAlbums() ([]models.Album, error) {
	var albums []models.Album
	if err := db.DB.Preload("Category").Preload("Songs").Find(&albums).Error; err != nil {
		return nil, fmt.Errorf("failed to get all albums: %w", err)
	}
	return albums, nil
}

func (db *Database) UpdateAlbum(album *models.Album) error {
	if album == nil {
		return errors.New("album cannot be nil")
	}
	if album.AlbumID == 0 {
		return errors.New("album ID cannot be zero")
	}

	// Check if album exists
	exists, err := db.AlbumExists(album.AlbumID)
	if err != nil {
		return fmt.Errorf("failed to check if album exists: %w", err)
	}
	if !exists {
		return errors.New("album does not exist")
	}

	// Validate the updated album data
	if err := db.validateAlbum(album, true); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := db.DB.Save(album).Error; err != nil {
		return fmt.Errorf("failed to update album: %w", err)
	}
	return nil
}

func (db *Database) DeleteAlbum(albumID uint) error {
	if albumID == 0 {
		return errors.New("album ID cannot be zero")
	}

	// Check if album exists
	exists, err := db.AlbumExists(albumID)
	if err != nil {
		return fmt.Errorf("failed to check if album exists: %w", err)
	}
	if !exists {
		return errors.New("album does not exist")
	}

	if err := db.DB.Delete(&models.Album{}, albumID).Error; err != nil {
		return fmt.Errorf("failed to delete album: %w", err)
	}
	return nil
}

func (db *Database) AlbumExists(albumID uint) (bool, error) {
	if albumID == 0 {
		return false, nil
	}

	var count int64
	if err := db.DB.Model(&models.Album{}).Where("album_id = ?", albumID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check if album exists: %w", err)
	}
	return count > 0, nil
}

func (db *Database) SearchAlbums(query string) ([]models.Album, error) {
	if strings.TrimSpace(query) == "" {
		return db.GetAllAlbums()
	}

	var albums []models.Album
	searchPattern := "%" + query + "%"
	if err := db.DB.Preload("Category").Preload("Songs").
		Joins("LEFT JOIN categories ON albums.category_id = categories.category_id").
		Where("name_original ILIKE ? OR name_english ILIKE ? OR categories.name ILIKE ? OR type ILIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern).
		Find(&albums).Error; err != nil {
		return nil, fmt.Errorf("failed to search albums: %w", err)
	}
	return albums, nil
}

func (db *Database) GetAlbumsWithoutArt() ([]models.Album, error) {
	var albums []models.Album
	if err := db.DB.Preload("Category").Preload("Songs").
		Where("album_art_url = '' OR album_art_url IS NULL").
		Find(&albums).Error; err != nil {
		return nil, fmt.Errorf("failed to get albums without art: %w", err)
	}
	return albums, nil
}

func (db *Database) GetAlbumsWithArt() ([]models.Album, error) {
	var albums []models.Album
	if err := db.DB.Preload("Category").Preload("Songs").
		Where("album_art_url != '' AND album_art_url IS NOT NULL").
		Find(&albums).Error; err != nil {
		return nil, fmt.Errorf("failed to get albums with art: %w", err)
	}
	return albums, nil
}