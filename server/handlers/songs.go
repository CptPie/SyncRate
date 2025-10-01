package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/CptPie/SyncRate/models"
	"github.com/CptPie/SyncRate/server/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetSongs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("GetSongs: Starting to load all songs")

		var songs []models.Song
		var categories []models.Category

		result := db.Preload("Artists").Preload("Units").Preload("Category").Find(&songs)
		if result.Error != nil {
			log.Printf("GetSongs: Database error: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Failed to load songs: " + result.Error.Error(),
			})
			return
		}

		db.Find(&categories)

		// Convert to JSON for JavaScript
		songsJSON, _ := json.Marshal(songs)
		categoriesJSON, _ := json.Marshal(categories)

		log.Printf("GetSongs: Successfully loaded %d songs", len(songs))

		c.HTML(http.StatusOK, "songs.html", gin.H{
			"title":          "SyncRate | All Songs",
			"songs":          songs,
			"categories":     categories,
			"songsJSON":      string(songsJSON),
			"categoriesJSON": string(categoriesJSON),
			"isAdminPage":    false,
		})
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
		result := db.Preload("Artists").Preload("Category").Preload("Units").First(&song, uint(id))
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

		var votes []models.Vote
		voteResult := db.Where("song_id = ?", id).Preload("User").Find(&votes)
		if voteResult.Error != nil {
			log.Printf("GetSong: Error loading votes for song %d: %v", id, voteResult.Error)
		}

		log.Printf("GetSong: Successfully loaded song '%s' with %d votes", song.NameOriginal, len(votes))

		log.Printf("%v\n", song)

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

		c.HTML(http.StatusOK, "song.html", gin.H{
			"title":    song.NameOriginal,
			"song":     song,
			"votes":    votes,
			"songJSON": string(songJSON),
			"embedURL": embedURL,
		})
	}
}

func PostVote(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Add authentication middleware first
		// For now, this is a placeholder
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "Voting not implemented yet - need authentication",
		})
	}
}
