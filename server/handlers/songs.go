package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/CptPie/SyncRate/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetSongs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("GetSongs: Starting to load all songs")

		var songs []models.Song
		result := db.Preload("Artists.Artist").Preload("Category").Find(&songs)
		if result.Error != nil {
			log.Printf("GetSongs: Database error: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to load songs: " + result.Error.Error(),
			})
			return
		}

		log.Printf("GetSongs: Successfully loaded %d songs", len(songs))

		c.HTML(http.StatusOK, "songs.html", gin.H{
			"title": "All Songs",
			"songs": songs,
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
				"error": "Invalid song ID: " + err.Error(),
			})
			return
		}

		var song models.Song
		result := db.Preload("Artists.Artist").Preload("Category").Preload("Units").First(&song, uint(id))
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

		c.HTML(http.StatusOK, "song.html", gin.H{
			"title": song.NameOriginal,
			"song":  song,
			"votes": votes,
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

