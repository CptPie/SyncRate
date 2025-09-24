package handlers

import (
	"log"
	"net/http"

	"github.com/CptPie/SyncRate/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetHome(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("GetHome: Starting to load songs")

		var songs []models.Song
		result := db.Preload("Artists.Artist").Preload("Category").Find(&songs)

		if result.Error != nil {
			log.Printf("GetHome: Database error: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to load songs: " + result.Error.Error(),
			})
			return
		}

		log.Printf("GetHome: Successfully loaded %d songs", len(songs))

		log.Println("GetHome: About to render home.html template")

		// Check if template exists first
		log.Println("GetHome: Available templates:")
		// This is a debug hack - let's try rendering with explicit error handling

		templateData := gin.H{
			"title": "SyncRate - Music Rating",
			"songs": songs,
		}
		log.Printf("GetHome: Template data: %+v", templateData)

		c.HTML(http.StatusOK, "home.html", templateData)
		log.Println("GetHome: Template rendered successfully")
	}
}

