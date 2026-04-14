package handlers

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math"
	mathrand "math/rand"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/CptPie/SyncRate/models"
	wsocket "github.com/CptPie/SyncRate/server/websocket"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
	"golang.org/x/text/unicode/norm"
)

var (
	quizRoomManager = wsocket.NewRoomManager()
)

// StartQuizDatabaseCleanup starts a background routine to clean up old quiz rooms
func StartQuizDatabaseCleanup(db *gorm.DB) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cleanupOldQuizRooms(db, 48*time.Hour)
			}
		}
	}()
}

func cleanupOldQuizRooms(db *gorm.DB, inactivityThreshold time.Duration) {
	cutoffTime := time.Now().Add(-inactivityThreshold)

	result := db.Where("last_active < ?", cutoffTime).Delete(&models.QuizRoom{})
	if result.Error != nil {
		log.Printf("Error cleaning up old quiz rooms: %v", result.Error)
		return
	}

	if result.RowsAffected > 0 {
		log.Printf("Cleaned up %d inactive quiz rooms from database", result.RowsAffected)
	}
}

// GetCreateQuizRoom shows the quiz room creation page
func GetCreateQuizRoom(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if userID, exists := c.Get("user_id"); !exists || userID == nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}

		var categories []models.Category
		db.Find(&categories)

		templateData := GetUserContext(c)
		templateData["title"] = "SyncRate | Create Quiz"
		templateData["categories"] = categories

		c.HTML(http.StatusOK, "create-quiz-room.html", templateData)
	}
}

// PostCreateQuizRoom creates a new quiz room
func PostCreateQuizRoom(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists || userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		var requestBody struct {
			MaxRounds      *int     `json:"max_rounds"`
			QuestionType   string   `json:"question_type"`
			QuizStyle      string   `json:"quiz_style"`
			FixedTimeLimit int      `json:"fixed_time_limit"`
			CategoryID     *uint    `json:"category_id"`
			VotedOnly      bool     `json:"voted_only"`
			VotedRatio     *float64 `json:"voted_ratio"`
			CoversOnly     bool     `json:"covers_only"`
			FuzzyInput     bool     `json:"fuzzy_input"`
			OneOfArtists   bool     `json:"one_of_artists"`
		}

		if err := c.ShouldBindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		// Validate question type
		validTypes := map[string]bool{"title": true, "artist": true, "both": true}
		if !validTypes[requestBody.QuestionType] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid question type"})
			return
		}

		// Validate quiz style
		if requestBody.QuizStyle != "fixed" && requestBody.QuizStyle != "progressive" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid quiz style"})
			return
		}

		// Validate fixed time limit
		if requestBody.QuizStyle == "fixed" {
			validTimes := map[int]bool{1: true, 5: true, 10: true, 30: true, 60: true}
			if !validTimes[requestBody.FixedTimeLimit] {
				requestBody.FixedTimeLimit = 30
			}
		}

		// Check song availability and generate warnings
		songWarning := checkQuizSongAvailability(db, userID.(uint), requestBody.CategoryID, requestBody.VotedOnly, requestBody.VotedRatio, requestBody.CoversOnly, requestBody.MaxRounds)

		roomID := generateQuizRoomCode()

		room := models.QuizRoom{
			RoomID:         roomID,
			CreatorID:      userID.(uint),
			MaxRounds:      requestBody.MaxRounds,
			QuestionType:   requestBody.QuestionType,
			QuizStyle:      requestBody.QuizStyle,
			FixedTimeLimit: requestBody.FixedTimeLimit,
			CategoryID:     requestBody.CategoryID,
			VotedOnly:      requestBody.VotedOnly,
			VotedRatio:     requestBody.VotedRatio,
			CoversOnly:     requestBody.CoversOnly,
			FuzzyInput:     requestBody.FuzzyInput,
			OneOfArtists:   requestBody.OneOfArtists,
			QuizState: models.QuizState{
				Rounds:       []models.QuizRound{},
				Scores:       []models.PlayerScore{},
				CurrentRound: 0,
				CurrentStage: 0,
			},
			Status:     "setup",
			CreatedAt:  time.Now(),
			LastActive: time.Now(),
		}

		if err := db.Create(&room).Error; err != nil {
			log.Printf("Error creating quiz room: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create quiz room"})
			return
		}

		username, _ := c.Get("username")
		usernameStr := ""
		if username != nil {
			usernameStr = username.(string)
		}

		quizRoomManager.CreateRoom(roomID, fmt.Sprintf("%d", userID.(uint)), usernameStr)

		response := gin.H{
			"room_id": roomID,
			"url":     fmt.Sprintf("/quiz-room/%s", roomID),
		}
		if songWarning != "" {
			response["warning"] = songWarning
		}
		c.JSON(http.StatusOK, response)
	}
}

// GetQuizRoom shows the quiz room interface
func GetQuizRoom(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		roomID := c.Param("roomId")

		userID, exists := c.Get("user_id")
		if !exists || userID == nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}

		var room models.QuizRoom
		if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.HTML(http.StatusNotFound, "error.html", gin.H{
					"title": "SyncRate | Room Not Found",
					"error": "Quiz room not found",
				})
				return
			}
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Failed to load quiz room",
			})
			return
		}

		// Load songs and artists for fuzzy search if fuzzy input is enabled
		songsJSON := "[]"
		artistsJSON := "[]"
		unitsJSON := "[]"
		if room.FuzzyInput {
			var songs []models.Song
			query := db.Model(&models.Song{})
			if room.CategoryID != nil {
				query = query.Where("category_id = ?", *room.CategoryID)
			}
			if room.CoversOnly {
				query = query.Where("is_cover = ?", true)
			}
			query.Find(&songs)

			var artists []models.Artist
			db.Find(&artists)

			var units []models.Unit
			db.Find(&units)

			if b, err := json.Marshal(songs); err == nil {
				songsJSON = string(b)
			}
			if b, err := json.Marshal(artists); err == nil {
				artistsJSON = string(b)
			}
			if b, err := json.Marshal(units); err == nil {
				unitsJSON = string(b)
			}
		}

		templateData := GetUserContext(c)
		templateData["title"] = fmt.Sprintf("SyncRate | Quiz %s", roomID)
		templateData["room"] = room
		templateData["room_id"] = roomID
		templateData["songsJSON"] = songsJSON
		templateData["artistsJSON"] = artistsJSON
		templateData["unitsJSON"] = unitsJSON

		c.HTML(http.StatusOK, "quiz-room.html", templateData)
	}
}

// GetQuizRoomWS handles WebSocket connections for quiz rooms
func GetQuizRoomWS(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		roomID := c.Param("roomId")

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

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}
		defer conn.Close()

		userIDStr := fmt.Sprintf("%d", userID.(uint))
		if err := quizRoomManager.JoinRoom(roomID, userIDStr, usernameStr, conn); err != nil {
			log.Printf("Error joining quiz room: %v", err)
			conn.WriteJSON(map[string]interface{}{
				"type":  "error",
				"error": err.Error(),
			})
			return
		}

		handleQuizConnection(db, roomID, userIDStr, conn)

		quizRoomManager.LeaveRoom(userIDStr)
		broadcastQuizUserUpdate(roomID)
	}
}

func handleQuizConnection(db *gorm.DB, roomID, userID string, conn *websocket.Conn) {
	sendQuizState(db, roomID, userID)
	broadcastQuizUserUpdate(roomID)

	for {
		var msg wsocket.WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("Quiz WebSocket read error: %v", err)
			break
		}

		handleQuizMessage(db, roomID, userID, msg, conn)
	}
}

func sendQuizState(db *gorm.DB, roomID string, userID string) {
	var room models.QuizRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return
	}

	// Build state to send — hide correct answers if round is active
	stateToSend := buildQuizStateForClient(room, userID)

	stateData, _ := json.Marshal(stateToSend)

	message := wsocket.WSMessage{
		Type:      "quiz_state",
		Data:      stateData,
		Timestamp: time.Now(),
	}

	quizRoomManager.SendToClient(userID, message)
}

// buildQuizStateForClient creates a sanitized state (hides answers for active rounds)
func buildQuizStateForClient(room models.QuizRoom, userID string) map[string]interface{} {
	sanitizedRounds := make([]map[string]interface{}, len(room.QuizState.Rounds))

	for i, round := range room.QuizState.Rounds {
		roundData := map[string]interface{}{
			"round_number":     round.RoundNumber,
			"audio_start_time": round.AudioStartTime,
			"user_guesses":     round.UserGuesses,
			"status":           round.Status,
			"completed_at":     round.CompletedAt,
		}

		if round.Status == "completed" {
			roundData["song"] = round.Song
		} else {
			// Only send embed URL for audio playback, hide answer details
			roundData["song"] = map[string]interface{}{
				"embed_url":  round.Song.EmbedURL,
				"song_id":    round.Song.SongID,
				"source_url": round.Song.SourceURL,
			}
		}

		sanitizedRounds[i] = roundData
	}

	return map[string]interface{}{
		"status":           room.Status,
		"question_type":    room.QuestionType,
		"quiz_style":       room.QuizStyle,
		"fixed_time_limit": room.FixedTimeLimit,
		"fuzzy_input":      room.FuzzyInput,
		"one_of_artists":   room.OneOfArtists,
		"max_rounds":       room.MaxRounds,
		"rounds":           sanitizedRounds,
		"scores":           room.QuizState.Scores,
		"current_round":    room.QuizState.CurrentRound,
		"current_stage":    room.QuizState.CurrentStage,
		"stage_ready":      room.QuizState.StageReady,
	}
}

func handleQuizMessage(db *gorm.DB, roomID, userID string, msg wsocket.WSMessage, conn *websocket.Conn) {
	db.Model(&models.QuizRoom{}).
		Where("room_id = ?", roomID).
		Update("last_active", time.Now())

	switch msg.Type {
	case "start_quiz":
		handleStartQuiz(db, roomID)
	case "next_round":
		handleNextRound(db, roomID)
	case "submit_guess":
		handleSubmitGuess(db, roomID, userID, msg.Data)
	case "advance_stage":
		handleAdvanceStage(db, roomID)
	case "set_ready":
		handleSetReady(db, roomID, userID, msg.Data)
	case "complete_round":
		handleCompleteRound(db, roomID)
	case wsocket.MsgVoteUpdate:
		handleQuizVoteUpdate(db, roomID, userID, msg.Data)
	default:
		log.Printf("Unknown quiz message type: %s", msg.Type)
	}
}

func handleStartQuiz(db *gorm.DB, roomID string) {
	err := db.Model(&models.QuizRoom{}).
		Where("room_id = ?", roomID).
		Updates(map[string]interface{}{
			"status":     "in_progress",
			"last_active": time.Now(),
		}).Error

	if err != nil {
		log.Printf("Error starting quiz: %v", err)
		return
	}

	// Immediately start the first round
	handleNextRound(db, roomID)
}

func handleNextRound(db *gorm.DB, roomID string) {
	var room models.QuizRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		log.Printf("Error finding quiz room: %v", err)
		return
	}

	// Check if max rounds reached
	if room.MaxRounds != nil && room.QuizState.CurrentRound >= *room.MaxRounds {
		// Quiz is done, broadcast final state
		broadcastQuizState(db, roomID)
		return
	}

	// Select a random song for this round
	song := selectQuizSong(db, &room)
	if song == nil {
		log.Printf("No more songs available for quiz room %s", roomID)
		// Broadcast error
		errorData, _ := json.Marshal(map[string]string{"error": "No more songs available"})
		quizRoomManager.BroadcastToRoom(roomID, wsocket.WSMessage{
			Type:      "error",
			Data:      errorData,
			Timestamp: time.Now(),
		})
		return
	}

	// Build quiz song data
	quizSong := buildQuizSong(db, *song)

	// Determine audio start time (skip first 10s and last 10s)
	audioStart := 10 // default fallback
	// We can't know the exact video duration without calling YouTube API,
	// so we'll pick a random time between 10 and 240 seconds (4 min mark)
	// The frontend will handle cases where this exceeds actual duration
	maxStart := 240
	if maxStart > 10 {
		audioStart = 10 + mathrand.Intn(maxStart-10)
	}

	newRound := models.QuizRound{
		RoundNumber:    room.QuizState.CurrentRound + 1,
		Song:           quizSong,
		AudioStartTime: audioStart,
		UserGuesses:    []models.UserGuess{},
		Status:         "playing",
	}

	room.QuizState.Rounds = append(room.QuizState.Rounds, newRound)
	room.QuizState.CurrentRound = newRound.RoundNumber
	room.QuizState.CurrentStage = 0
	room.QuizState.StageReady = map[string]bool{}

	db.Model(&models.QuizRoom{}).
		Where("room_id = ?", roomID).
		Updates(map[string]interface{}{
			"quiz_state":  room.QuizState,
			"last_active": time.Now(),
		})

	broadcastQuizState(db, roomID)
}

func handleSubmitGuess(db *gorm.DB, roomID, userID string, data json.RawMessage) {
	var guessData struct {
		TitleGuess  string `json:"title_guess"`
		ArtistGuess string `json:"artist_guess"`
	}

	if err := json.Unmarshal(data, &guessData); err != nil {
		log.Printf("Error unmarshaling guess data: %v", err)
		return
	}

	var room models.QuizRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return
	}

	if room.QuizState.CurrentRound == 0 || len(room.QuizState.Rounds) == 0 {
		return
	}

	currentRound := &room.QuizState.Rounds[room.QuizState.CurrentRound-1]
	if currentRound.Status != "playing" {
		return
	}

	// Get username
	quizRoom, exists := quizRoomManager.GetRoom(roomID)
	if !exists {
		return
	}

	quizRoom.Mutex.RLock()
	client, exists := quizRoom.Clients[userID]
	username := ""
	if exists {
		username = client.Username
	}
	quizRoom.Mutex.RUnlock()

	// Find or create user guess entry
	var guess *models.UserGuess
	for i, g := range currentRound.UserGuesses {
		if g.UserID == userID {
			guess = &currentRound.UserGuesses[i]
			break
		}
	}

	if guess == nil {
		now := time.Now()
		currentRound.UserGuesses = append(currentRound.UserGuesses, models.UserGuess{
			UserID:    userID,
			Username:  username,
			GuessedAt: &now,
		})
		guess = &currentRound.UserGuesses[len(currentRound.UserGuesses)-1]
	}

	currentStage := room.QuizState.CurrentStage
	currentStage1 := currentStage + 1 // 1-indexed for storage

	// Check title guess (only if not already correct, not already guessed this stage, and question type includes title)
	if !guess.TitleCorrect && guess.TitleGuessStage < currentStage1 && (room.QuestionType == "title" || room.QuestionType == "both") && guessData.TitleGuess != "" {
		guess.TitleGuessStage = currentStage1
		if matchesTitle(guessData.TitleGuess, currentRound.Song, room.FuzzyInput) {
			guess.TitleCorrect = true
			guess.TitleStage = currentStage1
			if room.QuizStyle == "progressive" {
				guess.TitlePoints = models.ProgressiveStages[currentStage].Points
			} else {
				guess.TitlePoints = 1
			}
		}
		guess.TitleGuess = guessData.TitleGuess
	}

	// Check artist guess (only if not already correct, not already guessed this stage, and question type includes artist)
	if !guess.ArtistCorrect && guess.ArtistGuessStage < currentStage1 && (room.QuestionType == "artist" || room.QuestionType == "both") && guessData.ArtistGuess != "" {
		guess.ArtistGuessStage = currentStage1
		if matchesArtist(guessData.ArtistGuess, currentRound.Song, room.OneOfArtists, room.FuzzyInput) {
			guess.ArtistCorrect = true
			guess.ArtistStage = currentStage1
			if room.QuizStyle == "progressive" {
				guess.ArtistPoints = models.ProgressiveStages[currentStage].Points
			} else {
				guess.ArtistPoints = 1
			}
		}
		guess.ArtistGuess = guessData.ArtistGuess
	}

	guess.TotalPoints = guess.TitlePoints + guess.ArtistPoints

	db.Model(&models.QuizRoom{}).
		Where("room_id = ?", roomID).
		Updates(map[string]interface{}{
			"quiz_state":  room.QuizState,
			"last_active": time.Now(),
		})

	// Broadcast guess event (without revealing correct answers)
	broadcastGuessUpdate(roomID, userID, username, guess)

	// Check if all users have guessed everything correctly
	if allUsersGuessedCorrectly(roomID, currentRound, room.QuestionType) {
		handleCompleteRound(db, roomID)
	} else {
		broadcastQuizState(db, roomID)
	}
}

func handleAdvanceStage(db *gorm.DB, roomID string) {
	var room models.QuizRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return
	}

	if room.QuizStyle != "progressive" {
		return
	}

	if room.QuizState.CurrentStage >= len(models.ProgressiveStages)-1 {
		// Already at last stage, complete the round
		handleCompleteRound(db, roomID)
		return
	}

	room.QuizState.CurrentStage++
	room.QuizState.StageReady = map[string]bool{}

	db.Model(&models.QuizRoom{}).
		Where("room_id = ?", roomID).
		Updates(map[string]interface{}{
			"quiz_state":  room.QuizState,
			"last_active": time.Now(),
		})

	broadcastQuizState(db, roomID)
}

func handleSetReady(db *gorm.DB, roomID, userID string, data json.RawMessage) {
	var payload struct {
		Ready bool `json:"ready"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return
	}

	var room models.QuizRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return
	}

	if room.QuizState.StageReady == nil {
		room.QuizState.StageReady = map[string]bool{}
	}
	if payload.Ready {
		room.QuizState.StageReady[userID] = true
	} else {
		delete(room.QuizState.StageReady, userID)
	}

	db.Model(&models.QuizRoom{}).
		Where("room_id = ?", roomID).
		Updates(map[string]interface{}{
			"quiz_state":  room.QuizState,
			"last_active": time.Now(),
		})

	broadcastQuizState(db, roomID)
}

func handleCompleteRound(db *gorm.DB, roomID string) {
	var room models.QuizRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return
	}

	if room.QuizState.CurrentRound == 0 || len(room.QuizState.Rounds) == 0 {
		return
	}

	currentRound := &room.QuizState.Rounds[room.QuizState.CurrentRound-1]
	if currentRound.Status == "completed" {
		return
	}

	now := time.Now()
	currentRound.Status = "completed"

	// Update cumulative scores for all users who guessed this round
	for _, guess := range currentRound.UserGuesses {
		updateQuizScores(&room.QuizState, guess.UserID, guess.Username)
	}
	currentRound.CompletedAt = &now

	// Load existing votes for the song
	existingVotes := loadQuizSongVotes(db, currentRound.Song.SongID)

	db.Model(&models.QuizRoom{}).
		Where("room_id = ?", roomID).
		Updates(map[string]interface{}{
			"quiz_state":  room.QuizState,
			"last_active": time.Now(),
		})

	// Broadcast round complete with full song info and votes
	roundCompleteData, _ := json.Marshal(map[string]interface{}{
		"type":           "round_complete",
		"round":          currentRound,
		"scores":         room.QuizState.Scores,
		"existing_votes": existingVotes,
	})

	quizRoomManager.BroadcastToRoom(roomID, wsocket.WSMessage{
		Type:      "round_complete",
		Data:      roundCompleteData,
		Timestamp: time.Now(),
	})

	// Also broadcast full quiz state
	broadcastQuizState(db, roomID)
}

func handleQuizVoteUpdate(db *gorm.DB, roomID, userID string, data json.RawMessage) {
	quizRoomManager.BroadcastToRoom(roomID, wsocket.WSMessage{
		Type:      wsocket.MsgVoteUpdate,
		Data:      data,
		Timestamp: time.Now(),
	})
}

// --- Song selection ---

// checkQuizSongAvailability counts available songs and warns about ratio shortfalls
func checkQuizSongAvailability(db *gorm.DB, userID uint, categoryID *uint, votedOnly bool, votedRatio *float64, coversOnly bool, maxRounds *int) string {
	// Count total available songs with filters
	countQuery := func(baseQ *gorm.DB) int64 {
		var count int64
		q := baseQ.Model(&models.Song{})
		if categoryID != nil {
			q = q.Where("category_id = ?", *categoryID)
		}
		if coversOnly {
			q = q.Where("is_cover = ?", true)
		}
		q.Count(&count)
		return count
	}

	// Count voted songs
	votedQuery := db.Where("song_id IN (?)", db.Table("votes").Select("song_id").Where("user_id = ?", userID))
	votedCount := countQuery(votedQuery)

	// Count unvoted songs
	unvotedQuery := db.Where("song_id NOT IN (?)", db.Table("votes").Select("song_id").Where("user_id = ?", userID))
	unvotedCount := countQuery(unvotedQuery)

	totalAvailable := votedCount + unvotedCount

	if totalAvailable == 0 {
		return "No songs available with the selected filters."
	}

	var warningParts []string

	if votedOnly {
		if votedCount == 0 {
			warningParts = append(warningParts, "no voted songs found, unvoted songs will be used instead")
		} else if maxRounds != nil && votedCount < int64(*maxRounds) {
			warningParts = append(warningParts, fmt.Sprintf("only %d voted songs available (need %d for all rounds), unvoted songs will fill remaining rounds", votedCount, *maxRounds))
		}
	} else if votedRatio != nil {
		ratio := *votedRatio
		if maxRounds != nil {
			needed := *maxRounds
			wantVoted := int64(float64(needed) * ratio)
			wantUnvoted := int64(needed) - wantVoted

			if wantVoted > 0 && votedCount < wantVoted {
				warningParts = append(warningParts, fmt.Sprintf("voted songs: found %d of %d requested", votedCount, wantVoted))
			}
			if wantUnvoted > 0 && unvotedCount < wantUnvoted {
				warningParts = append(warningParts, fmt.Sprintf("unvoted songs: found %d of %d requested", unvotedCount, wantUnvoted))
			}
		} else {
			// Endless mode — just warn if one category is empty when ratio expects it
			if ratio > 0 && votedCount == 0 {
				warningParts = append(warningParts, "no voted songs found, all songs will be unvoted")
			}
			if ratio < 1.0 && unvotedCount == 0 {
				warningParts = append(warningParts, "no unvoted songs found, all songs will be voted")
			}
		}
	}

	if len(warningParts) > 0 {
		return "Not enough songs for requested ratio. Shortfall in " + strings.Join(warningParts, "; ") + ". Remaining slots will be filled automatically."
	}

	return ""
}

func selectQuizSong(db *gorm.DB, room *models.QuizRoom) *models.Song {
	// Collect song IDs already used in this quiz
	usedSongIDs := make([]uint, len(room.QuizState.Rounds))
	for i, round := range room.QuizState.Rounds {
		usedSongIDs[i] = round.Song.SongID
	}

	// Build base filters (category, covers, already-used)
	applyBaseFilters := func(q *gorm.DB) *gorm.DB {
		q = q.Preload("Artists").Preload("Units").Preload("Category")
		if room.CategoryID != nil {
			q = q.Where("songs.category_id = ?", *room.CategoryID)
		}
		if room.CoversOnly {
			q = q.Where("songs.is_cover = ?", true)
		}
		if len(usedSongIDs) > 0 {
			q = q.Where("songs.song_id NOT IN ?", usedSongIDs)
		}
		return q
	}

	var song models.Song

	if room.VotedOnly {
		// Try voted songs first
		query := applyBaseFilters(db.Model(&models.Song{}))
		query = query.Joins("INNER JOIN votes ON votes.song_id = songs.song_id AND votes.user_id = ?", room.CreatorID)
		if err := query.Order("RANDOM()").First(&song).Error; err == nil {
			return &song
		}
		// Fall back to any song
		fallback := applyBaseFilters(db.Model(&models.Song{}))
		if err := fallback.Order("RANDOM()").First(&song).Error; err == nil {
			return &song
		}
		return nil
	}

	if room.VotedRatio != nil && *room.VotedRatio > 0 && *room.VotedRatio < 1.0 {
		// Use ratio to probabilistically pick voted vs unvoted
		ratio := *room.VotedRatio
		pickVoted := mathrand.Float64() < ratio

		if pickVoted {
			query := applyBaseFilters(db.Model(&models.Song{}))
			query = query.Joins("INNER JOIN votes ON votes.song_id = songs.song_id AND votes.user_id = ?", room.CreatorID)
			if err := query.Order("RANDOM()").First(&song).Error; err == nil {
				return &song
			}
		} else {
			query := applyBaseFilters(db.Model(&models.Song{}))
			query = query.Where("songs.song_id NOT IN (?)", db.Table("votes").Select("song_id").Where("user_id = ?", room.CreatorID))
			if err := query.Order("RANDOM()").First(&song).Error; err == nil {
				return &song
			}
		}
		// Fall back to any song if preferred category is empty
		fallback := applyBaseFilters(db.Model(&models.Song{}))
		if err := fallback.Order("RANDOM()").First(&song).Error; err == nil {
			return &song
		}
		return nil
	}

	if room.VotedRatio != nil && *room.VotedRatio >= 1.0 {
		// All voted
		query := applyBaseFilters(db.Model(&models.Song{}))
		query = query.Joins("INNER JOIN votes ON votes.song_id = songs.song_id AND votes.user_id = ?", room.CreatorID)
		if err := query.Order("RANDOM()").First(&song).Error; err == nil {
			return &song
		}
		// Fall back to any song
		fallback := applyBaseFilters(db.Model(&models.Song{}))
		if err := fallback.Order("RANDOM()").First(&song).Error; err == nil {
			return &song
		}
		return nil
	}

	if room.VotedRatio != nil && *room.VotedRatio <= 0 {
		// All unvoted
		query := applyBaseFilters(db.Model(&models.Song{}))
		query = query.Where("songs.song_id NOT IN (?)", db.Table("votes").Select("song_id").Where("user_id = ?", room.CreatorID))
		if err := query.Order("RANDOM()").First(&song).Error; err == nil {
			return &song
		}
		// Fall back to any song
		fallback := applyBaseFilters(db.Model(&models.Song{}))
		if err := fallback.Order("RANDOM()").First(&song).Error; err == nil {
			return &song
		}
		return nil
	}

	// No ratio set — pick any song
	query := applyBaseFilters(db.Model(&models.Song{}))
	if err := query.Order("RANDOM()").First(&song).Error; err == nil {
		return &song
	}
	return nil
}

func buildQuizSong(db *gorm.DB, song models.Song) models.QuizSong {
	artists := make([]models.QuizArtist, len(song.Artists))
	for i, a := range song.Artists {
		artists[i] = models.QuizArtist{
			ArtistID:     a.ArtistID,
			NameOriginal: a.NameOriginal,
			NameEnglish:  a.NameEnglish,
		}
	}

	// Load units for the song
	var songUnits []models.Unit
	db.Model(&song).Association("Units").Find(&songUnits)

	units := make([]models.QuizUnit, len(songUnits))
	for i, u := range songUnits {
		units[i] = models.QuizUnit{
			UnitID:       u.UnitID,
			NameOriginal: u.NameOriginal,
			NameEnglish:  u.NameEnglish,
		}
	}

	embedURL := getEmbedURL(song.SourceURL)
	avgRating := getAverageSongRatingFromDB(db, song.SongID)

	categoryName := ""
	if song.Category != nil {
		categoryName = song.Category.Name
	}

	return models.QuizSong{
		SongID:        song.SongID,
		TitleOriginal: song.NameOriginal,
		TitleEnglish:  song.NameEnglish,
		Artists:       artists,
		Units:         units,
		ThumbnailURL:  song.ThumbnailURL,
		SourceURL:     song.SourceURL,
		EmbedURL:      embedURL,
		CategoryName:  categoryName,
		IsCover:       song.IsCover,
		AverageRating: avgRating,
	}
}

// --- Answer matching ---

// normalizeString lowercases and strips accents/diacritics for comparison
func normalizeString(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	// Decompose unicode, then strip combining marks
	t := norm.NFD.String(s)
	result := make([]rune, 0, len(t))
	for _, r := range t {
		if !unicode.Is(unicode.Mn, r) { // Mn = Mark, nonspacing (combining marks)
			result = append(result, r)
		}
	}
	return string(result)
}

// levenshteinDistance computes edit distance between two strings
func levenshteinDistance(a, b string) int {
	la := len([]rune(a))
	lb := len([]rune(b))

	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	aRunes := []rune(a)
	bRunes := []rune(b)

	// Use two-row optimization
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if aRunes[i-1] == bRunes[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}

	return prev[lb]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// similarityScore returns a 0-1 score (1 = identical)
func similarityScore(a, b string) float64 {
	na := normalizeString(a)
	nb := normalizeString(b)

	if na == nb {
		return 1.0
	}

	dist := levenshteinDistance(na, nb)
	maxLen := math.Max(float64(len([]rune(na))), float64(len([]rune(nb))))
	if maxLen == 0 {
		return 1.0
	}

	return 1.0 - float64(dist)/maxLen
}

const similarityThreshold = 0.75

func matchesTitle(guess string, song models.QuizSong, fuzzyInput bool) bool {
	if fuzzyInput {
		// With fuzzy input, the user selected from a dropdown — exact ID match
		// But the guess is the text selected, so match exactly
		normGuess := normalizeString(guess)
		if normalizeString(song.TitleOriginal) == normGuess {
			return true
		}
		if song.TitleEnglish != "" && normalizeString(song.TitleEnglish) == normGuess {
			return true
		}
		return false
	}

	// Free text — use similarity matching
	if similarityScore(guess, song.TitleOriginal) >= similarityThreshold {
		return true
	}
	if song.TitleEnglish != "" && similarityScore(guess, song.TitleEnglish) >= similarityThreshold {
		return true
	}
	return false
}

func matchesArtist(guess string, song models.QuizSong, oneOfArtists bool, fuzzyInput bool) bool {
	if fuzzyInput {
		normGuess := normalizeString(guess)

		// Check against individual artists
		for _, artist := range song.Artists {
			if normalizeString(artist.NameOriginal) == normGuess || normalizeString(artist.NameEnglish) == normGuess {
				if oneOfArtists {
					return true
				}
			}
		}
		// Check against units/groups
		for _, unit := range song.Units {
			if normalizeString(unit.NameOriginal) == normGuess || normalizeString(unit.NameEnglish) == normGuess {
				return true
			}
		}

		// If not oneOfArtists, need all artists to match — but with single input
		// For simplicity with fuzzy dropdown, matching any artist or unit is sufficient
		if oneOfArtists {
			return false
		}

		// For "all artists" mode, check if the guess matches any combo
		// Since it's a single input, check if any artist or unit matches
		for _, artist := range song.Artists {
			if normalizeString(artist.NameOriginal) == normGuess || normalizeString(artist.NameEnglish) == normGuess {
				return true
			}
		}
		return false
	}

	// Free text mode with similarity matching
	for _, artist := range song.Artists {
		if similarityScore(guess, artist.NameOriginal) >= similarityThreshold {
			if oneOfArtists || len(song.Artists) == 1 {
				return true
			}
		}
		if artist.NameEnglish != "" && similarityScore(guess, artist.NameEnglish) >= similarityThreshold {
			if oneOfArtists || len(song.Artists) == 1 {
				return true
			}
		}
	}

	// Check units/groups
	for _, unit := range song.Units {
		if similarityScore(guess, unit.NameOriginal) >= similarityThreshold {
			return true
		}
		if unit.NameEnglish != "" && similarityScore(guess, unit.NameEnglish) >= similarityThreshold {
			return true
		}
	}

	return false
}

// --- Helpers ---

func allUsersGuessedCorrectly(roomID string, round *models.QuizRound, questionType string) bool {
	room, exists := quizRoomManager.GetRoom(roomID)
	if !exists {
		return false
	}

	room.Mutex.RLock()
	totalUsers := len(room.Clients)
	room.Mutex.RUnlock()

	if totalUsers == 0 {
		return false
	}

	correctCount := 0
	for _, guess := range round.UserGuesses {
		allCorrect := true
		if (questionType == "title" || questionType == "both") && !guess.TitleCorrect {
			allCorrect = false
		}
		if (questionType == "artist" || questionType == "both") && !guess.ArtistCorrect {
			allCorrect = false
		}
		if allCorrect {
			correctCount++
		}
	}

	return correctCount >= totalUsers
}

func updateQuizScores(state *models.QuizState, userID, username string) {
	// Recalculate total score for this user across all rounds
	totalScore := 0
	for _, round := range state.Rounds {
		for _, guess := range round.UserGuesses {
			if guess.UserID == userID {
				totalScore += guess.TotalPoints
				break
			}
		}
	}

	// Update or add score entry
	found := false
	for i, score := range state.Scores {
		if score.UserID == userID {
			state.Scores[i].Score = totalScore
			state.Scores[i].Username = username
			found = true
			break
		}
	}

	if !found {
		state.Scores = append(state.Scores, models.PlayerScore{
			UserID:   userID,
			Username: username,
			Score:    totalScore,
		})
	}
}

func broadcastGuessUpdate(roomID, userID, username string, guess *models.UserGuess) {
	guessInfo := map[string]interface{}{
		"user_id":        userID,
		"username":       username,
		"title_correct":  guess.TitleCorrect,
		"artist_correct": guess.ArtistCorrect,
	}

	data, _ := json.Marshal(guessInfo)
	quizRoomManager.BroadcastToRoom(roomID, wsocket.WSMessage{
		Type:      "guess_update",
		Data:      data,
		Timestamp: time.Now(),
	})
}

func broadcastQuizState(db *gorm.DB, roomID string) {
	var room models.QuizRoom
	if err := db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		return
	}

	// Send sanitized state to each client
	wsRoom, exists := quizRoomManager.GetRoom(roomID)
	if !exists {
		return
	}

	wsRoom.Mutex.RLock()
	clientIDs := make([]string, 0, len(wsRoom.Clients))
	for id := range wsRoom.Clients {
		clientIDs = append(clientIDs, id)
	}
	wsRoom.Mutex.RUnlock()

	for _, clientID := range clientIDs {
		stateToSend := buildQuizStateForClient(room, clientID)
		stateData, _ := json.Marshal(stateToSend)

		quizRoomManager.SendToClient(clientID, wsocket.WSMessage{
			Type:      "quiz_state",
			Data:      stateData,
			Timestamp: time.Now(),
		})
	}
}

func broadcastQuizUserUpdate(roomID string) {
	room, exists := quizRoomManager.GetRoom(roomID)
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

	quizRoomManager.BroadcastToRoom(roomID, message)
}

func loadQuizSongVotes(db *gorm.DB, songID uint) map[uint][]wsocket.VoteUpdateData {
	result := make(map[uint][]wsocket.VoteUpdateData)

	var votes []models.Vote
	db.Where("song_id = ?", songID).Find(&votes)

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

func generateQuizRoomCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}
