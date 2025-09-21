package database

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/CptPie/SyncRate/models"
)

type ColoredEntity interface {
	GetNameOriginal() string
	GetNameEnglish() string
	GetPrimaryColor() string
	GetSecondaryColor() string
	GetEntityType() string
}

func (db *Database) validateColoredEntity(entity ColoredEntity, entityType string, isUpdate bool) error {
	if entity == nil {
		return fmt.Errorf("%s cannot be nil", entityType)
	}

	// NameOriginal validation
	if strings.TrimSpace(entity.GetNameOriginal()) == "" {
		return errors.New("original name cannot be empty")
	}
	if len(entity.GetNameOriginal()) > 255 {
		return errors.New("original name cannot exceed 255 characters")
	}

	// NameEnglish validation
	if len(entity.GetNameEnglish()) > 255 {
		return errors.New("english name cannot exceed 255 characters")
	}

	// Color validation (should be hex colors if provided)
	if entity.GetPrimaryColor() != "" {
		if err := db.validateHexColor(entity.GetPrimaryColor()); err != nil {
			return fmt.Errorf("invalid primary color: %w", err)
		}
	}

	if entity.GetSecondaryColor() != "" {
		if err := db.validateHexColor(entity.GetSecondaryColor()); err != nil {
			return fmt.Errorf("invalid secondary color: %w", err)
		}
	}

	return nil
}

func (db *Database) validateArtist(artist *models.Artist, isUpdate bool) error {
	if artist == nil {
		return errors.New("artist cannot be nil")
	}

	// CategoryID validation (if provided)
	if artist.CategoryID != nil && *artist.CategoryID != 0 {
		exists, err := db.CategoryExists(*artist.CategoryID)
		if err != nil {
			return fmt.Errorf("failed to check if category exists: %w", err)
		}
		if !exists {
			return errors.New("specified category does not exist")
		}
	}

	return db.validateColoredEntity(artist, "artist", isUpdate)
}

func (db *Database) validateHexColor(color string) error {
	// Hex color validation (should be 7 characters: # + 6 hex digits)
	if len(color) != 7 {
		return errors.New("hex color must be 7 characters long")
	}
	if !strings.HasPrefix(color, "#") {
		return errors.New("hex color must start with #")
	}

	hexPattern := regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	if !hexPattern.MatchString(color) {
		return errors.New("invalid hex color format")
	}

	return nil
}

func (db *Database) CreateArtist(artist *models.Artist) error {
	if err := db.validateArtist(artist, false); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := db.DB.Create(artist).Error; err != nil {
		return fmt.Errorf("failed to create artist: %w", err)
	}
	return nil
}

func (db *Database) GetArtistByID(artistID uint) (*models.Artist, error) {
	if artistID == 0 {
		return nil, errors.New("artist ID cannot be zero")
	}

	var artist models.Artist
	if err := db.DB.Preload("Category").Preload("Units").Preload("Songs").First(&artist, artistID).Error; err != nil {
		return nil, fmt.Errorf("failed to get artist: %w", err)
	}
	return &artist, nil
}

func (db *Database) GetArtistsByName(name string) ([]models.Artist, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("name cannot be empty")
	}

	var artists []models.Artist
	searchPattern := "%" + name + "%"
	if err := db.DB.Preload("Category").Preload("Units").Preload("Songs").
		Where("name_original ILIKE ? OR name_english ILIKE ?", searchPattern, searchPattern).
		Find(&artists).Error; err != nil {
		return nil, fmt.Errorf("failed to search artists by name: %w", err)
	}
	return artists, nil
}

func (db *Database) GetArtistsByCategory(categoryID uint) ([]models.Artist, error) {
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

	var artists []models.Artist
	if err := db.DB.Preload("Category").Preload("Units").Preload("Songs").
		Where("category_id = ?", categoryID).Find(&artists).Error; err != nil {
		return nil, fmt.Errorf("failed to get artists by category: %w", err)
	}
	return artists, nil
}

func (db *Database) GetAllArtists() ([]models.Artist, error) {
	var artists []models.Artist
	if err := db.DB.Preload("Category").Preload("Units").Preload("Songs").Find(&artists).Error; err != nil {
		return nil, fmt.Errorf("failed to get all artists: %w", err)
	}
	return artists, nil
}

func (db *Database) UpdateArtist(artist *models.Artist) error {
	if artist == nil {
		return errors.New("artist cannot be nil")
	}
	if artist.ArtistID == 0 {
		return errors.New("artist ID cannot be zero")
	}

	// Check if artist exists
	exists, err := db.ArtistExists(artist.ArtistID)
	if err != nil {
		return fmt.Errorf("failed to check if artist exists: %w", err)
	}
	if !exists {
		return errors.New("artist does not exist")
	}

	// Validate the updated artist data
	if err := db.validateArtist(artist, true); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := db.DB.Save(artist).Error; err != nil {
		return fmt.Errorf("failed to update artist: %w", err)
	}
	return nil
}

func (db *Database) DeleteArtist(artistID uint) error {
	if artistID == 0 {
		return errors.New("artist ID cannot be zero")
	}

	// Check if artist exists
	exists, err := db.ArtistExists(artistID)
	if err != nil {
		return fmt.Errorf("failed to check if artist exists: %w", err)
	}
	if !exists {
		return errors.New("artist does not exist")
	}

	if err := db.DB.Delete(&models.Artist{}, artistID).Error; err != nil {
		return fmt.Errorf("failed to delete artist: %w", err)
	}
	return nil
}

func (db *Database) ArtistExists(artistID uint) (bool, error) {
	if artistID == 0 {
		return false, nil
	}

	var count int64
	if err := db.DB.Model(&models.Artist{}).Where("artist_id = ?", artistID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check if artist exists: %w", err)
	}
	return count > 0, nil
}

func (db *Database) GetArtistsByColor(color string) ([]models.Artist, error) {
	if strings.TrimSpace(color) == "" {
		return nil, errors.New("color cannot be empty")
	}

	if err := db.validateHexColor(color); err != nil {
		return nil, fmt.Errorf("invalid color format: %w", err)
	}

	var artists []models.Artist
	if err := db.DB.Preload("Category").Preload("Units").Preload("Songs").
		Where("primary_color = ? OR secondary_color = ?", color, color).
		Find(&artists).Error; err != nil {
		return nil, fmt.Errorf("failed to get artists by color: %w", err)
	}
	return artists, nil
}

func (db *Database) SearchArtists(query string) ([]models.Artist, error) {
	if strings.TrimSpace(query) == "" {
		return db.GetAllArtists()
	}

	var artists []models.Artist
	searchPattern := "%" + query + "%"
	if err := db.DB.Preload("Category").Preload("Units").Preload("Songs").
		Joins("LEFT JOIN categories ON artists.category_id = categories.category_id").
		Where("name_original ILIKE ? OR name_english ILIKE ? OR categories.name ILIKE ?",
			searchPattern, searchPattern, searchPattern).
		Find(&artists).Error; err != nil {
		return nil, fmt.Errorf("failed to search artists: %w", err)
	}
	return artists, nil
}