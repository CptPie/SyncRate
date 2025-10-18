package main

import (
	"fmt"
	"log"
	"os"

	"github.com/CptPie/SyncRate/database"
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

	// Start background cleanup for old rating rooms
	handlers.StartDatabaseCleanup(db.DB)
	log.Println("Started database cleanup routine for rating rooms")

	// Start web server
	r := router.SetupRouter(db.DB)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	log.Fatal(r.Run(":" + port))
}
