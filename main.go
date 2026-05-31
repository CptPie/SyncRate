package main

import (
	"fmt"
	"log"
	"os"

	"github.com/CptPie/SyncRate/database"
	"github.com/CptPie/SyncRate/models"
	"github.com/CptPie/SyncRate/server/handlers"
	"github.com/CptPie/SyncRate/server/router"
)

func main() {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)
	db := database.New(dsn)

	err := db.Connect()
	if err != nil {
		log.Fatal(err.Error())
	}

	err = db.Migrate()
	if err != nil {
		log.Fatal(err.Error())
	}

	// Bootstrap the admin account: promote the configured username if it
	// exists. No-op when ADMIN_USERNAME is unset or the user hasn't
	// registered yet (they gain admin on the next startup after registering).
	if adminUsername := os.Getenv("ADMIN_USERNAME"); adminUsername != "" {
		result := db.DB.Model(&models.User{}).
			Where("username = ?", adminUsername).
			Update("is_admin", true)
		if result.Error != nil {
			log.Printf("WARNING: failed to promote admin user %q: %v", adminUsername, result.Error)
		} else if result.RowsAffected > 0 {
			log.Printf("Ensured admin privileges for user %q", adminUsername)
		} else {
			log.Printf("ADMIN_USERNAME %q not found yet; register it then restart to grant admin", adminUsername)
		}
	}

	// Start background cleanup for old rating rooms
	handlers.StartDatabaseCleanup(db.DB)
	log.Println("Started database cleanup routine for rating rooms")

	// Start background cleanup for old radio rooms
	handlers.StartRadioRoomDatabaseCleanup(db.DB)
	log.Println("Started database cleanup routine for radio rooms")

	// Start background cleanup for old tournament rooms
	handlers.StartTournamentDatabaseCleanup(db.DB)
	log.Println("Started database cleanup routine for tournament rooms")

	// Start background cleanup for old quiz rooms
	handlers.StartQuizDatabaseCleanup(db.DB)
	log.Println("Started database cleanup routine for quiz rooms")

	// Start web server
	r := router.SetupRouter(db.DB)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	log.Fatal(r.Run(":" + port))
}
