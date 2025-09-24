package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/CptPie/SyncRate/models"
	"github.com/CptPie/SyncRate/server/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Admin index page
func GetAdmin(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("GetAdmin: Loading admin panel")

		var categoryCount, unitCount, artistCount, songCount int64
		db.Model(&models.Category{}).Count(&categoryCount)
		db.Model(&models.Unit{}).Count(&unitCount)
		db.Model(&models.Artist{}).Count(&artistCount)
		db.Model(&models.Song{}).Count(&songCount)

		c.HTML(http.StatusOK, "admin-index.html", gin.H{
			"title":         "Admin Panel",
			"categoryCount": categoryCount,
			"unitCount":     unitCount,
			"artistCount":   artistCount,
			"songCount":     songCount,
		})
	}
}

// Add Category page
func GetAddCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("GetAddCategory: Loading add category page")

		var categories []models.Category
		db.Find(&categories)

		c.HTML(http.StatusOK, "add-category.html", gin.H{
			"title":      "Add Category",
			"categories": categories,
		})
	}
}

// Add Unit page
func GetAddUnit(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("GetAddUnit: Loading add unit page")

		var units []models.Unit
		var categories []models.Category
		var artists []models.Artist
		db.Find(&units)
		db.Find(&categories)
		db.Preload("Category").Find(&artists)

		// Convert to JSON for JavaScript
		categoriesJSON, _ := json.Marshal(categories)
		artistsJSON, _ := json.Marshal(artists)

		c.HTML(http.StatusOK, "add-unit.html", gin.H{
			"title":          "Add Unit",
			"units":          units,
			"categories":     categories,
			"artists":        artists,
			"categoriesJSON": string(categoriesJSON),
			"artistsJSON":    string(artistsJSON),
		})
	}
}

// Add Artist page
func GetAddArtist(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("GetAddArtist: Loading add artist page")

		var artists []models.Artist
		var units []models.Unit
		var categories []models.Category
		db.Find(&artists)
		db.Find(&units)
		db.Find(&categories)

		// Convert to JSON for JavaScript
		unitsJSON, _ := json.Marshal(units)
		categoriesJSON, _ := json.Marshal(categories)

		c.HTML(http.StatusOK, "add-artist.html", gin.H{
			"title":          "Add Artist",
			"artists":        artists,
			"units":          units,
			"categories":     categories,
			"unitsJSON":      string(unitsJSON),
			"categoriesJSON": string(categoriesJSON),
		})
	}
}

// Add Song page
func GetAddSong(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("GetAddSong: Loading add song page")

		var categories []models.Category
		var artists []models.Artist
		var units []models.Unit
		db.Find(&categories)
		db.Find(&artists)
		db.Find(&units)

		// Convert to JSON for JavaScript
		categoriesJSON, _ := json.Marshal(categories)
		artistsJSON, _ := json.Marshal(artists)
		unitsJSON, _ := json.Marshal(units)

		c.HTML(http.StatusOK, "add-song.html", gin.H{
			"title":          "Add Song",
			"categories":     categories,
			"artists":        artists,
			"units":          units,
			"categoriesJSON": string(categoriesJSON),
			"artistsJSON":    string(artistsJSON),
			"unitsJSON":      string(unitsJSON),
		})
	}
}

// POST handlers for form submissions

func PostAddCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("PostAddCategory: Adding new category")

		name := strings.TrimSpace(c.PostForm("name"))
		if name == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Category name is required",
			})
			return
		}

		category := models.Category{
			Name: name,
		}

		result := db.Create(&category)
		if result.Error != nil {
			log.Printf("PostAddCategory: Error creating category: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to create category: " + result.Error.Error(),
			})
			return
		}

		log.Printf("PostAddCategory: Successfully created category '%s' with ID %d", category.Name, category.CategoryID)
		c.Redirect(http.StatusSeeOther, "/admin/add-category")
	}
}

func PostAddUnit(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("PostAddUnit: Adding new unit")

		nameOrig := strings.TrimSpace(c.PostForm("name_original"))
		if nameOrig == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Unit name is required",
			})
			return
		}

		unit := models.Unit{
			NameOriginal:   nameOrig,
			NameEnglish:    strings.TrimSpace(c.PostForm("name_english")),
			PrimaryColor:   strings.TrimSpace(c.PostForm("primary_color")),
			SecondaryColor: strings.TrimSpace(c.PostForm("secondary_color")),
		}

		// Parse category ID if provided
		if categoryIDStr := strings.TrimSpace(c.PostForm("category_id")); categoryIDStr != "" {
			if categoryID, err := strconv.ParseUint(categoryIDStr, 10, 32); err == nil {
				categoryIDUint := uint(categoryID)
				unit.CategoryID = &categoryIDUint
			}
		}

		// Start transaction
		tx := db.Begin()

		result := tx.Create(&unit)
		if result.Error != nil {
			tx.Rollback()
			log.Printf("PostAddUnit: Error creating unit: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to create unit: " + result.Error.Error(),
			})
			return
		}

		// Handle artist associations
		if artistIDsStr := strings.TrimSpace(c.PostForm("artist_ids")); artistIDsStr != "" {
			artistIDs := strings.Split(artistIDsStr, ",")
			for _, artistIDStr := range artistIDs {
				if artistID, err := strconv.ParseUint(strings.TrimSpace(artistIDStr), 10, 32); err == nil {
					artistUnit := models.ArtistUnit{
						ArtistID: uint(artistID),
						UnitID:   unit.UnitID,
					}
					if err := tx.Create(&artistUnit).Error; err != nil {
						log.Printf("PostAddUnit: Warning - failed to associate unit with artist %d: %v", artistID, err)
					}
				}
			}
		}

		tx.Commit()

		log.Printf("PostAddUnit: Successfully created unit '%s' with ID %d", unit.NameOriginal, unit.UnitID)
		c.Redirect(http.StatusSeeOther, "/admin/add-unit")
	}
}

func PostAddArtist(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("PostAddArtist: Adding new artist")

		nameOrig := strings.TrimSpace(c.PostForm("name_original"))
		if nameOrig == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Artist name is required",
			})
			return
		}

		artist := models.Artist{
			NameOriginal:   nameOrig,
			NameEnglish:    strings.TrimSpace(c.PostForm("name_english")),
			PrimaryColor:   strings.TrimSpace(c.PostForm("primary_color")),
			SecondaryColor: strings.TrimSpace(c.PostForm("secondary_color")),
		}

		// Parse category ID if provided
		if categoryIDStr := strings.TrimSpace(c.PostForm("category_id")); categoryIDStr != "" {
			if categoryID, err := strconv.ParseUint(categoryIDStr, 10, 32); err == nil {
				categoryIDUint := uint(categoryID)
				artist.CategoryID = &categoryIDUint
			}
		}

		// Start transaction
		tx := db.Begin()

		result := tx.Create(&artist)
		if result.Error != nil {
			tx.Rollback()
			log.Printf("PostAddArtist: Error creating artist: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to create artist: " + result.Error.Error(),
			})
			return
		}

		// Handle unit associations
		if unitIDsStr := strings.TrimSpace(c.PostForm("unit_ids")); unitIDsStr != "" {
			unitIDs := strings.Split(unitIDsStr, ",")
			for _, unitIDStr := range unitIDs {
				if unitID, err := strconv.ParseUint(strings.TrimSpace(unitIDStr), 10, 32); err == nil {
					artistUnit := models.ArtistUnit{
						ArtistID: artist.ArtistID,
						UnitID:   uint(unitID),
					}
					if err := tx.Create(&artistUnit).Error; err != nil {
						log.Printf("PostAddArtist: Warning - failed to associate artist with unit %d: %v", unitID, err)
					}
				}
			}
		}

		tx.Commit()

		log.Printf("PostAddArtist: Successfully created artist '%s' with ID %d", artist.NameOriginal, artist.ArtistID)
		c.Redirect(http.StatusSeeOther, "/admin/add-artist")
	}
}

// View Categories page
func GetViewCategories(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("GetViewCategories: Loading view categories page")

		var categories []models.Category
		db.Find(&categories)

		c.HTML(http.StatusOK, "view-categories.html", gin.H{
			"title":      "View Categories",
			"categories": categories,
		})
	}
}

// View Units page
func GetViewUnits(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("GetViewUnits: Loading view units page")

		var units []models.Unit
		var categories []models.Category
		var artists []models.Artist
		db.Preload("Category").Preload("Artists").Find(&units)
		db.Find(&categories)
		db.Preload("Category").Find(&artists)

		// Convert to JSON for JavaScript
		unitsJSON, _ := json.Marshal(units)
		categoriesJSON, _ := json.Marshal(categories)
		artistsJSON, _ := json.Marshal(artists)

		c.HTML(http.StatusOK, "view-units.html", gin.H{
			"title":          "View Units",
			"units":          units,
			"categories":     categories,
			"artists":        artists,
			"unitsJSON":      string(unitsJSON),
			"categoriesJSON": string(categoriesJSON),
			"artistsJSON":    string(artistsJSON),
		})
	}
}

// View Artists page
func GetViewArtists(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("GetViewArtists: Loading view artists page")

		var artists []models.Artist
		var categories []models.Category
		var units []models.Unit
		db.Preload("Category").Preload("Units.Unit").Find(&artists)
		db.Find(&categories)
		db.Find(&units)

		// Convert to JSON for JavaScript
		artistsJSON, _ := json.Marshal(artists)
		categoriesJSON, _ := json.Marshal(categories)
		unitsJSON, _ := json.Marshal(units)

		c.HTML(http.StatusOK, "view-artists.html", gin.H{
			"title":          "View Artists",
			"artists":        artists,
			"categories":     categories,
			"units":          units,
			"artistsJSON":    string(artistsJSON),
			"categoriesJSON": string(categoriesJSON),
			"unitsJSON":      string(unitsJSON),
		})
	}
}

// Edit Category
func PostEditCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		log.Printf("PostEditCategory: Editing category ID: %s", idParam)

		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			log.Printf("PostEditCategory: Invalid category ID format: %v", err)
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Invalid category ID: " + err.Error(),
			})
			return
		}

		name := strings.TrimSpace(c.PostForm("name"))
		if name == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Category name is required",
			})
			return
		}

		var category models.Category
		result := db.First(&category, uint(id))
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				c.HTML(http.StatusNotFound, "error.html", gin.H{
					"error": "Category not found",
				})
				return
			}
			log.Printf("PostEditCategory: Database error: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to find category: " + result.Error.Error(),
			})
			return
		}

		category.Name = name
		result = db.Save(&category)
		if result.Error != nil {
			log.Printf("PostEditCategory: Error updating category: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to update category: " + result.Error.Error(),
			})
			return
		}

		log.Printf("PostEditCategory: Successfully updated category '%s' with ID %d", category.Name, category.CategoryID)
		c.Redirect(http.StatusSeeOther, "/admin/categories")
	}
}

// Delete Category
func PostDeleteCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		log.Printf("PostDeleteCategory: Deleting category ID: %s", idParam)

		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			log.Printf("PostDeleteCategory: Invalid category ID format: %v", err)
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Invalid category ID: " + err.Error(),
			})
			return
		}

		result := db.Delete(&models.Category{}, uint(id))
		if result.Error != nil {
			log.Printf("PostDeleteCategory: Error deleting category: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to delete category: " + result.Error.Error(),
			})
			return
		}

		log.Printf("PostDeleteCategory: Successfully deleted category with ID %d", id)
		c.Redirect(http.StatusSeeOther, "/admin/categories")
	}
}

// Edit Unit
func PostEditUnit(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		log.Printf("PostEditUnit: Editing unit ID: %s", idParam)

		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			log.Printf("PostEditUnit: Invalid unit ID format: %v", err)
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Invalid unit ID: " + err.Error(),
			})
			return
		}

		nameOrig := strings.TrimSpace(c.PostForm("name_original"))
		if nameOrig == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Unit name is required",
			})
			return
		}

		var unit models.Unit
		result := db.First(&unit, uint(id))
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				c.HTML(http.StatusNotFound, "error.html", gin.H{
					"error": "Unit not found",
				})
				return
			}
			log.Printf("PostEditUnit: Database error: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to find unit: " + result.Error.Error(),
			})
			return
		}

		// Update unit fields
		unit.NameOriginal = nameOrig
		unit.NameEnglish = strings.TrimSpace(c.PostForm("name_english"))
		unit.PrimaryColor = strings.TrimSpace(c.PostForm("primary_color"))
		unit.SecondaryColor = strings.TrimSpace(c.PostForm("secondary_color"))

		// Parse category ID if provided
		unit.CategoryID = nil
		if categoryIDStr := strings.TrimSpace(c.PostForm("category_id")); categoryIDStr != "" {
			if categoryID, err := strconv.ParseUint(categoryIDStr, 10, 32); err == nil {
				categoryIDUint := uint(categoryID)
				unit.CategoryID = &categoryIDUint
			}
		}

		// Start transaction
		tx := db.Begin()

		result = tx.Save(&unit)
		if result.Error != nil {
			tx.Rollback()
			log.Printf("PostEditUnit: Error updating unit: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to update unit: " + result.Error.Error(),
			})
			return
		}

		// Handle artist associations - delete existing and create new ones
		if err := tx.Where("unit_id = ?", unit.UnitID).Delete(&models.ArtistUnit{}).Error; err != nil {
			tx.Rollback()
			log.Printf("PostEditUnit: Error deleting existing artist associations: %v", err)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to update artist associations: " + err.Error(),
			})
			return
		}

		if artistIDsStr := strings.TrimSpace(c.PostForm("artist_ids")); artistIDsStr != "" {
			artistIDs := strings.Split(artistIDsStr, ",")
			for _, artistIDStr := range artistIDs {
				if artistID, err := strconv.ParseUint(strings.TrimSpace(artistIDStr), 10, 32); err == nil {
					artistUnit := models.ArtistUnit{
						ArtistID: uint(artistID),
						UnitID:   unit.UnitID,
					}
					if err := tx.Create(&artistUnit).Error; err != nil {
						log.Printf("PostEditUnit: Warning - failed to associate unit with artist %d: %v", artistID, err)
					}
				}
			}
		}

		tx.Commit()
		log.Printf("PostEditUnit: Successfully updated unit '%s' with ID %d", unit.NameOriginal, unit.UnitID)
		c.Redirect(http.StatusSeeOther, "/admin/units")
	}
}

// Delete Unit
func PostDeleteUnit(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		log.Printf("PostDeleteUnit: Deleting unit ID: %s", idParam)

		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			log.Printf("PostDeleteUnit: Invalid unit ID format: %v", err)
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Invalid unit ID: " + err.Error(),
			})
			return
		}

		// Start transaction to delete unit and its associations
		tx := db.Begin()

		// Delete artist-unit associations
		if err := tx.Where("unit_id = ?", id).Delete(&models.ArtistUnit{}).Error; err != nil {
			tx.Rollback()
			log.Printf("PostDeleteUnit: Error deleting unit associations: %v", err)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to delete unit associations: " + err.Error(),
			})
			return
		}

		// Delete the unit
		result := tx.Delete(&models.Unit{}, uint(id))
		if result.Error != nil {
			tx.Rollback()
			log.Printf("PostDeleteUnit: Error deleting unit: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to delete unit: " + result.Error.Error(),
			})
			return
		}

		tx.Commit()
		log.Printf("PostDeleteUnit: Successfully deleted unit with ID %d", id)
		c.Redirect(http.StatusSeeOther, "/admin/units")
	}
}

// Edit Artist
func PostEditArtist(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		log.Printf("PostEditArtist: Editing artist ID: %s", idParam)

		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			log.Printf("PostEditArtist: Invalid artist ID format: %v", err)
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Invalid artist ID: " + err.Error(),
			})
			return
		}

		nameOrig := strings.TrimSpace(c.PostForm("name_original"))
		if nameOrig == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Artist name is required",
			})
			return
		}

		var artist models.Artist
		result := db.First(&artist, uint(id))
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				c.HTML(http.StatusNotFound, "error.html", gin.H{
					"error": "Artist not found",
				})
				return
			}
			log.Printf("PostEditArtist: Database error: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to find artist: " + result.Error.Error(),
			})
			return
		}

		// Start transaction
		tx := db.Begin()

		// Update artist fields
		artist.NameOriginal = nameOrig
		artist.NameEnglish = strings.TrimSpace(c.PostForm("name_english"))
		artist.PrimaryColor = strings.TrimSpace(c.PostForm("primary_color"))
		artist.SecondaryColor = strings.TrimSpace(c.PostForm("secondary_color"))

		// Parse category ID if provided
		artist.CategoryID = nil
		if categoryIDStr := strings.TrimSpace(c.PostForm("category_id")); categoryIDStr != "" {
			if categoryID, err := strconv.ParseUint(categoryIDStr, 10, 32); err == nil {
				categoryIDUint := uint(categoryID)
				artist.CategoryID = &categoryIDUint
			}
		}

		result = tx.Save(&artist)
		if result.Error != nil {
			tx.Rollback()
			log.Printf("PostEditArtist: Error updating artist: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to update artist: " + result.Error.Error(),
			})
			return
		}

		// Handle unit associations - delete existing and create new ones
		if err := tx.Where("artist_id = ?", artist.ArtistID).Delete(&models.ArtistUnit{}).Error; err != nil {
			tx.Rollback()
			log.Printf("PostEditArtist: Error deleting existing unit associations: %v", err)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to update unit associations: " + err.Error(),
			})
			return
		}

		if unitIDsStr := strings.TrimSpace(c.PostForm("unit_ids")); unitIDsStr != "" {
			unitIDs := strings.Split(unitIDsStr, ",")
			for _, unitIDStr := range unitIDs {
				if unitID, err := strconv.ParseUint(strings.TrimSpace(unitIDStr), 10, 32); err == nil {
					artistUnit := models.ArtistUnit{
						ArtistID: artist.ArtistID,
						UnitID:   uint(unitID),
					}
					if err := tx.Create(&artistUnit).Error; err != nil {
						log.Printf("PostEditArtist: Warning - failed to associate artist with unit %d: %v", unitID, err)
					}
				}
			}
		}

		tx.Commit()
		log.Printf("PostEditArtist: Successfully updated artist '%s' with ID %d", artist.NameOriginal, artist.ArtistID)
		c.Redirect(http.StatusSeeOther, "/admin/artists")
	}
}

// Delete Artist
func PostDeleteArtist(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idParam := c.Param("id")
		log.Printf("PostDeleteArtist: Deleting artist ID: %s", idParam)

		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			log.Printf("PostDeleteArtist: Invalid artist ID format: %v", err)
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Invalid artist ID: " + err.Error(),
			})
			return
		}

		// Start transaction to delete artist and its associations
		tx := db.Begin()

		// Delete artist-unit associations
		if err := tx.Where("artist_id = ?", id).Delete(&models.ArtistUnit{}).Error; err != nil {
			tx.Rollback()
			log.Printf("PostDeleteArtist: Error deleting artist-unit associations: %v", err)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to delete artist associations: " + err.Error(),
			})
			return
		}

		// Delete song-artist associations
		if err := tx.Where("artist_id = ?", id).Delete(&models.SongArtist{}).Error; err != nil {
			tx.Rollback()
			log.Printf("PostDeleteArtist: Error deleting song-artist associations: %v", err)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to delete song associations: " + err.Error(),
			})
			return
		}

		// Delete the artist
		result := tx.Delete(&models.Artist{}, uint(id))
		if result.Error != nil {
			tx.Rollback()
			log.Printf("PostDeleteArtist: Error deleting artist: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to delete artist: " + result.Error.Error(),
			})
			return
		}

		tx.Commit()
		log.Printf("PostDeleteArtist: Successfully deleted artist with ID %d", id)
		c.Redirect(http.StatusSeeOther, "/admin/artists")
	}
}

func PostAddSong(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("PostAddSong: Adding new song")

		nameOriginal := strings.TrimSpace(c.PostForm("name_original"))
		sourceURL := strings.TrimSpace(c.PostForm("source_url"))
		thumbnailURL := strings.TrimSpace(c.PostForm("thumbnail_url"))

		if nameOriginal == "" || sourceURL == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Original name and source URL are required",
			})
			return
		}

		// Auto-extract thumbnail URL from YouTube if not provided
		if thumbnailURL == "" && utils.IsYouTubeURL(sourceURL) {
			var err error
			thumbnailURL, err = utils.ExtractYouTubeThumbnail(sourceURL)
			if err != nil {
				log.Printf("PostAddSong: Failed to extract YouTube thumbnail: %v", err)
				c.HTML(http.StatusBadRequest, "error.html", gin.H{
					"error": "Failed to extract YouTube thumbnail: " + err.Error(),
				})
				return
			}
			log.Printf("PostAddSong: Auto-extracted YouTube thumbnail: %s", thumbnailURL)
		}

		// Require thumbnail URL if still empty
		if thumbnailURL == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error": "Thumbnail URL is required",
			})
			return
		}

		song := models.Song{
			NameOriginal: nameOriginal,
			NameEnglish:  strings.TrimSpace(c.PostForm("name_english")),
			SourceURL:    sourceURL,
			ThumbnailURL: thumbnailURL,
			IsCover:      c.PostForm("is_cover") == "true",
		}

		// Parse category ID if provided
		if categoryIDStr := strings.TrimSpace(c.PostForm("category_id")); categoryIDStr != "" {
			if categoryID, err := strconv.ParseUint(categoryIDStr, 10, 32); err == nil {
				categoryIDUint := uint(categoryID)
				song.CategoryID = &categoryIDUint
			}
		}

		// Start transaction
		tx := db.Begin()

		result := tx.Create(&song)
		if result.Error != nil {
			tx.Rollback()
			log.Printf("PostAddSong: Error creating song: %v", result.Error)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to create song: " + result.Error.Error(),
			})
			return
		}

		// Handle artist associations
		if artistIDsStr := strings.TrimSpace(c.PostForm("artist_ids")); artistIDsStr != "" {
			artistIDs := strings.Split(artistIDsStr, ",")
			for _, artistIDStr := range artistIDs {
				if artistID, err := strconv.ParseUint(strings.TrimSpace(artistIDStr), 10, 32); err == nil {
					songArtist := models.SongArtist{
						SongID:   song.SongID,
						ArtistID: uint(artistID),
					}
					if err := tx.Create(&songArtist).Error; err != nil {
						log.Printf("PostAddSong: Warning - failed to associate song with artist %d: %v", artistID, err)
					}
				}
			}
		}

		// Handle unit associations
		if unitIDsStr := strings.TrimSpace(c.PostForm("unit_ids")); unitIDsStr != "" {
			unitIDs := strings.Split(unitIDsStr, ",")
			for _, unitIDStr := range unitIDs {
				if unitID, err := strconv.ParseUint(strings.TrimSpace(unitIDStr), 10, 32); err == nil {
					songUnit := models.SongUnit{
						SongID: song.SongID,
						UnitID: uint(unitID),
					}
					if err := tx.Create(&songUnit).Error; err != nil {
						log.Printf("PostAddSong: Warning - failed to associate song with unit %d: %v", unitID, err)
					}
				}
			}
		}

		tx.Commit()

		log.Printf("PostAddSong: Successfully created song '%s' with ID %d", song.NameOriginal, song.SongID)
		c.Redirect(http.StatusSeeOther, "/admin/add-song")
	}
}
