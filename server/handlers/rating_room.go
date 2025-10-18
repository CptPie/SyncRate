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
	// Global room manager instance
	roomManager = wsocket.NewRoomManager()

	// WebSocket upgrader
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for now (restrict in production)
		},
	}
)

// Start cleanup routine
func init() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				roomManager.CleanupInactiveRooms(10 * time.Minute)
			}
		}
	}()
}

// GetCreateRatingRoom shows the room creation page
func GetCreateRatingRoom(db *gorm.DB) gin.HandlerFunc {
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
		templateData["title"] = "SyncRate | Create Rating Room"
		templateData["categories"] = categories

		c.HTML(http.StatusOK, "create-rating-room.html", templateData)
	}
}

// PostCreateRatingRoom creates a new rating room
func PostCreateRatingRoom(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if user is authenticated
		userID, exists := c.Get("user_id")
		if !exists || userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		// Parse request body for filters
		var requestBody struct {
			CategoryID *uint `json:"category_id"`
			CoversOnly bool  `json:"covers_only"`
		}

		// Bind JSON, but don't fail if body is empty (filters are optional)
		if err := c.ShouldBindJSON(&requestBody); err != nil && err.Error() != "EOF" {
			log.Printf("Error parsing request body: %v", err)
		}

		// Generate unique room code
		roomID := generateRoomCode()

		// Create room in database
		room := models.RatingRoom{
			RoomID:     roomID,
			CreatorID:  userID.(uint),
			CategoryID: requestBody.CategoryID,
			CoversOnly: requestBody.CoversOnly,
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
		}

		if err := db.Create(&room).Error; err != nil {
			log.Printf("Error creating room in database: %v", err)
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
		roomManager.CreateRoom(roomID, fmt.Sprintf("%d", userID.(uint)), usernameStr)

		c.JSON(http.StatusOK, gin.H{
			"room_id": roomID,
			"url":     fmt.Sprintf("/rating-room/%s", roomID),
		})
	}
}

// GetRatingRoom shows the rating room interface
func GetRatingRoom(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		roomID := c.Param("roomId")

		// Check if user is authenticated
		userID, exists := c.Get("user_id")
		if !exists || userID == nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}

		// Check if room exists in database
		var room models.RatingRoom
		if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.HTML(http.StatusNotFound, "error.html", gin.H{
					"title": "SyncRate | Room Not Found",
					"error": "Rating room not found",
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
		templateData["title"] = fmt.Sprintf("SyncRate | Rating Room %s", roomID)
		templateData["room"] = room
		templateData["room_id"] = roomID

		c.HTML(http.StatusOK, "rating-room.html", templateData)
	}
}

// GetRatingRoomWS handles WebSocket connections for rating rooms
func GetRatingRoomWS(db *gorm.DB) gin.HandlerFunc {
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
		if err := roomManager.JoinRoom(roomID, userIDStr, usernameStr, conn); err != nil {
			log.Printf("Error joining room: %v", err)
			conn.WriteJSON(map[string]interface{}{
				"type":  "error",
				"error": err.Error(),
			})
			return
		}

		// Handle connection
		handleRoomConnection(db, roomID, userIDStr, conn)

		// Clean up when connection closes
		roomManager.LeaveRoom(userIDStr)
	}
}

// handleRoomConnection manages the WebSocket connection for a room
func handleRoomConnection(db *gorm.DB, roomID, userID string, conn *websocket.Conn) {
	// Send initial room state
	sendRoomState(db, roomID, conn)

	// Listen for messages
	for {
		var msg wsocket.WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		handleRoomMessage(db, roomID, userID, msg)
	}
}

// sendRoomState sends the current room state to a newly connected client
func sendRoomState(db *gorm.DB, roomID string, conn *websocket.Conn) {
	// Get room from database
	var room models.RatingRoom
	if err := db.Preload("CurrentSong").Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return
	}

	// If there's a current song, send it
	if room.CurrentSong != nil {
		// Load song with related data
		var song models.Song
		if err := db.Preload("Artists").Preload("Units").Preload("Category").
			First(&song, room.CurrentSong.SongID).Error; err == nil {

			// Get embed URL using existing utility function
			embedURL := ""
			if utils.IsYouTubeURL(song.SourceURL) {
				if url, err := utils.GetYouTubeEmbedURL(song.SourceURL); err == nil {
					embedURL = url
				}
			}

			// Send song change message
			songData := wsocket.SongChangeData{
				SongID:       song.SongID,
				SongTitle:    song.NameOriginal,
				EmbedURL:     embedURL,
				ThumbnailURL: song.ThumbnailURL,
			}

			data, _ := json.Marshal(songData)
			message := wsocket.WSMessage{
				Type:      wsocket.MsgSongChange,
				Data:      data,
				Timestamp: time.Now(),
			}

			conn.WriteJSON(message)
		}
	}
}

// handleRoomMessage processes incoming WebSocket messages
func handleRoomMessage(db *gorm.DB, roomID, userID string, msg wsocket.WSMessage) {
	switch msg.Type {
	case wsocket.MsgVideoSync:
		// Broadcast video sync to all room members
		roomManager.BroadcastToRoom(roomID, msg)

	case wsocket.MsgVoteUpdate:
		// Handle vote update (save to database and broadcast)
		handleVoteUpdate(db, roomID, userID, msg.Data)

	case wsocket.MsgNextSong:
		// Handle next song request
		handleNextSong(db, roomID, userID)

	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

// handleVoteUpdate processes vote updates
func handleVoteUpdate(db *gorm.DB, roomID, userID string, data json.RawMessage) {
	var voteData wsocket.VoteUpdateData
	if err := json.Unmarshal(data, &voteData); err != nil {
		log.Printf("Error unmarshaling vote data: %v", err)
		return
	}

	// Get current song for the room
	var room models.RatingRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return
	}

	if room.CurrentSongID == nil {
		return
	}

	// Convert userID to uint
	var userIDUint uint
	if _, err := fmt.Sscanf(userID, "%d", &userIDUint); err != nil {
		return
	}

	// Create or update vote
	vote := models.Vote{
		UserID:  userIDUint,
		SongID:  *room.CurrentSongID,
		Rating:  voteData.Rating,
		Comment: voteData.Comment,
	}

	// Use GORM's upsert (create or update)
	if err := db.Where("user_id = ? AND song_id = ?", userIDUint, *room.CurrentSongID).
		Assign(vote).FirstOrCreate(&vote).Error; err != nil {
		log.Printf("Error saving vote: %v", err)
		return
	}

	// Broadcast vote update to room
	message := wsocket.WSMessage{
		Type:      wsocket.MsgVoteUpdate,
		Data:      data,
		Timestamp: time.Now(),
	}

	roomManager.BroadcastToRoom(roomID, message)
}

// handleNextSong handles requests to move to the next song
func handleNextSong(db *gorm.DB, roomID, userID string) {
	// For now, allow any user to advance (could add creator-only restriction later)
	nextSong := findNextUnratedSong(db, roomID)
	if nextSong != nil {
		updateRoomCurrentSong(db, roomID, nextSong.SongID)
		broadcastSongChange(db, roomID, *nextSong)
	} else {
		// No more unrated songs - could broadcast "completed" message
		log.Printf("No more unrated songs for room %s", roomID)
	}
}

// findNextUnratedSong finds the next song that hasn't been rated by at least one user in the room
func findNextUnratedSong(db *gorm.DB, roomID string) *models.Song {
	// Get all users in the room
	room, exists := roomManager.GetRoom(roomID)
	if !exists {
		return nil
	}

	room.Mutex.RLock()
	userIDs := make([]string, 0, len(room.Clients))
	for userID := range room.Clients {
		userIDs = append(userIDs, userID)
	}
	room.Mutex.RUnlock()

	if len(userIDs) == 0 {
		return nil
	}

	// Load room filters from database
	var dbRoom models.RatingRoom
	if err := db.Where("room_id = ?", roomID).First(&dbRoom).Error; err != nil {
		log.Printf("Error loading room filters: %v", err)
		return nil
	}

	// Build base query with filters
	baseQuery := db.Preload("Artists").Preload("Units").Preload("Category")

	// Apply category filter if set
	if dbRoom.CategoryID != nil {
		baseQuery = baseQuery.Where("category_id = ?", *dbRoom.CategoryID)
	}

	// Apply covers filter if set
	if dbRoom.CoversOnly {
		baseQuery = baseQuery.Where("is_cover = ?", true)
	}

	// Find songs that haven't been rated by at least one user in the room
	var song models.Song
	err := baseQuery.
		Where("song_id NOT IN (?)",
			db.Table("votes").
				Select("DISTINCT song_id").
				Where("user_id IN ?", userIDs).
				Group("song_id").
				Having("COUNT(DISTINCT user_id) = ?", len(userIDs)),
		).
		Order("RANDOM()").
		First(&song).Error

	if err != nil {
		// If no completely unrated songs, find songs rated by fewer than all users
		err = baseQuery.
			Where("song_id NOT IN (?)",
				db.Table("votes").
					Select("song_id").
					Where("user_id IN ?", userIDs).
					Group("song_id").
					Having("COUNT(*) >= ?", len(userIDs)),
			).
			Order("RANDOM()").
			First(&song).Error

		if err != nil {
			// All songs have been rated by all users
			return nil
		}
	}

	return &song
}

// updateRoomCurrentSong updates the current song in the database
func updateRoomCurrentSong(db *gorm.DB, roomID string, songID uint) error {
	return db.Model(&models.RatingRoom{}).
		Where("room_id = ?", roomID).
		Updates(map[string]interface{}{
			"current_song_id": songID,
			"last_active":     time.Now(),
		}).Error
}

// broadcastSongChange sends a song change message to all users in the room
func broadcastSongChange(db *gorm.DB, roomID string, song models.Song) {
	// Get embed URL using existing utility function
	embedURL := ""
	if utils.IsYouTubeURL(song.SourceURL) {
		if url, err := utils.GetYouTubeEmbedURL(song.SourceURL); err == nil {
			embedURL = url
		}
	}

	// Create song change message
	songData := wsocket.SongChangeData{
		SongID:       song.SongID,
		SongTitle:    song.NameOriginal,
		EmbedURL:     embedURL,
		ThumbnailURL: song.ThumbnailURL,
	}

	data, _ := json.Marshal(songData)
	message := wsocket.WSMessage{
		Type:      wsocket.MsgSongChange,
		Data:      data,
		Timestamp: time.Now(),
	}

	roomManager.BroadcastToRoom(roomID, message)
}

// Helper functions

// generateRoomCode creates a unique 6-character room code
func generateRoomCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}