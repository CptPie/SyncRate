package main

import (
	"fmt"
	"log"
	"os"

	"github.com/CptPie/SyncRate/database"
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

}
