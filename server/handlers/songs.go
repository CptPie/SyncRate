package handlers

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/CptPie/SyncRate/database"
	"github.com/CptPie/SyncRate/models"
	"github.com/CptPie/SyncRate/server/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Compact projections of the song graph for the /songs listing.
//
// The full models drag along per-row timestamps plus nested back-references
// (Artist.Category, Artist.Songs, Album.Songs, Song.Votes) that the page never
// reads. At 4137 songs that dead weight was ~75% of a 10.7 MB response, so only
// the fields the card renderer, the colour styling and the search index
// actually touch are serialised.
//
// The JSON field names deliberately mirror the model field names: the renderers
// in web/templates/components/search-filter.html, the search field list it
// passes to FuzzySearchFilter, and web/static/js/artist-colors.js all index
// into these objects by name.
type songListEntity struct {
	NameOriginal   string
	NameEnglish    string
	PrimaryColor   string
	SecondaryColor string
}

type songListAlbum struct {
	NameOriginal string
	NameEnglish  string
}

type songListCategory struct {
	CategoryID uint
	Name       string
}

type songListItem struct {
	SongID       uint
	NameOriginal string
	NameEnglish  string
	SourceURL    string
	ThumbnailURL string
	IsCover      bool
	CategoryID   *uint
	Category     *songListCategory
	Artists      []songListEntity
	Units        []songListEntity
	Albums       []songListAlbum
	AverageScore float64 `json:"average_score"`
	VoteCount    int64   `json:"vote_count"`
}

func newSongListEntity(nameOriginal, nameEnglish, primary, secondary string) songListEntity {
	return songListEntity{
		NameOriginal:   nameOriginal,
		NameEnglish:    nameEnglish,
		PrimaryColor:   primary,
		SecondaryColor: secondary,
	}
}

func GetSongs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("GetSongs: Starting to load all songs")

		var songs []models.Song
		var categories []models.Category

		result := db.Preload("Artists").Preload("Units").Preload("Category").Preload("Albums").Order("song_id").Find(&songs)
		if result.Error != nil {
			log.Printf("GetSongs: Database error: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Failed to load songs: " + result.Error.Error(),
			})
			return
		}

		db.Find(&categories)

		// One grouped query for every song's score, rather than per-song lookups.
		dbWrapper := &database.Database{DB: db}
		stats, err := dbWrapper.GetSongRatingStats()
		if err != nil {
			// Scores are decoration; the catalogue is still worth serving without them.
			log.Printf("GetSongs: Error loading song rating stats: %v", err)
			stats = map[uint]database.SongRatingStat{}
		}

		songList := make([]songListItem, 0, len(songs))
		for _, song := range songs {
			item := songListItem{
				SongID:       song.SongID,
				NameOriginal: song.NameOriginal,
				NameEnglish:  song.NameEnglish,
				SourceURL:    song.SourceURL,
				ThumbnailURL: song.ThumbnailURL,
				IsCover:      song.IsCover,
				CategoryID:   song.CategoryID,
			}

			if song.Category != nil {
				item.Category = &songListCategory{
					CategoryID: song.Category.CategoryID,
					Name:       song.Category.Name,
				}
			}

			for _, artist := range song.Artists {
				item.Artists = append(item.Artists, newSongListEntity(
					artist.NameOriginal, artist.NameEnglish, artist.PrimaryColor, artist.SecondaryColor))
			}
			for _, unit := range song.Units {
				item.Units = append(item.Units, newSongListEntity(
					unit.NameOriginal, unit.NameEnglish, unit.PrimaryColor, unit.SecondaryColor))
			}
			for _, album := range song.Albums {
				item.Albums = append(item.Albums, songListAlbum{
					NameOriginal: album.NameOriginal,
					NameEnglish:  album.NameEnglish,
				})
			}

			if stat, ok := stats[song.SongID]; ok {
				// The card renders this with toFixed(1); full float64 precision
				// would just be ~14 wasted bytes per song on the wire.
				item.AverageScore = math.Round(stat.AvgRating*100) / 100
				item.VoteCount = stat.VoteCount
			}

			songList = append(songList, item)
		}

		// Inlined into the page for the client-side search and pagination.
		songsJSON, _ := json.Marshal(songList)
		categoriesJSON, _ := json.Marshal(categories)

		log.Printf("GetSongs: Successfully loaded %d songs", len(songs))

		templateData := GetUserContext(c)
		templateData["title"] = "SyncRate | All Songs"
		templateData["songs"] = songList
		templateData["categories"] = categories
		templateData["songsJSON"] = string(songsJSON)
		templateData["categoriesJSON"] = string(categoriesJSON)
		templateData["isAdminPage"] = false

		c.HTML(http.StatusOK, "songs.html", templateData)
	}
}

func GetSong(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		log.Printf("GetSong: Requested song ID: %s", idParam)

		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			log.Printf("GetSong: Invalid song ID format: %v", err)
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Invalid song ID: " + err.Error(),
			})
			return
		}

		var song models.Song
		result := db.Preload("Artists").Preload("Category").Preload("Units").Preload("Albums").First(&song, uint(id))
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				log.Printf("GetSong: Song with ID %d not found", id)
				c.HTML(http.StatusNotFound, "error.html", gin.H{
					"error": "Song not found",
				})
				return
			}
			log.Printf("GetSong: Database error loading song %d: %v", id, result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to load song: " + result.Error.Error(),
			})
			return
		}

		// Load votes with user information using a JOIN
		type VoteWithUser struct {
			models.Vote
			Username string
		}

		var votesWithUsers []VoteWithUser
		voteResult := db.Table("votes").
			Select("votes.*, users.username").
			Joins("LEFT JOIN users ON votes.user_id = users.user_id").
			Where("votes.song_id = ?", id).
			Find(&votesWithUsers)

		if voteResult.Error != nil {
			log.Printf("GetSong: Error loading votes for song %d: %v", id, voteResult.Error)
		}

		log.Printf("GetSong: Successfully loaded song '%s' with %d votes", song.NameOriginal, len(votesWithUsers))

		// Convert song to JSON for JavaScript color initialization
		// Wrap in array to match the format expected by artist-colors.js
		songsArray := []models.Song{song}
		songJSON, err := json.Marshal(songsArray)
		if err != nil {
			log.Printf("GetSong: Error marshaling song JSON: %v", err)
			songJSON = []byte("[]") // Empty array fallback
		}

		// Generate YouTube embed URL if source is a YouTube URL
		var embedURL string
		if song.SourceURL != "" {
			if url, err := utils.GetYouTubeEmbedURL(song.SourceURL); err == nil {
				embedURL = url
			}
		}

		templateData := GetUserContext(c)
		templateData["title"] = song.NameOriginal
		templateData["song"] = song
		templateData["votes"] = votesWithUsers
		templateData["songJSON"] = string(songJSON)
		templateData["embedURL"] = embedURL

		// Calculate average score and vote count for this song
		dbWrapper := &database.Database{DB: db}

		avgScore, err := dbWrapper.GetAverageRatingForSong(uint(id))
		if err != nil {
			log.Printf("Error getting average rating for song %d: %v", id, err)
			avgScore = 0
		}

		voteCount, err := dbWrapper.GetVoteCountForSong(uint(id))
		if err != nil {
			log.Printf("Error getting vote count for song %d: %v", id, err)
			voteCount = 0
		}

		templateData["average_score"] = avgScore
		templateData["vote_count"] = voteCount

		// Check if current user has voted for this song
		if userID, exists := c.Get("user_id"); exists && userID != nil {
			var userVote models.Vote
			// Use Find instead of First to avoid "record not found" errors in logs
			result := db.Where("user_id = ? AND song_id = ?", userID, id).Limit(1).Find(&userVote)
			if result.Error != nil {
				log.Printf("GetSong: Error checking user vote: %v", result.Error)
			} else if result.RowsAffected > 0 {
				// User has voted - include the vote in template data
				templateData["user_vote"] = userVote
			}
			// If RowsAffected == 0, user hasn't voted yet, which is fine
		}

		c.HTML(http.StatusOK, "song.html", templateData)
	}
}

func PostVote(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if user is authenticated
		userID, exists := c.Get("user_id")
		if !exists || userID == nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}

		// Get song ID from URL
		songIDParam := c.Param("id")
		songID, err := strconv.ParseUint(songIDParam, 10, 32)
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Invalid song ID",
			})
			return
		}

		// Verify song exists
		var song models.Song
		if err := db.First(&song, uint(songID)).Error; err != nil {
			c.HTML(http.StatusNotFound, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Song not found",
			})
			return
		}

		// Get form data
		ratingStr := c.PostForm("rating")
		comment := strings.TrimSpace(c.PostForm("comment"))

		// Validate rating
		rating, err := strconv.Atoi(ratingStr)
		if err != nil || rating < 1 || rating > 10 {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Rating must be a number between 1 and 10",
			})
			return
		}

		// Check if user already voted for this song
		var existingVote models.Vote
		result := db.Session(&gorm.Session{Logger: db.Logger.LogMode(logger.Silent)}).Where("user_id = ? AND song_id = ?", userID, uint(songID)).First(&existingVote)

		if result.Error == nil {
			// Update existing vote
			existingVote.Rating = rating
			existingVote.Comment = comment
			if err := db.Save(&existingVote).Error; err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"title": "SyncRate | Error",
					"error": "Failed to update vote",
				})
				return
			}
		} else if result.Error == gorm.ErrRecordNotFound {
			// Create new vote
			vote := models.Vote{
				UserID:  userID.(uint),
				SongID:  uint(songID),
				Rating:  rating,
				Comment: comment,
			}
			if err := db.Create(&vote).Error; err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"title": "SyncRate | Error",
					"error": "Failed to create vote",
				})
				return
			}
		} else {
			// Database error
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Database error",
			})
			return
		}

		// Redirect back to song page
		c.Redirect(http.StatusFound, "/songs/"+songIDParam)
	}
}
