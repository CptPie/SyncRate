package handlers

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/CptPie/SyncRate/models"
	"github.com/CptPie/SyncRate/server/utils"
	wsocket "github.com/CptPie/SyncRate/server/websocket"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

var (
	// Global radio room manager instance
	radioRoomManager = wsocket.NewRoomManager()
)

// StartRadioRoomDatabaseCleanup starts a background routine to clean up old radio rooms from the database
func StartRadioRoomDatabaseCleanup(db *gorm.DB) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cleanupOldRadioRooms(db, 24*time.Hour)
			}
		}
	}()
}

// cleanupOldRadioRooms removes radio rooms that haven't been active for the specified duration
func cleanupOldRadioRooms(db *gorm.DB, inactivityThreshold time.Duration) {
	cutoffTime := time.Now().Add(-inactivityThreshold)

	result := db.Where("last_active < ?", cutoffTime).Delete(&models.RadioRoom{})
	if result.Error != nil {
		log.Printf("Error cleaning up old radio rooms: %v", result.Error)
		return
	}

	if result.RowsAffected > 0 {
		log.Printf("Cleaned up %d inactive radio rooms from database (older than %v)", result.RowsAffected, inactivityThreshold)
	}
}

// GetCreateRadioRoom shows the radio room creation page
func GetCreateRadioRoom(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if user is authenticated
		if userID, exists := c.Get("user_id"); !exists || userID == nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}

		// Load categories for filter options
		var categories []models.Category
		db.Find(&categories)

		templateData := GetUserContext(c)
		templateData["title"] = "SyncRate | Create Radio Room"
		templateData["categories"] = categories

		c.HTML(http.StatusOK, "create-radio-room.html", templateData)
	}
}

// PostCreateRadioRoom creates a new radio room
func PostCreateRadioRoom(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if user is authenticated
		userID, exists := c.Get("user_id")
		if !exists || userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		// Parse request body for filters
		var requestBody struct {
			CategoryID    *uint `json:"category_id"`
			MinRating     *int  `json:"min_rating"`
			IncludeCovers bool  `json:"include_covers"`
		}

		// Bind JSON, but don't fail if body is empty (filters are optional)
		if err := c.ShouldBindJSON(&requestBody); err != nil && err.Error() != "EOF" {
			log.Printf("Error parsing request body: %v", err)
		}

		log.Printf("Creating radio room with filters: %+v", requestBody)

		// Generate unique room code
		roomID := generateRadioRoomCode()

		// Create room in database
		room := models.RadioRoom{
			RoomID:        roomID,
			CreatorID:     userID.(uint),
			CategoryID:    requestBody.CategoryID,
			MinRating:     requestBody.MinRating,
			IncludeCovers: requestBody.IncludeCovers,
			CreatedAt:     time.Now(),
			LastActive:    time.Now(),
		}

		if err := db.Create(&room).Error; err != nil {
			log.Printf("Error creating radio room in database: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create room"})
			return
		}

		// Get username for room manager
		username, _ := c.Get("username")
		usernameStr := ""
		if username != nil {
			usernameStr = username.(string)
		}

		// Create room in memory
		radioRoomManager.CreateRoom(roomID, fmt.Sprintf("%d", userID.(uint)), usernameStr)

		c.JSON(http.StatusOK, gin.H{
			"room_id": roomID,
			"url":     fmt.Sprintf("/radio-room/%s", roomID),
		})
	}
}

// GetRadioRoom shows the radio room interface
func GetRadioRoom(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		roomID := c.Param("roomId")

		// Check if user is authenticated
		userID, exists := c.Get("user_id")
		if !exists || userID == nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}

		// Check if room exists in database
		var room models.RadioRoom
		if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.HTML(http.StatusNotFound, "error.html", gin.H{
					"title": "SyncRate | Room Not Found",
					"error": "Radio room not found",
				})
				return
			}
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Failed to load room",
			})
			return
		}

		templateData := GetUserContext(c)
		templateData["title"] = fmt.Sprintf("SyncRate | Radio Room %s", roomID)
		templateData["room"] = room
		templateData["room_id"] = roomID

		c.HTML(http.StatusOK, "radio-room.html", templateData)
	}
}

// GetRadioRoomWS handles WebSocket connections for radio rooms
func GetRadioRoomWS(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		roomID := c.Param("roomId")

		// Check if user is authenticated
		userID, exists := c.Get("user_id")
		if !exists || userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		username, _ := c.Get("username")
		usernameStr := ""
		if username != nil {
			usernameStr = username.(string)
		}

		// Upgrade connection to WebSocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Join room
		userIDStr := fmt.Sprintf("%d", userID.(uint))
		if err := radioRoomManager.JoinRoom(roomID, userIDStr, usernameStr, conn); err != nil {
			log.Printf("Error joining radio room: %v", err)
			conn.WriteJSON(map[string]interface{}{
				"type":  "error",
				"error": err.Error(),
			})
			return
		}

		// Handle connection
		handleRadioRoomConnection(db, roomID, userIDStr, conn)

		// Clean up when connection closes
		radioRoomManager.LeaveRoom(userIDStr)
	}
}

// handleRadioRoomConnection manages the WebSocket connection for a radio room
func handleRadioRoomConnection(db *gorm.DB, roomID, userID string, conn *websocket.Conn) {
	// Check if room exists in database
	if err := checkRadioRoomExists(db, roomID); err != nil {
		conn.WriteJSON(map[string]interface{}{
			"type":  "error",
			"error": "This radio room no longer exists",
		})
		return
	}

	// Send initial room state
	sendRadioRoomState(db, roomID, conn)

	// Listen for messages
	for {
		var msg wsocket.WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		handleRadioRoomMessage(db, roomID, userID, msg, conn)
	}
}

// sendRadioRoomState sends the current room state to a newly connected client
func sendRadioRoomState(db *gorm.DB, roomID string, conn *websocket.Conn) {
	// Get room from database
	var room models.RadioRoom
	if err := db.Preload("CurrentSong").Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return
	}

	// Radio rooms always have video sync enabled
	settingsData, _ := json.Marshal(wsocket.RoomSettingsData{
		VideoSyncEnabled: true,
	})
	settingsMessage := wsocket.WSMessage{
		Type:      wsocket.MsgRoomSettings,
		Data:      settingsData,
		Timestamp: time.Now(),
	}
	conn.WriteJSON(settingsMessage)

	// If there's a current song, send it
	if room.CurrentSong != nil {
		sendRadioSongData(db, roomID, *room.CurrentSong, conn)
	}
}

// handleRadioRoomMessage processes incoming WebSocket messages
func handleRadioRoomMessage(db *gorm.DB, roomID, userID string, msg wsocket.WSMessage, conn *websocket.Conn) {
	// Check if room still exists in database
	if err := checkRadioRoomExists(db, roomID); err != nil {
		log.Printf("Radio room %s no longer exists: %v", roomID, err)
		conn.WriteJSON(map[string]interface{}{
			"type":  "error",
			"error": "This radio room no longer exists. The page will reload.",
		})
		return
	}

	// Update last_active timestamp for any room activity
	updateRadioRoomActivity(db, roomID)

	switch msg.Type {
	case wsocket.MsgVideoSync:
		// Broadcast video sync to all room members
		radioRoomManager.BroadcastToRoom(roomID, msg)

	case wsocket.MsgVoteUpdate:
		// Handle vote update (broadcast to all users)
		handleRadioVoteUpdate(db, roomID, userID, msg.Data)

	case wsocket.MsgNextSong:
		// Handle next song request
		handleRadioNextSong(db, roomID)

	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

// handleRadioVoteUpdate processes vote updates and broadcasts them
func handleRadioVoteUpdate(db *gorm.DB, roomID, userID string, data json.RawMessage) {
	var voteData wsocket.VoteUpdateData
	if err := json.Unmarshal(data, &voteData); err != nil {
		log.Printf("Error unmarshaling vote data: %v", err)
		return
	}

	// Get current song for the room
	var room models.RadioRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return
	}

	if room.CurrentSongID == nil {
		return
	}

	// Broadcast vote update to room (we don't save votes in radio rooms, just display them)
	message := wsocket.WSMessage{
		Type:      wsocket.MsgVoteUpdate,
		Data:      data,
		Timestamp: time.Now(),
	}

	radioRoomManager.BroadcastToRoom(roomID, message)
}

// handleRadioNextSong handles requests to move to the next song
func handleRadioNextSong(db *gorm.DB, roomID string) {
	nextSong := findNextRadioSong(db, roomID)
	if nextSong != nil {
		updateRadioRoomCurrentSong(db, roomID, nextSong.SongID)
		broadcastRadioSongChange(db, roomID, *nextSong)
	} else {
		log.Printf("No songs available for radio room %s", roomID)
	}
}

// findNextRadioSong finds the next song based on room filters
func findNextRadioSong(db *gorm.DB, roomID string) *models.Song {
	// Load room filters from database
	var dbRoom models.RadioRoom
	if err := db.Where("room_id = ?", roomID).First(&dbRoom).Error; err != nil {
		log.Printf("Error loading radio room filters: %v", err)
		return nil
	}

	// Build base query with filters
	baseQuery := db.Preload("Artists").Preload("Units").Preload("Albums").Preload("Category")

	// Apply category filter if set
	if dbRoom.CategoryID != nil {
		baseQuery = baseQuery.Where("category_id = ?", *dbRoom.CategoryID)
	}

	// Apply covers filter
	if !dbRoom.IncludeCovers {
		baseQuery = baseQuery.Where("is_cover = ?", false)
	}

	// Apply rating filter if set
	if dbRoom.MinRating != nil {
		// Need to join with votes and calculate average rating
		baseQuery = baseQuery.Where("song_id IN (?)",
			db.Table("votes").
				Select("song_id").
				Group("song_id").
				Having("AVG(rating) >= ?", *dbRoom.MinRating),
		)
	}

	var song models.Song
	err := baseQuery.
		Order("RANDOM()").
		First(&song).Error

	if err != nil {
		log.Printf("Error finding next radio song: %v", err)
		return nil
	}

	return &song
}

// updateRadioRoomCurrentSong updates the current song in the database
func updateRadioRoomCurrentSong(db *gorm.DB, roomID string, songID uint) error {
	return db.Model(&models.RadioRoom{}).
		Where("room_id = ?", roomID).
		Updates(map[string]interface{}{
			"current_song_id": songID,
			"last_active":     time.Now(),
		}).Error
}

// broadcastRadioSongChange sends a song change message to all users in the radio room
func broadcastRadioSongChange(db *gorm.DB, roomID string, song models.Song) {
	sendRadioSongDataToRoom(db, roomID, song)
}

// sendRadioSongData sends song data to a specific connection
func sendRadioSongData(db *gorm.DB, roomID string, song models.Song, conn *websocket.Conn) {
	// Load song with related data if not already loaded
	var fullSong models.Song
	if err := db.Preload("Artists").Preload("Units").Preload("Albums").Preload("Category").
		First(&fullSong, song.SongID).Error; err != nil {
		log.Printf("Error loading song data: %v", err)
		return
	}

	// Get embed URL using existing utility function
	embedURL := ""
	if utils.IsYouTubeURL(fullSong.SourceURL) {
		if url, err := utils.GetYouTubeEmbedURL(fullSong.SourceURL); err == nil {
			embedURL = url
		}
	}

	// Build artists array with color information
	artists := make([]wsocket.ArtistData, 0, len(fullSong.Artists))
	for _, artist := range fullSong.Artists {
		artists = append(artists, wsocket.ArtistData{
			ArtistID:       artist.ArtistID,
			NameOriginal:   artist.NameOriginal,
			NameEnglish:    artist.NameEnglish,
			PrimaryColor:   artist.PrimaryColor,
			SecondaryColor: artist.SecondaryColor,
		})
	}

	// Build units array with color information
	units := make([]wsocket.UnitData, 0, len(fullSong.Units))
	for _, unit := range fullSong.Units {
		units = append(units, wsocket.UnitData{
			UnitID:         unit.UnitID,
			NameOriginal:   unit.NameOriginal,
			NameEnglish:    unit.NameEnglish,
			PrimaryColor:   unit.PrimaryColor,
			SecondaryColor: unit.SecondaryColor,
		})
	}

	// Build albums array
	albums := make([]wsocket.AlbumData, 0, len(fullSong.Albums))
	for _, album := range fullSong.Albums {
		albums = append(albums, wsocket.AlbumData{
			AlbumID:      album.AlbumID,
			NameOriginal: album.NameOriginal,
			NameEnglish:  album.NameEnglish,
			Type:         album.Type,
		})
	}

	// Get category name
	categoryName := ""
	if fullSong.Category != nil {
		categoryName = fullSong.Category.Name
	}

	// Load existing votes for this song from users in the room
	existingVotes := loadRadioRoomVotes(db, roomID, fullSong.SongID)

	// Create song change message
	songData := wsocket.SongChangeData{
		SongID:            fullSong.SongID,
		SongTitleOriginal: fullSong.NameOriginal,
		SongTitleEnglish:  fullSong.NameEnglish,
		EmbedURL:          embedURL,
		ThumbnailURL:      fullSong.ThumbnailURL,
		Artists:           artists,
		Units:             units,
		Albums:            albums,
		Category:          categoryName,
		IsCover:           fullSong.IsCover,
		ExistingVotes:     existingVotes,
	}

	data, _ := json.Marshal(songData)
	message := wsocket.WSMessage{
		Type:      wsocket.MsgSongChange,
		Data:      data,
		Timestamp: time.Now(),
	}

	conn.WriteJSON(message)
}

// sendRadioSongDataToRoom broadcasts song data to all connections in the room
func sendRadioSongDataToRoom(db *gorm.DB, roomID string, song models.Song) {
	// Load song with related data if not already loaded
	var fullSong models.Song
	if err := db.Preload("Artists").Preload("Units").Preload("Albums").Preload("Category").
		First(&fullSong, song.SongID).Error; err != nil {
		log.Printf("Error loading song data: %v", err)
		return
	}

	// Get embed URL using existing utility function
	embedURL := ""
	if utils.IsYouTubeURL(fullSong.SourceURL) {
		if url, err := utils.GetYouTubeEmbedURL(fullSong.SourceURL); err == nil {
			embedURL = url
		}
	}

	// Build artists array with color information
	artists := make([]wsocket.ArtistData, 0, len(fullSong.Artists))
	for _, artist := range fullSong.Artists {
		artists = append(artists, wsocket.ArtistData{
			ArtistID:       artist.ArtistID,
			NameOriginal:   artist.NameOriginal,
			NameEnglish:    artist.NameEnglish,
			PrimaryColor:   artist.PrimaryColor,
			SecondaryColor: artist.SecondaryColor,
		})
	}

	// Build units array with color information
	units := make([]wsocket.UnitData, 0, len(fullSong.Units))
	for _, unit := range fullSong.Units {
		units = append(units, wsocket.UnitData{
			UnitID:         unit.UnitID,
			NameOriginal:   unit.NameOriginal,
			NameEnglish:    unit.NameEnglish,
			PrimaryColor:   unit.PrimaryColor,
			SecondaryColor: unit.SecondaryColor,
		})
	}

	// Build albums array
	albums := make([]wsocket.AlbumData, 0, len(fullSong.Albums))
	for _, album := range fullSong.Albums {
		albums = append(albums, wsocket.AlbumData{
			AlbumID:      album.AlbumID,
			NameOriginal: album.NameOriginal,
			NameEnglish:  album.NameEnglish,
			Type:         album.Type,
		})
	}

	// Get category name
	categoryName := ""
	if fullSong.Category != nil {
		categoryName = fullSong.Category.Name
	}

	// Load existing votes for this song from users in the room
	existingVotes := loadRadioRoomVotes(db, roomID, fullSong.SongID)

	// Create song change message
	songData := wsocket.SongChangeData{
		SongID:            fullSong.SongID,
		SongTitleOriginal: fullSong.NameOriginal,
		SongTitleEnglish:  fullSong.NameEnglish,
		EmbedURL:          embedURL,
		ThumbnailURL:      fullSong.ThumbnailURL,
		Artists:           artists,
		Units:             units,
		Albums:            albums,
		Category:          categoryName,
		IsCover:           fullSong.IsCover,
		ExistingVotes:     existingVotes,
	}

	data, _ := json.Marshal(songData)
	message := wsocket.WSMessage{
		Type:      wsocket.MsgSongChange,
		Data:      data,
		Timestamp: time.Now(),
	}

	radioRoomManager.BroadcastToRoom(roomID, message)
}

// Helper functions

// loadRadioRoomVotes loads all votes for a specific song from users currently in the room
func loadRadioRoomVotes(db *gorm.DB, roomID string, songID uint) []wsocket.VoteUpdateData {
	// Get all users in the room
	room, exists := radioRoomManager.GetRoom(roomID)
	if !exists {
		return []wsocket.VoteUpdateData{}
	}

	room.Mutex.RLock()
	userIDs := make([]string, 0, len(room.Clients))
	usernames := make(map[string]string) // map user_id to username
	for userID, client := range room.Clients {
		userIDs = append(userIDs, userID)
		usernames[userID] = client.Username
	}
	room.Mutex.RUnlock()

	if len(userIDs) == 0 {
		return []wsocket.VoteUpdateData{}
	}

	// Load votes for this song from these users
	var votes []models.Vote
	db.Where("song_id = ? AND user_id IN ?", songID, userIDs).Find(&votes)

	// Convert to VoteUpdateData
	voteData := make([]wsocket.VoteUpdateData, 0, len(votes))
	for _, vote := range votes {
		voteData = append(voteData, wsocket.VoteUpdateData{
			UserID:   fmt.Sprintf("%d", vote.UserID),
			Username: usernames[fmt.Sprintf("%d", vote.UserID)],
			Rating:   vote.Rating,
			Comment:  vote.Comment,
		})
	}

	return voteData
}

// updateRadioRoomActivity updates the last_active timestamp for a radio room
func updateRadioRoomActivity(db *gorm.DB, roomID string) {
	db.Model(&models.RadioRoom{}).
		Where("room_id = ?", roomID).
		Update("last_active", time.Now())
}

// checkRadioRoomExists checks if a radio room exists in the database and returns an error if not
func checkRadioRoomExists(db *gorm.DB, roomID string) error {
	var room models.RadioRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("room not found")
		}
		return fmt.Errorf("database error: %v", err)
	}
	return nil
}

// generateRadioRoomCode creates a unique 6-character room code
func generateRadioRoomCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}
