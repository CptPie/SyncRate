package database

import (
	"errors"
	"fmt"
	"strings"

	"github.com/CptPie/SyncRate/models"
)

func (db *Database) validateUnit(unit *models.Unit, isUpdate bool) error {
	if unit == nil {
		return errors.New("unit cannot be nil")
	}
	return db.validateColoredEntity(unit, "unit", isUpdate)
}

func (db *Database) CreateUnit(unit *models.Unit) error {
	if err := db.validateUnit(unit, false); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := db.DB.Create(unit).Error; err != nil {
		return fmt.Errorf("failed to create unit: %w", err)
	}
	return nil
}

func (db *Database) GetUnitByID(unitID uint) (*models.Unit, error) {
	if unitID == 0 {
		return nil, errors.New("unit ID cannot be zero")
	}

	var unit models.Unit
	if err := db.DB.Preload("Artists").First(&unit, unitID).Error; err != nil {
		return nil, fmt.Errorf("failed to get unit: %w", err)
	}
	return &unit, nil
}

func (db *Database) GetUnitsByName(name string) ([]models.Unit, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("name cannot be empty")
	}

	var units []models.Unit
	searchPattern := "%" + name + "%"
	if err := db.DB.Preload("Artists").
		Where("name_original ILIKE ? OR name_english ILIKE ?", searchPattern, searchPattern).
		Find(&units).Error; err != nil {
		return nil, fmt.Errorf("failed to search units by name: %w", err)
	}
	return units, nil
}

func (db *Database) GetUnitsByCategory(category string) ([]models.Unit, error) {
	if strings.TrimSpace(category) == "" {
		return nil, errors.New("category cannot be empty")
	}

	var units []models.Unit
	if err := db.DB.Preload("Artists").
		Where("category = ?", category).Find(&units).Error; err != nil {
		return nil, fmt.Errorf("failed to get units by category: %w", err)
	}
	return units, nil
}

func (db *Database) GetAllUnits() ([]models.Unit, error) {
	var units []models.Unit
	if err := db.DB.Preload("Artists").Find(&units).Error; err != nil {
		return nil, fmt.Errorf("failed to get all units: %w", err)
	}
	return units, nil
}

func (db *Database) UpdateUnit(unit *models.Unit) error {
	if unit == nil {
		return errors.New("unit cannot be nil")
	}
	if unit.UnitID == 0 {
		return errors.New("unit ID cannot be zero")
	}

	// Check if unit exists
	exists, err := db.UnitExists(unit.UnitID)
	if err != nil {
		return fmt.Errorf("failed to check if unit exists: %w", err)
	}
	if !exists {
		return errors.New("unit does not exist")
	}

	// Validate the updated unit data
	if err := db.validateUnit(unit, true); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := db.DB.Save(unit).Error; err != nil {
		return fmt.Errorf("failed to update unit: %w", err)
	}
	return nil
}

func (db *Database) DeleteUnit(unitID uint) error {
	if unitID == 0 {
		return errors.New("unit ID cannot be zero")
	}

	// Check if unit exists
	exists, err := db.UnitExists(unitID)
	if err != nil {
		return fmt.Errorf("failed to check if unit exists: %w", err)
	}
	if !exists {
		return errors.New("unit does not exist")
	}

	// Check if unit has songs associated with it
	var songCount int64
	if err := db.DB.Model(&models.Song{}).Where("unit_id = ?", unitID).Count(&songCount).Error; err != nil {
		return fmt.Errorf("failed to check unit song relationships: %w", err)
	}
	if songCount > 0 {
		return fmt.Errorf("cannot delete unit: %d songs are still associated with this unit", songCount)
	}

	if err := db.DB.Delete(&models.Unit{}, unitID).Error; err != nil {
		return fmt.Errorf("failed to delete unit: %w", err)
	}
	return nil
}

func (db *Database) UnitExists(unitID uint) (bool, error) {
	if unitID == 0 {
		return false, nil
	}

	var count int64
	if err := db.DB.Model(&models.Unit{}).Where("unit_id = ?", unitID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check if unit exists: %w", err)
	}
	return count > 0, nil
}

func (db *Database) GetUnitsByColor(color string) ([]models.Unit, error) {
	if strings.TrimSpace(color) == "" {
		return nil, errors.New("color cannot be empty")
	}

	if err := db.validateHexColor(color); err != nil {
		return nil, fmt.Errorf("invalid color format: %w", err)
	}

	var units []models.Unit
	if err := db.DB.Preload("Artists").
		Where("primary_color = ? OR secondary_color = ?", color, color).
		Find(&units).Error; err != nil {
		return nil, fmt.Errorf("failed to get units by color: %w", err)
	}
	return units, nil
}

func (db *Database) SearchUnits(query string) ([]models.Unit, error) {
	if strings.TrimSpace(query) == "" {
		return db.GetAllUnits()
	}

	var units []models.Unit
	searchPattern := "%" + query + "%"
	if err := db.DB.Preload("Artists").
		Where("name_original ILIKE ? OR name_english ILIKE ? OR category ILIKE ?",
			searchPattern, searchPattern, searchPattern).
		Find(&units).Error; err != nil {
		return nil, fmt.Errorf("failed to search units: %w", err)
	}
	return units, nil
}

func (db *Database) GetUnitsWithSongs() ([]models.Unit, error) {
	var units []models.Unit
	if err := db.DB.Preload("Artists").
		Joins("JOIN songs ON units.unit_id = songs.unit_id").
		Distinct().Find(&units).Error; err != nil {
		return nil, fmt.Errorf("failed to get units with songs: %w", err)
	}
	return units, nil
}

func (db *Database) GetUnitsWithoutSongs() ([]models.Unit, error) {
	var units []models.Unit
	if err := db.DB.Preload("Artists").
		Where("unit_id NOT IN (SELECT DISTINCT unit_id FROM songs WHERE unit_id IS NOT NULL)").
		Find(&units).Error; err != nil {
		return nil, fmt.Errorf("failed to get units without songs: %w", err)
	}
	return units, nil
}

func (db *Database) GetSongCountForUnit(unitID uint) (int64, error) {
	if unitID == 0 {
		return 0, errors.New("unit ID cannot be zero")
	}

	// Check if unit exists
	exists, err := db.UnitExists(unitID)
	if err != nil {
		return 0, fmt.Errorf("failed to check if unit exists: %w", err)
	}
	if !exists {
		return 0, errors.New("unit does not exist")
	}

	var count int64
	if err := db.DB.Model(&models.Song{}).Where("unit_id = ?", unitID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count songs for unit: %w", err)
	}
	return count, nil
}