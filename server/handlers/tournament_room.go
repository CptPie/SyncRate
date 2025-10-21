package handlers

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math"
	mathrand "math/rand"
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
	// Global tournament room manager instance
	tournamentRoomManager = wsocket.NewRoomManager()
)

// StartTournamentDatabaseCleanup starts a background routine to clean up old tournament rooms
func StartTournamentDatabaseCleanup(db *gorm.DB) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cleanupOldTournamentRooms(db, 48*time.Hour) // Keep tournaments for 2 days
			}
		}
	}()
}

// cleanupOldTournamentRooms removes old tournament rooms
func cleanupOldTournamentRooms(db *gorm.DB, inactivityThreshold time.Duration) {
	cutoffTime := time.Now().Add(-inactivityThreshold)

	result := db.Where("last_active < ?", cutoffTime).Delete(&models.TournamentRoom{})
	if result.Error != nil {
		log.Printf("Error cleaning up old tournament rooms: %v", result.Error)
		return
	}

	if result.RowsAffected > 0 {
		log.Printf("Cleaned up %d inactive tournament rooms from database", result.RowsAffected)
	}
}

// GetCreateTournamentRoom shows the tournament creation page
func GetCreateTournamentRoom(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if user is authenticated
		if userID, exists := c.Get("user_id"); !exists || userID == nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}

		// Load categories
		var categories []models.Category
		db.Find(&categories)

		templateData := GetUserContext(c)
		templateData["title"] = "SyncRate | Create Tournament"
		templateData["categories"] = categories

		c.HTML(http.StatusOK, "create-tournament-room.html", templateData)
	}
}

// PostCreateTournamentRoom creates a new tournament room
func PostCreateTournamentRoom(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if user is authenticated
		userID, exists := c.Get("user_id")
		if !exists || userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		// Parse request body
		var requestBody struct {
			TreeSize         int      `json:"tree_size"`
			CategoryID       *uint    `json:"category_id"`
			VotedOnly        bool     `json:"voted_only"`
			VotedRatio       *float64 `json:"voted_ratio"`
			CoversOnly       bool     `json:"covers_only"`
			VideoSyncEnabled bool     `json:"video_sync_enabled"`
		}

		if err := c.ShouldBindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		// Validate tree size
		validSizes := map[int]bool{8: true, 16: true, 32: true, 64: true, 128: true}
		if !validSizes[requestBody.TreeSize] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tree size "})
			return
		}

		// Generate unique room code
		roomID := generateTournamentRoomCode()

		// Select songs for the tournament
		songs, err := selectTournamentSongs(db, userID.(uint), requestBody.TreeSize, requestBody.CategoryID, requestBody.VotedOnly, requestBody.VotedRatio, requestBody.CoversOnly)
		if err != nil {
			log.Printf("Error selecting tournament songs: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to select songs: %v", err)})
			return
		}

		if len(songs) < requestBody.TreeSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Not enough songs available (found %d, need %d)", len(songs), requestBody.TreeSize)})
			return
		}

		// Generate tournament tree
		treeState := generateTournamentTree(db, songs, requestBody.TreeSize)

		// Create room in database
		room := models.TournamentRoom{
			RoomID:           roomID,
			CreatorID:        userID.(uint),
			TreeSize:         requestBody.TreeSize,
			CategoryID:       requestBody.CategoryID,
			VotedOnly:        requestBody.VotedOnly,
			VotedRatio:       requestBody.VotedRatio,
			CoversOnly:       requestBody.CoversOnly,
			VideoSyncEnabled: requestBody.VideoSyncEnabled,
			TreeState:        treeState,
			Status:           "setup",
			CreatedAt:        time.Now(),
			LastActive:       time.Now(),
		}

		if err := db.Create(&room).Error; err != nil {
			log.Printf("Error creating tournament room: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create tournament"})
			return
		}

		// Get username for room manager
		username, _ := c.Get("username")
		usernameStr := ""
		if username != nil {
			usernameStr = username.(string)
		}

		// Create room in memory
		tournamentRoomManager.CreateRoom(roomID, fmt.Sprintf("%d", userID.(uint)), usernameStr)

		c.JSON(http.StatusOK, gin.H{
			"room_id": roomID,
			"url":     fmt.Sprintf("/tournament-room/%s", roomID),
		})
	}
}

// selectTournamentSongs selects songs for the tournament based on filters
func selectTournamentSongs(db *gorm.DB, userID uint, count int, categoryID *uint, votedOnly bool, votedRatio *float64, coversOnly bool) ([]models.Song, error) {
	var songs []models.Song

	// Build base query
	baseQuery := db.Preload("Artists").Preload("Units").Preload("Category")

	// Apply category filter
	if categoryID != nil {
		baseQuery = baseQuery.Where("category_id = ?", *categoryID)
	}

	// Apply covers filter
	if coversOnly {
		baseQuery = baseQuery.Where("is_cover = ?", true)
	}

	if votedOnly {
		// Only voted songs
		err := baseQuery.
			Joins("INNER JOIN votes ON votes.song_id = songs.song_id AND votes.user_id = ?", userID).
			Order("RANDOM()").
			Limit(count).
			Find(&songs).Error

		// If not enough voted songs, fill with unvoted
		if len(songs) < count {
			remaining := count - len(songs)
			var unvotedSongs []models.Song

			baseQuery.
				Where("song_id NOT IN (?)", db.Table("votes").Select("song_id").Where("user_id = ?", userID)).
				Order("RANDOM()").
				Limit(remaining).
				Find(&unvotedSongs)

			songs = append(songs, unvotedSongs...)
		}

		return songs, err
	}

	// Mix of voted and unvoted songs (best effort)
	if votedRatio != nil && *votedRatio > 0 && *votedRatio < 1 {
		votedCount := int(float64(count) * (*votedRatio))
		unvotedCount := count - votedCount

		// Get voted songs (up to desired count)
		var votedSongs []models.Song
		votedQuery := db.Model(&models.Song{}).
			Preload("Artists").Preload("Units").Preload("Category").
			Joins("INNER JOIN votes ON votes.song_id = songs.song_id AND votes.user_id = ?", userID)

		// Apply same filters to voted query
		if categoryID != nil {
			votedQuery = votedQuery.Where("songs.category_id = ?", *categoryID)
		}
		if coversOnly {
			votedQuery = votedQuery.Where("songs.is_cover = ?", true)
		}

		votedQuery.Order("RANDOM()").Limit(votedCount).Find(&votedSongs)

		// Get unvoted songs (up to desired count)
		var unvotedSongs []models.Song
		unvotedQuery := db.Model(&models.Song{}).
			Preload("Artists").Preload("Units").Preload("Category").
			Where("song_id NOT IN (?)", db.Table("votes").Select("song_id").Where("user_id = ?", userID))

		// Apply same filters to unvoted query
		if categoryID != nil {
			unvotedQuery = unvotedQuery.Where("category_id = ?", *categoryID)
		}
		if coversOnly {
			unvotedQuery = unvotedQuery.Where("is_cover = ?", true)
		}

		unvotedQuery.Order("RANDOM()").Limit(unvotedCount).Find(&unvotedSongs)

		// Combine what we found
		songs = append(votedSongs, unvotedSongs...)

		// If we don't have enough songs yet, fill with whatever is available
		if len(songs) < count {
			remaining := count - len(songs)

			// Get song IDs we already have
			existingIDs := make([]uint, len(songs))
			for i, s := range songs {
				existingIDs[i] = s.SongID
			}

			// Get more songs that we haven't selected yet
			var additionalSongs []models.Song
			fillQuery := db.Model(&models.Song{}).
				Preload("Artists").Preload("Units").Preload("Category").
				Where("song_id NOT IN (?)", existingIDs)

			// Apply same filters
			if categoryID != nil {
				fillQuery = fillQuery.Where("category_id = ?", *categoryID)
			}
			if coversOnly {
				fillQuery = fillQuery.Where("is_cover = ?", true)
			}

			fillQuery.Order("RANDOM()").Limit(remaining).Find(&additionalSongs)
			songs = append(songs, additionalSongs...)
		}

		return songs, nil
	}

	// All songs (no ratio preference)
	err := baseQuery.Order("RANDOM()").Limit(count).Find(&songs).Error
	return songs, err
}

// generateTournamentTree creates the tournament bracket structure
func generateTournamentTree(db *gorm.DB, songs []models.Song, treeSize int) models.TreeState {
	// Calculate number of rounds
	numRounds := int(math.Log2(float64(treeSize)))

	treeState := models.TreeState{
		Rounds: make([]models.Round, numRounds),
	}

	// Shuffle songs for randomness
	mathrand.Seed(time.Now().UnixNano())
	mathrand.Shuffle(len(songs), func(i, j int) {
		songs[i], songs[j] = songs[j], songs[i]
	})

	// Generate first round matches
	firstRoundMatchCount := treeSize / 2
	treeState.Rounds[0] = models.Round{
		RoundNumber: 1,
		Matches:     make([]models.Match, firstRoundMatchCount),
	}

	for i := 0; i < firstRoundMatchCount; i++ {
		song1 := songs[i*2]
		song2 := songs[i*2+1]

		// Get average ratings from database
		avgRating1 := getAverageSongRatingFromDB(db, song1.SongID)
		avgRating2 := getAverageSongRatingFromDB(db, song2.SongID)

		treeState.Rounds[0].Matches[i] = models.Match{
			MatchID: fmt.Sprintf("r1m%d", i+1),
			Song1: &models.MatchSong{
				SongID:           &song1.SongID,
				SongTitle:        song1.NameOriginal,
				SongTitleEnglish: song1.NameEnglish,
				Artists:          formatArtistNames(song1.Artists),
				ThumbnailURL:     song1.ThumbnailURL,
				SourceURL:        song1.SourceURL,
				EmbedURL:         getEmbedURL(song1.SourceURL),
				AverageRating:    avgRating1,
				CategoryName:     getCategoryName(song1.Category),
				IsCover:          song1.IsCover,
			},
			Song2: &models.MatchSong{
				SongID:           &song2.SongID,
				SongTitle:        song2.NameOriginal,
				SongTitleEnglish: song2.NameEnglish,
				Artists:          formatArtistNames(song2.Artists),
				ThumbnailURL:     song2.ThumbnailURL,
				SourceURL:        song2.SourceURL,
				EmbedURL:         getEmbedURL(song2.SourceURL),
				AverageRating:    avgRating2,
				CategoryName:     getCategoryName(song2.Category),
				IsCover:          song2.IsCover,
			},
			Status:    "pending",
			UserPicks: []models.UserPick{},
		}
	}

	// Generate subsequent rounds (empty for now, will be filled as tournament progresses)
	for r := 1; r < numRounds; r++ {
		matchCount := treeSize / int(math.Pow(2, float64(r+1)))
		treeState.Rounds[r] = models.Round{
			RoundNumber: r + 1,
			Matches:     make([]models.Match, matchCount),
		}

		for m := 0; m < matchCount; m++ {
			// Calculate which previous matches feed into this match
			prevMatch1ID := fmt.Sprintf("r%dm%d", r, m*2+1)
			prevMatch2ID := fmt.Sprintf("r%dm%d", r, m*2+2)

			treeState.Rounds[r].Matches[m] = models.Match{
				MatchID: fmt.Sprintf("r%dm%d", r+1, m+1),
				Song1: &models.MatchSong{
					FromMatchID: &prevMatch1ID,
					SongTitle:   "Winner of " + prevMatch1ID,
				},
				Song2: &models.MatchSong{
					FromMatchID: &prevMatch2ID,
					SongTitle:   "Winner of " + prevMatch2ID,
				},
				Status:    "pending",
				UserPicks: []models.UserPick{},
			}
		}
	}

	return treeState
}

// Helper functions

func getCategoryName(category *models.Category) string {
	if category == nil {
		return ""
	}
	return category.Name
}

func formatArtistNames(artists []models.Artist) string {
	if len(artists) == 0 {
		return "Unknown Artist"
	}
	names := make([]string, len(artists))
	for i, artist := range artists {
		if artist.NameEnglish != "" {
			names[i] = artist.NameEnglish
		} else {
			names[i] = artist.NameOriginal
		}
	}
	result := ""
	for i, name := range names {
		if i > 0 {
			result += ", "
		}
		result += name
	}
	return result
}

func getEmbedURL(sourceURL string) string {
	if utils.IsYouTubeURL(sourceURL) {
		if embedURL, err := utils.GetYouTubeEmbedURL(sourceURL); err == nil {
			return embedURL
		}
	}
	return ""
}

func getAverageSongRating(songID uint) float64 {
	// This will be called during tree generation, but we don't have db access here
	// The rating will be properly set during tree generation where we have db access
	return 0.0
}

func getAverageSongRatingFromDB(db *gorm.DB, songID uint) float64 {
	var avgRating struct {
		Average float64
	}

	err := db.Table("votes").
		Select("AVG(rating) as average").
		Where("song_id = ?", songID).
		Scan(&avgRating).Error

	if err != nil {
		return 0.0
	}

	return avgRating.Average
}

func generateTournamentRoomCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

// GetTournamentRoom shows the tournament room interface
func GetTournamentRoom(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		roomID := c.Param("roomId")

		// Check if user is authenticated
		userID, exists := c.Get("user_id")
		if !exists || userID == nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}

		// Check if room exists in database
		var room models.TournamentRoom
		if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.HTML(http.StatusNotFound, "error.html", gin.H{
					"title": "SyncRate | Room Not Found",
					"error": "Tournament room not found",
				})
				return
			}
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Failed to load tournament",
			})
			return
		}

		templateData := GetUserContext(c)
		templateData["title"] = fmt.Sprintf("SyncRate | Tournament %s", roomID)
		templateData["room"] = room
		templateData["room_id"] = roomID

		c.HTML(http.StatusOK, "tournament-room.html", templateData)
	}
}

// GetTournamentRoomWS handles WebSocket connections for tournament rooms
func GetTournamentRoomWS(db *gorm.DB) gin.HandlerFunc {
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
		if err := tournamentRoomManager.JoinRoom(roomID, userIDStr, usernameStr, conn); err != nil {
			log.Printf("Error joining tournament room: %v", err)
			conn.WriteJSON(map[string]interface{}{
				"type":  "error",
				"error": err.Error(),
			})
			return
		}

		// Handle connection
		handleTournamentConnection(db, roomID, userIDStr, conn)

		// Clean up when connection closes
		tournamentRoomManager.LeaveRoom(userIDStr)

		// Broadcast updated user list
		broadcastTournamentUserUpdate(roomID)
	}
}

// handleTournamentConnection manages the WebSocket connection for a tournament room
func handleTournamentConnection(db *gorm.DB, roomID, userID string, conn *websocket.Conn) {
	// Send initial tournament state
	sendTournamentState(db, roomID, conn)

	// Broadcast user update to all participants
	broadcastTournamentUserUpdate(roomID)

	// Listen for messages
	for {
		var msg wsocket.WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		handleTournamentMessage(db, roomID, userID, msg, conn)
	}
}

// sendTournamentState sends the current tournament state to a client
func sendTournamentState(db *gorm.DB, roomID string, conn *websocket.Conn) {
	var room models.TournamentRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return
	}

	// Load existing votes for all tournament songs
	existingVotes := loadExistingVotesForTournament(db, &room.TreeState)

	// Send tournament state
	stateData, _ := json.Marshal(map[string]interface{}{
		"tree_state":         room.TreeState,
		"status":             room.Status,
		"current_match_id":   room.CurrentMatchID,
		"video_sync_enabled": room.VideoSyncEnabled,
		"existing_votes":     existingVotes,
	})

	message := wsocket.WSMessage{
		Type:      "tournament_state",
		Data:      stateData,
		Timestamp: time.Now(),
	}

	conn.WriteJSON(message)
}

// handleTournamentMessage processes incoming WebSocket messages
func handleTournamentMessage(db *gorm.DB, roomID, userID string, msg wsocket.WSMessage, conn *websocket.Conn) {
	// Update last_active timestamp
	db.Model(&models.TournamentRoom{}).
		Where("room_id = ?", roomID).
		Update("last_active", time.Now())

	switch msg.Type {
	case "start_tournament":
		handleStartTournament(db, roomID)
	case "start_match":
		handleStartMatch(db, roomID, msg.Data)
	case "pick_winner":
		handlePickWinner(db, roomID, userID, msg.Data)
	case "navigate_match":
		// Broadcast match navigation to all clients for synchronized navigation
		tournamentRoomManager.BroadcastToRoom(roomID, msg)
	case wsocket.MsgVideoSync:
		tournamentRoomManager.BroadcastToRoom(roomID, msg)
	case wsocket.MsgVoteUpdate:
		handleTournamentVoteUpdate(db, roomID, userID, msg.Data)
	default:
		log.Printf("Unknown tournament message type: %s", msg.Type)
	}
}

func handleStartTournament(db *gorm.DB, roomID string) {
	// Set status to in_progress and set first match as current
	firstMatchID := "r1m1"

	err := db.Model(&models.TournamentRoom{}).
		Where("room_id = ?", roomID).
		Updates(map[string]interface{}{
			"status":           "in_progress",
			"current_match_id": firstMatchID,
			"last_active":      time.Now(),
		}).Error

	if err != nil {
		log.Printf("Error starting tournament: %v", err)
		return
	}

	// Broadcast updated state (clients will auto-open first match on status change)
	broadcastTournamentState(db, roomID)
	log.Printf("Tournament started in room %s, broadcasting state update", roomID)
}

func handleStartMatch(db *gorm.DB, roomID string, data json.RawMessage) {
	var matchData struct {
		MatchID string `json:"match_id"`
	}

	if err := json.Unmarshal(data, &matchData); err != nil {
		log.Printf("Error unmarshaling match data: %v", err)
		return
	}

	// Update current match
	db.Model(&models.TournamentRoom{}).
		Where("room_id = ?", roomID).
		Updates(map[string]interface{}{
			"current_match_id": matchData.MatchID,
			"last_active":      time.Now(),
		})

	broadcastTournamentState(db, roomID)
}

func handlePickWinner(db *gorm.DB, roomID string, userID string, data json.RawMessage) {
	var pickData struct {
		MatchID string `json:"match_id"`
		SongID  uint   `json:"song_id"`
	}

	if err := json.Unmarshal(data, &pickData); err != nil {
		log.Printf("Error unmarshaling pick data: %v", err)
		return
	}

	// Get the tournament room
	var room models.TournamentRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		log.Printf("Error finding tournament room: %v", err)
		return
	}

	// Find the match in the tree
	match := findMatchInTree(&room.TreeState, pickData.MatchID)
	if match == nil {
		log.Printf("Match not found: %s", pickData.MatchID)
		return
	}

	// Get username
	tournRoom, exists := tournamentRoomManager.GetRoom(roomID)
	if !exists {
		return
	}

	tournRoom.Mutex.RLock()
	client, exists := tournRoom.Clients[userID]
	username := ""
	if exists {
		username = client.Username
	}
	totalUsers := len(tournRoom.Clients)
	tournRoom.Mutex.RUnlock()

	// Add or update user's pick
	pickedSongID := pickData.SongID
	pickExists := false
	for i, pick := range match.UserPicks {
		if pick.UserID == userID {
			match.UserPicks[i].PickedSongID = &pickedSongID
			match.UserPicks[i].PickedAt = time.Now()
			pickExists = true
			break
		}
	}

	if !pickExists {
		match.UserPicks = append(match.UserPicks, models.UserPick{
			UserID:       userID,
			Username:     username,
			PickedSongID: &pickedSongID,
			PickedAt:     time.Now(),
		})
	}

	// Check if all users have picked
	if len(match.UserPicks) >= totalUsers {
		// Update average ratings with current values from database before determining winner
		if match.Song1 != nil && match.Song1.SongID != nil {
			match.Song1.AverageRating = getAverageSongRatingFromDB(db, *match.Song1.SongID)
		}
		if match.Song2 != nil && match.Song2.SongID != nil {
			match.Song2.AverageRating = getAverageSongRatingFromDB(db, *match.Song2.SongID)
		}

		// Determine winner (pass db to recalculate ratings)
		winner := determineMatchWinner(db, match)
		match.Winner = winner
		match.Status = "completed"
		now := time.Now()
		match.CompletedAt = &now

		// Advance winner to next round if applicable
		advanceWinner(&room.TreeState, pickData.MatchID, winner)
	}

	// Save updated tree state
	db.Model(&models.TournamentRoom{}).
		Where("room_id = ?", roomID).
		Updates(map[string]interface{}{
			"tree_state":  room.TreeState,
			"last_active": time.Now(),
		})

	// Broadcast updated state
	broadcastTournamentState(db, roomID)
}

func findMatchInTree(tree *models.TreeState, matchID string) *models.Match {
	for r := range tree.Rounds {
		for m := range tree.Rounds[r].Matches {
			if tree.Rounds[r].Matches[m].MatchID == matchID {
				return &tree.Rounds[r].Matches[m]
			}
		}
	}
	return nil
}

func determineMatchWinner(db *gorm.DB, match *models.Match) *models.MatchSong {
	// Count picks for each song
	song1Picks := 0
	song2Picks := 0

	for _, pick := range match.UserPicks {
		if pick.PickedSongID != nil {
			if match.Song1 != nil && match.Song1.SongID != nil && *pick.PickedSongID == *match.Song1.SongID {
				song1Picks++
			} else if match.Song2 != nil && match.Song2.SongID != nil && *pick.PickedSongID == *match.Song2.SongID {
				song2Picks++
			}
		}
	}

	// Determine winner by picks
	if song1Picks > song2Picks {
		return match.Song1
	} else if song2Picks > song1Picks {
		return match.Song2
	}

	// Tie: recalculate average ratings from database to get latest votes
	var currentRating1, currentRating2 float64
	if match.Song1 != nil && match.Song1.SongID != nil {
		currentRating1 = getAverageSongRatingFromDB(db, *match.Song1.SongID)
	}
	if match.Song2 != nil && match.Song2.SongID != nil {
		currentRating2 = getAverageSongRatingFromDB(db, *match.Song2.SongID)
	}

	if currentRating1 > currentRating2 {
		return match.Song1
	} else if currentRating2 > currentRating1 {
		return match.Song2
	}

	// Still tied: coin toss
	mathrand.Seed(time.Now().UnixNano())
	if mathrand.Intn(2) == 0 {
		return match.Song1
	}
	return match.Song2
}

func advanceWinner(tree *models.TreeState, completedMatchID string, winner *models.MatchSong) {
	// Find which match in the next round this winner should go to
	// Parse match ID (e.g., "r1m3" -> round 1, match 3)
	var round, matchNum int
	fmt.Sscanf(completedMatchID, "r%dm%d", &round, &matchNum)

	// Determine next round and match position
	nextRound := round + 1
	if nextRound > len(tree.Rounds) {
		// Tournament complete!
		return
	}

	// Calculate which match in the next round
	nextMatchNum := (matchNum + 1) / 2

	if nextMatchNum > 0 && nextMatchNum <= len(tree.Rounds[nextRound-1].Matches) {
		nextMatch := &tree.Rounds[nextRound-1].Matches[nextMatchNum-1]

		// Determine if winner goes to song1 or song2 position
		if matchNum%2 == 1 {
			nextMatch.Song1 = winner
		} else {
			nextMatch.Song2 = winner
		}
	}
}

func broadcastTournamentState(db *gorm.DB, roomID string) {
	var room models.TournamentRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return
	}

	// Load existing votes for all tournament songs
	existingVotes := loadExistingVotesForTournament(db, &room.TreeState)

	stateData, _ := json.Marshal(map[string]interface{}{
		"tree_state":         room.TreeState,
		"status":             room.Status,
		"current_match_id":   room.CurrentMatchID,
		"video_sync_enabled": room.VideoSyncEnabled,
		"existing_votes":     existingVotes,
	})

	message := wsocket.WSMessage{
		Type:      "tournament_state",
		Data:      stateData,
		Timestamp: time.Now(),
	}

	tournamentRoomManager.BroadcastToRoom(roomID, message)
}

func handleTournamentVoteUpdate(db *gorm.DB, roomID, userID string, data json.RawMessage) {
	// Broadcast vote update for rating songs during matches
	tournamentRoomManager.BroadcastToRoom(roomID, wsocket.WSMessage{
		Type:      wsocket.MsgVoteUpdate,
		Data:      data,
		Timestamp: time.Now(),
	})
}

func broadcastTournamentUserUpdate(roomID string) {
	room, exists := tournamentRoomManager.GetRoom(roomID)
	if !exists {
		return
	}

	room.Mutex.RLock()
	users := make([]wsocket.UserInfo, 0, len(room.Clients))
	for _, client := range room.Clients {
		users = append(users, wsocket.UserInfo{
			ID:       client.ID,
			Username: client.Username,
		})
	}
	room.Mutex.RUnlock()

	data, _ := json.Marshal(wsocket.UserUpdateData{Users: users})
	message := wsocket.WSMessage{
		Type:      wsocket.MsgUserUpdate,
		Data:      data,
		Timestamp: time.Now(),
	}

	tournamentRoomManager.BroadcastToRoom(roomID, message)
}

// loadExistingVotesForTournament loads existing votes for all songs in the tournament
func loadExistingVotesForTournament(db *gorm.DB, treeState *models.TreeState) map[uint][]wsocket.VoteUpdateData {
	if treeState == nil {
		return make(map[uint][]wsocket.VoteUpdateData)
	}

	result := make(map[uint][]wsocket.VoteUpdateData)

	// Collect all unique song IDs from all matches
	songIDsMap := make(map[uint]bool)
	for _, round := range treeState.Rounds {
		for _, match := range round.Matches {
			if match.Song1 != nil && match.Song1.SongID != nil {
				songIDsMap[*match.Song1.SongID] = true
			}
			if match.Song2 != nil && match.Song2.SongID != nil {
				songIDsMap[*match.Song2.SongID] = true
			}
		}
	}

	// Convert map keys to slice
	songIDs := make([]uint, 0, len(songIDsMap))
	for songID := range songIDsMap {
		songIDs = append(songIDs, songID)
	}

	if len(songIDs) == 0 {
		return result
	}

	// Load all votes for these songs
	var votes []models.Vote
	db.Where("song_id IN ?", songIDs).Find(&votes)

	// Get usernames for the votes
	userIDs := make([]uint, 0, len(votes))
	for _, vote := range votes {
		userIDs = append(userIDs, vote.UserID)
	}

	usernames := make(map[uint]string)
	if len(userIDs) > 0 {
		var users []models.User
		db.Where("user_id IN ?", userIDs).Find(&users)
		for _, user := range users {
			usernames[user.UserID] = user.Username
		}
	}

	// Group votes by song ID
	for _, vote := range votes {
		voteData := wsocket.VoteUpdateData{
			UserID:   fmt.Sprintf("%d", vote.UserID),
			Username: usernames[vote.UserID],
			Rating:   vote.Rating,
			Comment:  vote.Comment,
		}
		result[vote.SongID] = append(result[vote.SongID], voteData)
	}

	return result
}
