package database

import (
	"errors"
	"fmt"
	"strings"

	"github.com/CptPie/SyncRate/models"
)

func (db *Database) validateUser(user *models.User, isUpdate bool) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}

	// Username validation
	if strings.TrimSpace(user.Username) == "" {
		return errors.New("username cannot be empty")
	}
	if len(user.Username) > 50 {
		return errors.New("username cannot exceed 50 characters")
	}

	// Check username uniqueness (skip if updating and username hasn't changed)
	if !isUpdate {
		exists, err := db.UsernameExists(user.Username)
		if err != nil {
			return fmt.Errorf("failed to check username uniqueness: %w", err)
		}
		if exists {
			return errors.New("username already exists")
		}
	}

	return nil
}

func (db *Database) CreateUser(user *models.User) error {
	if err := db.validateUser(user, false); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := db.DB.Create(user).Error; err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (db *Database) GetUserByID(userID uint) (*models.User, error) {
	if userID == 0 {
		return nil, errors.New("user ID cannot be zero")
	}

	var user models.User
	if err := db.DB.Preload("Votes").First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

func (db *Database) GetUserByUsername(username string) (*models.User, error) {
	if strings.TrimSpace(username) == "" {
		return nil, errors.New("username cannot be empty")
	}

	var user models.User
	if err := db.DB.Preload("Votes").Where("username = ?", username).First(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}
	return &user, nil
}


func (db *Database) GetAllUsers() ([]models.User, error) {
	var users []models.User
	if err := db.DB.Preload("Votes").Find(&users).Error; err != nil {
		return nil, fmt.Errorf("failed to get all users: %w", err)
	}
	return users, nil
}

func (db *Database) UpdateUser(user *models.User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}
	if user.UserID == 0 {
		return errors.New("user ID cannot be zero")
	}

	// Check if user exists
	exists, err := db.UserExists(user.UserID)
	if err != nil {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}
	if !exists {
		return errors.New("user does not exist")
	}

	// For updates, we need to check uniqueness constraints more carefully
	// Get the current user to compare values
	currentUser, err := db.GetUserByID(user.UserID)
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Check username uniqueness only if it's being changed
	if user.Username != currentUser.Username {
		exists, err := db.UsernameExists(user.Username)
		if err != nil {
			return fmt.Errorf("failed to check username uniqueness: %w", err)
		}
		if exists {
			return errors.New("username already exists")
		}
	}

	// Validate the updated user data
	if err := db.validateUser(user, true); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := db.DB.Save(user).Error; err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

func (db *Database) DeleteUser(userID uint) error {
	if userID == 0 {
		return errors.New("user ID cannot be zero")
	}

	// Check if user exists
	exists, err := db.UserExists(userID)
	if err != nil {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}
	if !exists {
		return errors.New("user does not exist")
	}

	if err := db.DB.Delete(&models.User{}, userID).Error; err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

func (db *Database) UserExists(userID uint) (bool, error) {
	if userID == 0 {
		return false, nil
	}

	var count int64
	if err := db.DB.Model(&models.User{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check if user exists: %w", err)
	}
	return count > 0, nil
}

func (db *Database) UsernameExists(username string) (bool, error) {
	if strings.TrimSpace(username) == "" {
		return false, nil
	}

	var count int64
	if err := db.DB.Model(&models.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check if username exists: %w", err)
	}
	return count > 0, nil
}

