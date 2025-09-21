package database

import (
	"errors"
	"fmt"

	"github.com/CptPie/SyncRate/models"
)

func (db *Database) validateVote(vote *models.Vote, isUpdate bool) error {
	if vote == nil {
		return errors.New("vote cannot be nil")
	}

	// UserID validation
	if vote.UserID == 0 {
		return errors.New("user ID cannot be zero")
	}

	// SongID validation
	if vote.SongID == 0 {
		return errors.New("song ID cannot be zero")
	}

	// Rating validation (1-10 scale as defined in the model)
	if vote.Rating < 1 || vote.Rating > 10 {
		return errors.New("rating must be between 1 and 10")
	}

	// Check if user exists
	userExists, err := db.UserExists(vote.UserID)
	if err != nil {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}
	if !userExists {
		return errors.New("user does not exist")
	}

	// Check if song exists
	songExists, err := db.SongExists(vote.SongID)
	if err != nil {
		return fmt.Errorf("failed to check if song exists: %w", err)
	}
	if !songExists {
		return errors.New("song does not exist")
	}

	return nil
}

func (db *Database) CreateVote(vote *models.Vote) error {
	if err := db.validateVote(vote, false); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check if vote already exists (since it's a composite primary key)
	exists, err := db.VoteExists(vote.UserID, vote.SongID)
	if err != nil {
		return fmt.Errorf("failed to check if vote exists: %w", err)
	}
	if exists {
		return errors.New("vote already exists for this user and song")
	}

	if err := db.DB.Create(vote).Error; err != nil {
		return fmt.Errorf("failed to create vote: %w", err)
	}
	return nil
}

func (db *Database) GetVote(userID, songID uint) (*models.Vote, error) {
	if userID == 0 {
		return nil, errors.New("user ID cannot be zero")
	}
	if songID == 0 {
		return nil, errors.New("song ID cannot be zero")
	}

	var vote models.Vote
	if err := db.DB.Preload("User").Preload("Song").
		Where("user_id = ? AND song_id = ?", userID, songID).First(&vote).Error; err != nil {
		return nil, fmt.Errorf("failed to get vote: %w", err)
	}
	return &vote, nil
}

func (db *Database) GetVotesByUser(userID uint) ([]models.Vote, error) {
	if userID == 0 {
		return nil, errors.New("user ID cannot be zero")
	}

	// Check if user exists
	exists, err := db.UserExists(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if user exists: %w", err)
	}
	if !exists {
		return nil, errors.New("user does not exist")
	}

	var votes []models.Vote
	if err := db.DB.Preload("User").Preload("Song").
		Where("user_id = ?", userID).Find(&votes).Error; err != nil {
		return nil, fmt.Errorf("failed to get votes by user: %w", err)
	}
	return votes, nil
}

func (db *Database) GetVotesBySong(songID uint) ([]models.Vote, error) {
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

	var votes []models.Vote
	if err := db.DB.Preload("User").Preload("Song").
		Where("song_id = ?", songID).Find(&votes).Error; err != nil {
		return nil, fmt.Errorf("failed to get votes by song: %w", err)
	}
	return votes, nil
}

func (db *Database) GetVotesByRating(rating int) ([]models.Vote, error) {
	if rating < 1 || rating > 10 {
		return nil, errors.New("rating must be between 1 and 10")
	}

	var votes []models.Vote
	if err := db.DB.Preload("User").Preload("Song").
		Where("rating = ?", rating).Find(&votes).Error; err != nil {
		return nil, fmt.Errorf("failed to get votes by rating: %w", err)
	}
	return votes, nil
}

func (db *Database) GetAllVotes() ([]models.Vote, error) {
	var votes []models.Vote
	if err := db.DB.Preload("User").Preload("Song").Find(&votes).Error; err != nil {
		return nil, fmt.Errorf("failed to get all votes: %w", err)
	}
	return votes, nil
}

func (db *Database) UpdateVote(vote *models.Vote) error {
	if vote == nil {
		return errors.New("vote cannot be nil")
	}

	// Validate the vote
	if err := db.validateVote(vote, true); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check if vote exists
	exists, err := db.VoteExists(vote.UserID, vote.SongID)
	if err != nil {
		return fmt.Errorf("failed to check if vote exists: %w", err)
	}
	if !exists {
		return errors.New("vote does not exist")
	}

	if err := db.DB.Save(vote).Error; err != nil {
		return fmt.Errorf("failed to update vote: %w", err)
	}
	return nil
}

func (db *Database) DeleteVote(userID, songID uint) error {
	if userID == 0 {
		return errors.New("user ID cannot be zero")
	}
	if songID == 0 {
		return errors.New("song ID cannot be zero")
	}

	// Check if vote exists
	exists, err := db.VoteExists(userID, songID)
	if err != nil {
		return fmt.Errorf("failed to check if vote exists: %w", err)
	}
	if !exists {
		return errors.New("vote does not exist")
	}

	if err := db.DB.Where("user_id = ? AND song_id = ?", userID, songID).Delete(&models.Vote{}).Error; err != nil {
		return fmt.Errorf("failed to delete vote: %w", err)
	}
	return nil
}

func (db *Database) VoteExists(userID, songID uint) (bool, error) {
	if userID == 0 || songID == 0 {
		return false, nil
	}

	var count int64
	if err := db.DB.Model(&models.Vote{}).
		Where("user_id = ? AND song_id = ?", userID, songID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check if vote exists: %w", err)
	}
	return count > 0, nil
}

func (db *Database) GetAverageRatingForSong(songID uint) (float64, error) {
	if songID == 0 {
		return 0, errors.New("song ID cannot be zero")
	}

	// Check if song exists
	exists, err := db.SongExists(songID)
	if err != nil {
		return 0, fmt.Errorf("failed to check if song exists: %w", err)
	}
	if !exists {
		return 0, errors.New("song does not exist")
	}

	var avgRating float64
	if err := db.DB.Model(&models.Vote{}).
		Where("song_id = ?", songID).
		Select("AVG(rating)").Scan(&avgRating).Error; err != nil {
		return 0, fmt.Errorf("failed to calculate average rating: %w", err)
	}
	return avgRating, nil
}

func (db *Database) GetVoteCountForSong(songID uint) (int64, error) {
	if songID == 0 {
		return 0, errors.New("song ID cannot be zero")
	}

	// Check if song exists
	exists, err := db.SongExists(songID)
	if err != nil {
		return 0, fmt.Errorf("failed to check if song exists: %w", err)
	}
	if !exists {
		return 0, errors.New("song does not exist")
	}

	var count int64
	if err := db.DB.Model(&models.Vote{}).
		Where("song_id = ?", songID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count votes: %w", err)
	}
	return count, nil
}

func (db *Database) UpsertVote(vote *models.Vote) error {
	if err := db.validateVote(vote, false); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check if vote exists
	exists, err := db.VoteExists(vote.UserID, vote.SongID)
	if err != nil {
		return fmt.Errorf("failed to check if vote exists: %w", err)
	}

	if exists {
		// Update existing vote
		return db.UpdateVote(vote)
	} else {
		// Create new vote
		return db.CreateVote(vote)
	}
}