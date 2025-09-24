package router

import (
	"html/template"
	"log"

	"github.com/CptPie/SyncRate/server/handlers"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupRouter(db *gorm.DB) *gin.Engine {
	r := gin.Default()

	// Load HTML templates with proper parsing
	log.Println("Loading component templates...")
	tmpl := template.Must(template.ParseGlob("web/templates/components/*.html"))
	log.Println("Loading page templates...")
	tmpl = template.Must(tmpl.ParseGlob("web/templates/pages/*.html"))

	// Debug: list all defined templates
	for _, t := range tmpl.Templates() {
		log.Printf("Loaded template: %s", t.Name())
	}

	r.SetHTMLTemplate(tmpl)

	// Serve static files
	r.Static("/static", "web/static")

	// Home routes
	r.GET("/", func(c *gin.Context) {
		log.Println("Router: Home route (/) accessed")
		handlers.GetHome(db)(c)
	})

	// Song routes
	r.GET("/songs", handlers.GetSongs(db))
	r.GET("/songs/:id", handlers.GetSong(db))
	r.POST("/songs/:id/vote", handlers.PostVote(db))

	// User routes
	r.GET("/login", handlers.GetLogin(db))
	r.POST("/login", handlers.PostLogin(db))
	r.GET("/register", handlers.GetRegister(db))
	r.POST("/register", handlers.PostRegister(db))
	r.POST("/logout", handlers.PostLogout(db))

	// Admin routes
	r.GET("/admin", handlers.GetAdmin(db))

	// Add routes
	r.GET("/admin/add-category", handlers.GetAddCategory(db))
	r.POST("/admin/add-category", handlers.PostAddCategory(db))
	r.GET("/admin/add-unit", handlers.GetAddUnit(db))
	r.POST("/admin/add-unit", handlers.PostAddUnit(db))
	r.GET("/admin/add-artist", handlers.GetAddArtist(db))
	r.POST("/admin/add-artist", handlers.PostAddArtist(db))
	r.GET("/admin/add-song", handlers.GetAddSong(db))
	r.POST("/admin/add-song", handlers.PostAddSong(db))

	// View routes
	r.GET("/admin/categories", handlers.GetViewCategories(db))
	r.GET("/admin/units", handlers.GetViewUnits(db))
	r.GET("/admin/artists", handlers.GetViewArtists(db))
	r.GET("/admin/view-songs", handlers.GetViewSongs(db))

	// Edit routes
	r.POST("/admin/categories/:id/edit", handlers.PostEditCategory(db))
	r.POST("/admin/units/:id/edit", handlers.PostEditUnit(db))
	r.POST("/admin/artists/:id/edit", handlers.PostEditArtist(db))
	r.POST("/admin/songs/:id/edit", handlers.PostEditSong(db))

	// Delete routes
	r.POST("/admin/categories/:id/delete", handlers.PostDeleteCategory(db))
	r.POST("/admin/units/:id/delete", handlers.PostDeleteUnit(db))
	r.POST("/admin/artists/:id/delete", handlers.PostDeleteArtist(db))
	r.POST("/admin/songs/:id/delete", handlers.PostDeleteSong(db))

	return r
}

