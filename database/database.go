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
		return fmt.Errorf("failed to connect to database: %s", err.Error())
	}
	db.DB = gdb
	return nil
}

func (db *Database) Migrate() error {
	// Disable foreign key checks during migration to avoid circular dependencies
	fmt.Println("Disabling foreign key constraints for migration...")
	err := db.DB.Exec("SET session_replication_role = replica").Error
	if err != nil {
		return fmt.Errorf("failed to disable foreign key constraints: %s", err.Error())
	}

	// First migrate Category table (no dependencies)
	fmt.Println("Starting migration for Category table...")
	err = db.DB.AutoMigrate(&models.Category{})
	if err != nil {
		return fmt.Errorf("migration failed for Category: %s", err.Error())
	}
	fmt.Println("✓ Category table migrated successfully")

	// Then migrate all tables without foreign key constraints being enforced
	fmt.Println("Starting migration for User table...")
	err = db.DB.AutoMigrate(&models.User{})
	if err != nil {
		return fmt.Errorf("migration failed for User: %s", err.Error())
	}
	fmt.Println("✓ User table migrated successfully")

	fmt.Println("Starting migration for Unit table...")
	err = db.DB.AutoMigrate(&models.Unit{})
	if err != nil {
		return fmt.Errorf("migration failed for Unit: %s", err.Error())
	}
	fmt.Println("✓ Unit table migrated successfully")

	fmt.Println("Starting migration for Artist table...")
	err = db.DB.AutoMigrate(&models.Artist{})
	if err != nil {
		return fmt.Errorf("migration failed for Artist: %s", err.Error())
	}
	fmt.Println("✓ Artist table migrated successfully")

	fmt.Println("Starting migration for Album table...")
	err = db.DB.AutoMigrate(&models.Album{})
	if err != nil {
		return fmt.Errorf("migration failed for Album: %s", err.Error())
	}
	fmt.Println("✓ Album table migrated successfully")

	fmt.Println("Starting migration for Song table...")
	err = db.DB.AutoMigrate(&models.Song{})
	if err != nil {
		return fmt.Errorf("migration failed for Song: %s", err.Error())
	}
	fmt.Println("✓ Song table migrated successfully")

	fmt.Println("Starting migration for Vote table...")
	err = db.DB.AutoMigrate(&models.Vote{})
	if err != nil {
		return fmt.Errorf("migration failed for Vote: %s", err.Error())
	}
	fmt.Println("✓ Vote table migrated successfully")

	fmt.Println("Starting migration for RatingRoom table...")
	err = db.DB.AutoMigrate(&models.RatingRoom{})
	if err != nil {
		return fmt.Errorf("migration failed for RatingRoom: %s", err.Error())
	}
	fmt.Println("✓ RatingRoom table migrated successfully")

	// Migrate join tables
	fmt.Println("Starting migration for SongArtist join table...")
	err = db.DB.AutoMigrate(&models.SongArtist{})
	if err != nil {
		return fmt.Errorf("migration failed for SongArtist: %s", err.Error())
	}
	fmt.Println("✓ SongArtist table migrated successfully")

	fmt.Println("Starting migration for SongUnit join table...")
	err = db.DB.AutoMigrate(&models.SongUnit{})
	if err != nil {
		return fmt.Errorf("migration failed for SongUnit: %s", err.Error())
	}
	fmt.Println("✓ SongUnit table migrated successfully")

	fmt.Println("Starting migration for AlbumSong join table...")
	err = db.DB.AutoMigrate(&models.AlbumSong{})
	if err != nil {
		return fmt.Errorf("migration failed for AlbumSong: %s", err.Error())
	}
	fmt.Println("✓ AlbumSong table migrated successfully")

	fmt.Println("Starting migration for ArtistUnit join table...")
	err = db.DB.AutoMigrate(&models.ArtistUnit{})
	if err != nil {
		return fmt.Errorf("migration failed for ArtistUnit: %s", err.Error())
	}
	fmt.Println("✓ ArtistUnit table migrated successfully")

	// Re-enable foreign key constraints
	fmt.Println("Re-enabling foreign key constraints...")
	err = db.DB.Exec("SET session_replication_role = DEFAULT").Error
	if err != nil {
		return fmt.Errorf("failed to re-enable foreign key constraints: %s", err.Error())
	}

	fmt.Println("🎉 All migrations completed successfully!")
	return nil
}
