package database

import (
	"errors"
	"fmt"
	"strings"

	"github.com/CptPie/SyncRate/models"
)

func (db *Database) validateCategory(category *models.Category, isUpdate bool) error {
	if category == nil {
		return errors.New("category cannot be nil")
	}

	// Name validation
	if strings.TrimSpace(category.Name) == "" {
		return errors.New("category name cannot be empty")
	}
	if len(category.Name) > 100 {
		return errors.New("category name cannot exceed 100 characters")
	}

	// Check name uniqueness (skip if updating and name hasn't changed)
	if !isUpdate {
		exists, err := db.CategoryNameExists(category.Name)
		if err != nil {
			return fmt.Errorf("failed to check category name uniqueness: %w", err)
		}
		if exists {
			return errors.New("category name already exists")
		}
	}

	return nil
}

func (db *Database) CreateCategory(category *models.Category) error {
	if err := db.validateCategory(category, false); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := db.DB.Create(category).Error; err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}
	return nil
}

func (db *Database) GetCategoryByID(categoryID uint) (*models.Category, error) {
	if categoryID == 0 {
		return nil, errors.New("category ID cannot be zero")
	}

	var category models.Category
	if err := db.DB.First(&category, categoryID).Error; err != nil {
		return nil, fmt.Errorf("failed to get category: %w", err)
	}
	return &category, nil
}

func (db *Database) GetCategoryByName(name string) (*models.Category, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("category name cannot be empty")
	}

	var category models.Category
	if err := db.DB.Where("name = ?", name).First(&category).Error; err != nil {
		return nil, fmt.Errorf("failed to get category by name: %w", err)
	}
	return &category, nil
}

func (db *Database) GetAllCategories() ([]models.Category, error) {
	var categories []models.Category
	if err := db.DB.Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("failed to get all categories: %w", err)
	}
	return categories, nil
}

func (db *Database) UpdateCategory(category *models.Category) error {
	if category == nil {
		return errors.New("category cannot be nil")
	}
	if category.CategoryID == 0 {
		return errors.New("category ID cannot be zero")
	}

	// Check if category exists
	exists, err := db.CategoryExists(category.CategoryID)
	if err != nil {
		return fmt.Errorf("failed to check if category exists: %w", err)
	}
	if !exists {
		return errors.New("category does not exist")
	}

	// For updates, we need to check name uniqueness more carefully
	// Get the current category to compare values
	currentCategory, err := db.GetCategoryByID(category.CategoryID)
	if err != nil {
		return fmt.Errorf("failed to get current category: %w", err)
	}

	// Check name uniqueness only if it's being changed
	if category.Name != currentCategory.Name {
		exists, err := db.CategoryNameExists(category.Name)
		if err != nil {
			return fmt.Errorf("failed to check category name uniqueness: %w", err)
		}
		if exists {
			return errors.New("category name already exists")
		}
	}

	// Validate the updated category data
	if err := db.validateCategory(category, true); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := db.DB.Save(category).Error; err != nil {
		return fmt.Errorf("failed to update category: %w", err)
	}
	return nil
}

func (db *Database) DeleteCategory(categoryID uint) error {
	if categoryID == 0 {
		return errors.New("category ID cannot be zero")
	}

	// Check if category exists
	exists, err := db.CategoryExists(categoryID)
	if err != nil {
		return fmt.Errorf("failed to check if category exists: %w", err)
	}
	if !exists {
		return errors.New("category does not exist")
	}

	// Check if category is being used by any entities
	if err := db.checkCategoryUsage(categoryID); err != nil {
		return fmt.Errorf("cannot delete category: %w", err)
	}

	if err := db.DB.Delete(&models.Category{}, categoryID).Error; err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}
	return nil
}

func (db *Database) CategoryExists(categoryID uint) (bool, error) {
	if categoryID == 0 {
		return false, nil
	}

	var count int64
	if err := db.DB.Model(&models.Category{}).Where("category_id = ?", categoryID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check if category exists: %w", err)
	}
	return count > 0, nil
}

func (db *Database) CategoryNameExists(name string) (bool, error) {
	if strings.TrimSpace(name) == "" {
		return false, nil
	}

	var count int64
	if err := db.DB.Model(&models.Category{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check if category name exists: %w", err)
	}
	return count > 0, nil
}

func (db *Database) checkCategoryUsage(categoryID uint) error {
	var count int64

	// Check songs
	if err := db.DB.Model(&models.Song{}).Where("category_id = ?", categoryID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check category usage in songs: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%d songs are using this category", count)
	}

	// Check artists
	if err := db.DB.Model(&models.Artist{}).Where("category_id = ?", categoryID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check category usage in artists: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%d artists are using this category", count)
	}

	// Check albums
	if err := db.DB.Model(&models.Album{}).Where("category_id = ?", categoryID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check category usage in albums: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%d albums are using this category", count)
	}

	// Check units
	if err := db.DB.Model(&models.Unit{}).Where("category_id = ?", categoryID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check category usage in units: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%d units are using this category", count)
	}

	return nil
}

func (db *Database) SearchCategories(query string) ([]models.Category, error) {
	if strings.TrimSpace(query) == "" {
		return db.GetAllCategories()
	}

	var categories []models.Category
	searchPattern := "%" + query + "%"
	if err := db.DB.Where("name ILIKE ?", searchPattern).Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("failed to search categories: %w", err)
	}
	return categories, nil
}