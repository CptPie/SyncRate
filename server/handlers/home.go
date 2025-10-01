package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetHome(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		templateData := GetUserContext(c)
		templateData["title"] = "SyncRate | Home"

		c.HTML(http.StatusOK, "home.html", templateData)
		log.Println("GetHome: Template rendered successfully")
	}
}
