package router

import (
	"html/template"
	"log"

	"github.com/CptPie/SyncRate/server/handlers"
	"github.com/CptPie/SyncRate/server/middleware"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupRouter(db *gorm.DB) *gin.Engine {
	r := gin.Default()

	// Configure trusted proxies (disable for direct connections)
	r.SetTrustedProxies(nil)

	// Session setup
	store := cookie.NewStore([]byte("your-secret-key-change-this-in-production"))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
	})
	r.Use(sessions.Sessions("syncrate-session", store))

	// Add user context middleware to all routes
	r.Use(middleware.SetUserContext())

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

	// Admin routes (protected)
	admin := r.Group("/admin")
	admin.Use(middleware.RequireAuth())
	{
		admin.GET("/", handlers.GetAdmin(db))

		// Add routes
		admin.GET("/add-category", handlers.GetAddCategory(db))
		admin.POST("/add-category", handlers.PostAddCategory(db))
		admin.GET("/add-unit", handlers.GetAddUnit(db))
		admin.POST("/add-unit", handlers.PostAddUnit(db))
		admin.GET("/add-artist", handlers.GetAddArtist(db))
		admin.POST("/add-artist", handlers.PostAddArtist(db))
		admin.GET("/add-song", handlers.GetAddSong(db))
		admin.POST("/add-song", handlers.PostAddSong(db))

		// View routes
		admin.GET("/categories", handlers.GetViewCategories(db))
		admin.GET("/units", handlers.GetViewUnits(db))
		admin.GET("/artists", handlers.GetViewArtists(db))
		admin.GET("/view-songs", handlers.GetViewSongs(db))

		// Edit routes
		admin.POST("/categories/:id/edit", handlers.PostEditCategory(db))
		admin.POST("/units/:id/edit", handlers.PostEditUnit(db))
		admin.POST("/artists/:id/edit", handlers.PostEditArtist(db))
		admin.POST("/songs/:id/edit", handlers.PostEditSong(db))

		// Delete routes
		admin.POST("/categories/:id/delete", handlers.PostDeleteCategory(db))
		admin.POST("/units/:id/delete", handlers.PostDeleteUnit(db))
		admin.POST("/artists/:id/delete", handlers.PostDeleteArtist(db))
		admin.POST("/songs/:id/delete", handlers.PostDeleteSong(db))
	}

	return r
}

