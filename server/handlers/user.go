package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/CptPie/SyncRate/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetMyVotes renders the current user's votes page, listing every song the user
// has voted on along with their rating and comment. Filtering and sorting are
// handled client-side from the JSON payload embedded in the template.
func GetMyVotes(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Require authentication
		userID, exists := c.Get("user_id")
		if !exists || userID == nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}

		// voteCardData is the flattened shape consumed by the client-side
		// filter/sort logic.
		type voteCardData struct {
			VoteID       uint      `json:"vote_id"`
			Rating       int       `json:"rating"`
			Comment      string    `json:"comment"`
			SongID       uint      `json:"song_id"`
			NameOriginal string    `json:"name_original"`
			NameEnglish  string    `json:"name_english"`
			ThumbnailURL string    `json:"thumbnail_url"`
			SourceURL    string    `json:"source_url"`
			CategoryID   *uint     `json:"category_id"`
			CategoryName string    `json:"category_name"`
			IsCover      bool      `json:"is_cover"`
			Artists      string    `json:"artists"`
			CreatedAt    time.Time `json:"created_at"`
		}

		// Load the user's votes ordered by vote ID (the default sort).
		var votes []models.Vote
		if err := db.Where("user_id = ?", userID).Order("vote_id").Find(&votes).Error; err != nil {
			log.Printf("GetMyVotes: error loading votes for user %v: %v", userID, err)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title": "SyncRate | Error",
				"error": "Failed to load your votes: " + err.Error(),
			})
			return
		}

		// Load the songs referenced by those votes, with the relations needed
		// for display, in a single query.
		songMap := make(map[uint]models.Song)
		if len(votes) > 0 {
			songIDs := make([]uint, 0, len(votes))
			for _, v := range votes {
				songIDs = append(songIDs, v.SongID)
			}

			var songs []models.Song
			if err := db.Preload("Artists").Preload("Category").
				Where("song_id IN ?", songIDs).Find(&songs).Error; err != nil {
				log.Printf("GetMyVotes: error loading songs: %v", err)
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"title": "SyncRate | Error",
					"error": "Failed to load songs for your votes: " + err.Error(),
				})
				return
			}
			for _, s := range songs {
				songMap[s.SongID] = s
			}
		}

		// Build the flattened card data, skipping votes whose song no longer
		// exists (e.g. deleted from the catalog).
		cards := make([]voteCardData, 0, len(votes))
		for _, v := range votes {
			song, ok := songMap[v.SongID]
			if !ok {
				continue
			}
			cards = append(cards, voteCardData{
				VoteID:       v.VoteID,
				Rating:       v.Rating,
				Comment:      v.Comment,
				SongID:       song.SongID,
				NameOriginal: song.NameOriginal,
				NameEnglish:  song.NameEnglish,
				ThumbnailURL: song.ThumbnailURL,
				SourceURL:    song.SourceURL,
				CategoryID:   song.CategoryID,
				CategoryName: getCategoryName(song.Category),
				IsCover:      song.IsCover,
				Artists:      formatArtistNames(song.Artists),
				CreatedAt:    v.CreatedAt,
			})
		}

		// Categories for the filter dropdown.
		var categories []models.Category
		db.Order("name").Find(&categories)

		votesJSON, _ := json.Marshal(cards)
		categoriesJSON, _ := json.Marshal(categories)

		templateData := GetUserContext(c)
		templateData["title"] = "SyncRate | My Votes"
		templateData["votes"] = cards
		templateData["voteCount"] = len(cards)
		templateData["votesJSON"] = string(votesJSON)
		templateData["categoriesJSON"] = string(categoriesJSON)

		c.HTML(http.StatusOK, "my-votes.html", templateData)
	}
}
