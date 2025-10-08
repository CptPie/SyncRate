package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/CptPie/SyncRate/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ============= SONG API ENDPOINTS =============

type CreateSongRequest struct {
	NameOriginal string  `json:"name_original" binding:"required"`
	NameEnglish  string  `json:"name_english"`
	SourceURL    string  `json:"source_url" binding:"required"`
	ThumbnailURL string  `json:"thumbnail_url" binding:"required"`
	CategoryID   *uint   `json:"category_id"`
	IsCover      bool    `json:"is_cover"`
	ArtistIDs    []uint  `json:"artist_ids"`
	UnitIDs      []uint  `json:"unit_ids"`
	AlbumIDs     []uint  `json:"album_ids"`
}

func PostAPISong(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateSongRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		song := models.Song{
			NameOriginal: req.NameOriginal,
			NameEnglish:  req.NameEnglish,
			SourceURL:    req.SourceURL,
			ThumbnailURL: req.ThumbnailURL,
			CategoryID:   req.CategoryID,
			IsCover:      req.IsCover,
		}

		// Start transaction
		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// Create song
		if err := tx.Create(&song).Error; err != nil {
			tx.Rollback()
			log.Printf("Error creating song: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create song"})
			return
		}

		// Associate artists
		if len(req.ArtistIDs) > 0 {
			var artists []models.Artist
			if err := tx.Where("artist_id IN ?", req.ArtistIDs).Find(&artists).Error; err != nil {
				tx.Rollback()
				log.Printf("Error finding artists: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid artist IDs"})
				return
			}
			if len(artists) != len(req.ArtistIDs) {
				tx.Rollback()
				log.Printf("Artist count mismatch: requested %d, found %d", len(req.ArtistIDs), len(artists))
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Some artist IDs not found: requested %d, found %d", len(req.ArtistIDs), len(artists))})
				return
			}
			if err := tx.Model(&song).Association("Artists").Append(&artists); err != nil {
				tx.Rollback()
				log.Printf("Error associating artists: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to associate artists"})
				return
			}
			log.Printf("Associated %d artists to song %d", len(artists), song.SongID)
		}

		// Associate units
		if len(req.UnitIDs) > 0 {
			var units []models.Unit
			if err := tx.Where("unit_id IN ?", req.UnitIDs).Find(&units).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid unit IDs"})
				return
			}
			if err := tx.Model(&song).Association("Units").Append(&units); err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to associate units"})
				return
			}
		}

		// Associate albums
		if len(req.AlbumIDs) > 0 {
			var albums []models.Album
			if err := tx.Where("album_id IN ?", req.AlbumIDs).Find(&albums).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid album IDs"})
				return
			}
			if err := tx.Model(&song).Association("Albums").Append(&albums); err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to associate albums"})
				return
			}
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
			return
		}

		// Load full song with associations
		db.Preload("Artists").Preload("Units").Preload("Category").Preload("Albums").First(&song, song.SongID)

		c.JSON(http.StatusCreated, song)
	}
}

// ============= ARTIST API ENDPOINTS =============

type CreateArtistRequest struct {
	NameOriginal   string `json:"name_original" binding:"required"`
	NameEnglish    string `json:"name_english"`
	PrimaryColor   string `json:"primary_color"`
	SecondaryColor string `json:"secondary_color"`
	CategoryID     *uint  `json:"category_id"`
	UnitIDs        []uint `json:"unit_ids"`
}

func PostAPIArtist(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateArtistRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		artist := models.Artist{
			NameOriginal:   req.NameOriginal,
			NameEnglish:    req.NameEnglish,
			PrimaryColor:   req.PrimaryColor,
			SecondaryColor: req.SecondaryColor,
			CategoryID:     req.CategoryID,
		}

		// Start transaction
		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// Create artist
		if err := tx.Create(&artist).Error; err != nil {
			tx.Rollback()
			log.Printf("Error creating artist: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create artist"})
			return
		}

		// Associate units
		if len(req.UnitIDs) > 0 {
			var units []models.Unit
			if err := tx.Where("unit_id IN ?", req.UnitIDs).Find(&units).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid unit IDs"})
				return
			}
			if err := tx.Model(&artist).Association("Units").Append(&units); err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to associate units"})
				return
			}
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
			return
		}

		// Load full artist with associations
		db.Preload("Units").Preload("Category").First(&artist, artist.ArtistID)

		c.JSON(http.StatusCreated, artist)
	}
}

// ============= ALBUM API ENDPOINTS =============

type CreateAlbumRequest struct {
	NameOriginal string `json:"name_original" binding:"required"`
	NameEnglish  string `json:"name_english"`
	AlbumArtURL  string `json:"album_art_url"`
	Type         string `json:"type" binding:"required,oneof=Album Single EP"`
	CategoryID   *uint  `json:"category_id"`
	SongIDs      []uint `json:"song_ids"`
}

func PostAPIAlbum(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateAlbumRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		album := models.Album{
			NameOriginal: req.NameOriginal,
			NameEnglish:  req.NameEnglish,
			AlbumArtURL:  req.AlbumArtURL,
			Type:         req.Type,
			CategoryID:   req.CategoryID,
		}

		// Start transaction
		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// Create album
		if err := tx.Create(&album).Error; err != nil {
			tx.Rollback()
			log.Printf("Error creating album: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create album"})
			return
		}

		// Associate songs
		if len(req.SongIDs) > 0 {
			var songs []models.Song
			if err := tx.Where("song_id IN ?", req.SongIDs).Find(&songs).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid song IDs"})
				return
			}
			if err := tx.Model(&album).Association("Songs").Append(&songs); err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to associate songs"})
				return
			}
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
			return
		}

		// Load full album with associations
		db.Preload("Songs").Preload("Category").First(&album, album.AlbumID)

		c.JSON(http.StatusCreated, album)
	}
}

// ============= UNIT API ENDPOINTS =============

type CreateUnitRequest struct {
	NameOriginal   string `json:"name_original" binding:"required"`
	NameEnglish    string `json:"name_english"`
	PrimaryColor   string `json:"primary_color"`
	SecondaryColor string `json:"secondary_color"`
	CategoryID     *uint  `json:"category_id"`
	ArtistIDs      []uint `json:"artist_ids"`
}

func PostAPIUnit(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateUnitRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		unit := models.Unit{
			NameOriginal:   req.NameOriginal,
			NameEnglish:    req.NameEnglish,
			PrimaryColor:   req.PrimaryColor,
			SecondaryColor: req.SecondaryColor,
			CategoryID:     req.CategoryID,
		}

		// Start transaction
		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// Create unit
		if err := tx.Create(&unit).Error; err != nil {
			tx.Rollback()
			log.Printf("Error creating unit: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create unit"})
			return
		}

		// Associate artists
		if len(req.ArtistIDs) > 0 {
			var artists []models.Artist
			if err := tx.Where("artist_id IN ?", req.ArtistIDs).Find(&artists).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid artist IDs"})
				return
			}
			if err := tx.Model(&unit).Association("Artists").Append(&artists); err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to associate artists"})
				return
			}
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
			return
		}

		// Load full unit with associations
		db.Preload("Artists").Preload("Category").First(&unit, unit.UnitID)

		c.JSON(http.StatusCreated, unit)
	}
}

// ============= CATEGORY API ENDPOINTS =============

type CreateCategoryRequest struct {
	Name string `json:"name" binding:"required"`
}

func PostAPICategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateCategoryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		category := models.Category{
			Name: req.Name,
		}

		if err := db.Create(&category).Error; err != nil {
			log.Printf("Error creating category: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
			return
		}

		c.JSON(http.StatusCreated, category)
	}
}

// ============= VOTE API ENDPOINTS =============

type CreateVoteRequest struct {
	UserID  uint   `json:"user_id" binding:"required"`
	SongID  uint   `json:"song_id" binding:"required"`
	Rating  int    `json:"rating" binding:"required,min=1,max=10"`
	Comment string `json:"comment"`
}

func PostAPIVote(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateVoteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check if user exists
		var user models.User
		if err := db.First(&user, req.UserID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User not found"})
			return
		}

		// Check if song exists
		var song models.Song
		if err := db.First(&song, req.SongID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Song not found"})
			return
		}

		// Check if vote already exists
		var existingVote models.Vote
		result := db.Where("user_id = ? AND song_id = ?", req.UserID, req.SongID).First(&existingVote)

		if result.Error == nil {
			// Update existing vote
			existingVote.Rating = req.Rating
			existingVote.Comment = req.Comment
			if err := db.Save(&existingVote).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vote"})
				return
			}
			c.JSON(http.StatusOK, existingVote)
			return
		}

		// Create new vote
		vote := models.Vote{
			UserID:  req.UserID,
			SongID:  req.SongID,
			Rating:  req.Rating,
			Comment: req.Comment,
		}

		if err := db.Create(&vote).Error; err != nil {
			log.Printf("Error creating vote: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create vote"})
			return
		}

		c.JSON(http.StatusCreated, vote)
	}
}

// ============= GET ENDPOINTS FOR LISTING =============

func GetAPISongs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var songs []models.Song
		result := db.Preload("Artists").Preload("Units").Preload("Category").Preload("Albums").Find(&songs)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch songs"})
			return
		}
		c.JSON(http.StatusOK, songs)
	}
}

func GetAPISong(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid song ID"})
			return
		}

		var song models.Song
		result := db.Preload("Artists").Preload("Units").Preload("Category").Preload("Albums").First(&song, uint(id))
		if result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Song not found"})
			return
		}
		c.JSON(http.StatusOK, song)
	}
}

func GetAPIArtists(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var artists []models.Artist
		result := db.Preload("Units").Preload("Category").Find(&artists)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch artists"})
			return
		}
		c.JSON(http.StatusOK, artists)
	}
}

func GetAPIArtist(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid artist ID"})
			return
		}

		var artist models.Artist
		result := db.Preload("Units").Preload("Category").First(&artist, uint(id))
		if result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Artist not found"})
			return
		}
		c.JSON(http.StatusOK, artist)
	}
}

func GetAPIAlbums(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var albums []models.Album
		result := db.Preload("Songs").Preload("Category").Find(&albums)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch albums"})
			return
		}
		c.JSON(http.StatusOK, albums)
	}
}

func GetAPIAlbum(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid album ID"})
			return
		}

		var album models.Album
		result := db.Preload("Songs").Preload("Category").First(&album, uint(id))
		if result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Album not found"})
			return
		}
		c.JSON(http.StatusOK, album)
	}
}

func GetAPIUnits(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var units []models.Unit
		result := db.Preload("Artists").Preload("Category").Find(&units)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch units"})
			return
		}
		c.JSON(http.StatusOK, units)
	}
}

func GetAPIUnit(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid unit ID"})
			return
		}

		var unit models.Unit
		result := db.Preload("Artists").Preload("Category").First(&unit, uint(id))
		if result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Unit not found"})
			return
		}
		c.JSON(http.StatusOK, unit)
	}
}

func GetAPICategories(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var categories []models.Category
		result := db.Find(&categories)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
			return
		}
		c.JSON(http.StatusOK, categories)
	}
}

func GetAPICategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
			return
		}

		var category models.Category
		result := db.First(&category, uint(id))
		if result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
			return
		}
		c.JSON(http.StatusOK, category)
	}
}

func GetAPIVotes(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var votes []models.Vote

		// Optional filtering by user_id or song_id
		query := db
		if userID := c.Query("user_id"); userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		if songID := c.Query("song_id"); songID != "" {
			query = query.Where("song_id = ?", songID)
		}

		result := query.Find(&votes)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch votes"})
			return
		}
		c.JSON(http.StatusOK, votes)
	}
}

func GetAPIVote(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid vote ID"})
			return
		}

		var vote models.Vote
		result := db.First(&vote, uint(id))
		if result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Vote not found"})
			return
		}
		c.JSON(http.StatusOK, vote)
	}
}

// ============= USER API ENDPOINTS =============

type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
}

func PostAPIUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check if user already exists
		var existingUser models.User
		if err := db.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}

		// Hash the temporary password
		passwordHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Error hashing password: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		user := models.User{
			Username:     req.Username,
			PasswordHash: string(passwordHash),
			Email:        req.Username + "@temp.syncrate.local",
		}

		if err := db.Create(&user).Error; err != nil {
			log.Printf("Error creating user: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		c.JSON(http.StatusCreated, user)
	}
}

func GetAPIUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var users []models.User

		// Optional filtering by username (fuzzy search)
		query := db
		if username := c.Query("username"); username != "" {
			// Fuzzy search using LIKE
			query = query.Where("username ILIKE ?", "%"+username+"%")
		}

		result := query.Find(&users)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
			return
		}
		c.JSON(http.StatusOK, users)
	}
}

func GetAPIUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		var user models.User
		result := db.First(&user, uint(id))
		if result.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusOK, user)
	}
}
