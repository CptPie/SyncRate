package database

import (
	"fmt"

	"github.com/CptPie/SyncRate/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Database struct {
	dsn string
	DB  *gorm.DB
}

func New(dsn string) *Database {
	return &Database{
		dsn: dsn,
	}
}

func (db *Database) Connect() error {
	gdb, err := gorm.Open(postgres.Open(db.dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("Failed to connect to database: %s", err.Error())
	}
	db.DB = gdb
	return nil
}

func (db *Database) Migrate() error {
	// First migrate the base tables (no foreign key dependencies)
	err := db.DB.AutoMigrate(
		&models.User{},
		&models.Unit{},
		&models.Artist{},
		&models.Album{},
	)
	if err != nil {
		return fmt.Errorf("Migration failed for base tables: %s", err.Error())
	}

	// Then migrate Song (depends on Unit via UnitID foreign key)
	err = db.DB.AutoMigrate(&models.Song{})
	if err != nil {
		return fmt.Errorf("Migration failed for Song: %s", err.Error())
	}

	// Then migrate Vote (depends on User and Song)
	err = db.DB.AutoMigrate(&models.Vote{})
	if err != nil {
		return fmt.Errorf("Migration failed for Vote: %s", err.Error())
	}

	// Finally migrate join tables (depend on the main tables)
	err = db.DB.AutoMigrate(
		&models.SongArtist{},
		&models.AlbumSong{},
		&models.ArtistUnit{},
	)
	if err != nil {
		return fmt.Errorf("Migration failed for join tables: %s", err.Error())
	}

	return nil
}
