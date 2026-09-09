package handlers

import (
	"time"

	"github.com/CptPie/SyncRate/models"
)

// Compact projections for the admin listing pages.
//
// Same reasoning as the song list projections in songs.go: these pages inline
// the entire catalogue as JSON for their client-side search, pagination and
// edit modals, and the full models drag along soft-delete columns plus nested
// back-references (Artist.Songs, Album.Songs, Unit.Category.…) that nothing on
// the page reads.
//
// The JSON field names deliberately mirror the model field names, because the
// templates, the card renderers and setupFuzzySearch in
// web/static/js/fuzzy-search.js all index into these objects by name.

// adminCategoryRef is the id/name pair the cards and category filters need.
type adminCategoryRef struct {
	CategoryID uint
	Name       string
}

// adminArtistRef, adminUnitRef and adminAlbumRef are the id/name shapes used
// both for the related-entity lists on a card and as the option lists handed to
// setupFuzzySearch, which reads one id field and one name field.
type adminArtistRef struct {
	ArtistID     uint
	NameOriginal string
	NameEnglish  string
	// CategoryID lets the unit edit modal narrow the artist picker by category.
	CategoryID *uint
}

type adminUnitRef struct {
	UnitID       uint
	NameOriginal string
	NameEnglish  string
}

type adminAlbumRef struct {
	AlbumID      uint
	NameOriginal string
	NameEnglish  string
}

func newAdminCategoryRef(category *models.Category) *adminCategoryRef {
	if category == nil {
		return nil
	}
	return &adminCategoryRef{CategoryID: category.CategoryID, Name: category.Name}
}

func newAdminCategoryRefs(categories []models.Category) []adminCategoryRef {
	refs := make([]adminCategoryRef, 0, len(categories))
	for _, category := range categories {
		refs = append(refs, adminCategoryRef{CategoryID: category.CategoryID, Name: category.Name})
	}
	return refs
}

func newAdminArtistRefs(artists []models.Artist) []adminArtistRef {
	refs := make([]adminArtistRef, 0, len(artists))
	for _, artist := range artists {
		refs = append(refs, adminArtistRef{
			ArtistID:     artist.ArtistID,
			NameOriginal: artist.NameOriginal,
			NameEnglish:  artist.NameEnglish,
			CategoryID:   artist.CategoryID,
		})
	}
	return refs
}

func newAdminUnitRefs(units []models.Unit) []adminUnitRef {
	refs := make([]adminUnitRef, 0, len(units))
	for _, unit := range units {
		refs = append(refs, adminUnitRef{
			UnitID:       unit.UnitID,
			NameOriginal: unit.NameOriginal,
			NameEnglish:  unit.NameEnglish,
		})
	}
	return refs
}

func newAdminAlbumRefs(albums []models.Album) []adminAlbumRef {
	refs := make([]adminAlbumRef, 0, len(albums))
	for _, album := range albums {
		refs = append(refs, adminAlbumRef{
			AlbumID:      album.AlbumID,
			NameOriginal: album.NameOriginal,
			NameEnglish:  album.NameEnglish,
		})
	}
	return refs
}

// adminSongItem backs /admin/view-songs. It carries the ids of related entities
// as well as their names, because the edit modal pre-selects them by id.
type adminSongItem struct {
	SongID       uint
	NameOriginal string
	NameEnglish  string
	SourceURL    string
	ThumbnailURL string
	IsCover      bool
	CategoryID   *uint
	Category     *adminCategoryRef
	Artists      []adminArtistRef
	Units        []adminUnitRef
	Albums       []adminAlbumRef
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func newAdminSongItems(songs []models.Song) []adminSongItem {
	items := make([]adminSongItem, 0, len(songs))
	for _, song := range songs {
		items = append(items, adminSongItem{
			SongID:       song.SongID,
			NameOriginal: song.NameOriginal,
			NameEnglish:  song.NameEnglish,
			SourceURL:    song.SourceURL,
			ThumbnailURL: song.ThumbnailURL,
			IsCover:      song.IsCover,
			CategoryID:   song.CategoryID,
			Category:     newAdminCategoryRef(song.Category),
			Artists:      newAdminArtistRefs(song.Artists),
			Units:        newAdminUnitRefs(song.Units),
			Albums:       newAdminAlbumRefs(song.Albums),
			CreatedAt:    song.CreatedAt,
			UpdatedAt:    song.UpdatedAt,
		})
	}
	return items
}

// adminArtistItem backs /admin/artists. The cards are rendered server-side, so
// CreatedAt/UpdatedAt stay time.Time for the template's .Format calls.
type adminArtistItem struct {
	ArtistID       uint
	NameOriginal   string
	NameEnglish    string
	PrimaryColor   string
	SecondaryColor string
	CategoryID     *uint
	Category       *adminCategoryRef
	Units          []adminUnitRef
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func newAdminArtistItems(artists []models.Artist) []adminArtistItem {
	items := make([]adminArtistItem, 0, len(artists))
	for _, artist := range artists {
		items = append(items, adminArtistItem{
			ArtistID:       artist.ArtistID,
			NameOriginal:   artist.NameOriginal,
			NameEnglish:    artist.NameEnglish,
			PrimaryColor:   artist.PrimaryColor,
			SecondaryColor: artist.SecondaryColor,
			CategoryID:     artist.CategoryID,
			Category:       newAdminCategoryRef(artist.Category),
			Units:          newAdminUnitRefs(artist.Units),
			CreatedAt:      artist.CreatedAt,
			UpdatedAt:      artist.UpdatedAt,
		})
	}
	return items
}

// adminUnitItem backs /admin/units, also rendered server-side.
type adminUnitItem struct {
	UnitID         uint
	NameOriginal   string
	NameEnglish    string
	PrimaryColor   string
	SecondaryColor string
	CategoryID     *uint
	Category       *adminCategoryRef
	Artists        []adminArtistRef
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func newAdminUnitItems(units []models.Unit) []adminUnitItem {
	items := make([]adminUnitItem, 0, len(units))
	for _, unit := range units {
		items = append(items, adminUnitItem{
			UnitID:         unit.UnitID,
			NameOriginal:   unit.NameOriginal,
			NameEnglish:    unit.NameEnglish,
			PrimaryColor:   unit.PrimaryColor,
			SecondaryColor: unit.SecondaryColor,
			CategoryID:     unit.CategoryID,
			Category:       newAdminCategoryRef(unit.Category),
			Artists:        newAdminArtistRefs(unit.Artists),
			CreatedAt:      unit.CreatedAt,
			UpdatedAt:      unit.UpdatedAt,
		})
	}
	return items
}

// adminAlbumItem backs /admin/albums. The card shows only how many songs an
// album has, so SongCount replaces the hydrated tracklist.
type adminAlbumItem struct {
	AlbumID      uint
	NameOriginal string
	NameEnglish  string
	AlbumArtURL  string
	Type         string
	CategoryID   *uint
	Category     *adminCategoryRef
	SongCount    int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func newAdminAlbumItems(albums []models.Album, songCounts map[uint]int64) []adminAlbumItem {
	items := make([]adminAlbumItem, 0, len(albums))
	for _, album := range albums {
		items = append(items, adminAlbumItem{
			AlbumID:      album.AlbumID,
			NameOriginal: album.NameOriginal,
			NameEnglish:  album.NameEnglish,
			AlbumArtURL:  album.AlbumArtURL,
			Type:         album.Type,
			CategoryID:   album.CategoryID,
			Category:     newAdminCategoryRef(album.Category),
			SongCount:    songCounts[album.AlbumID],
			CreatedAt:    album.CreatedAt,
			UpdatedAt:    album.UpdatedAt,
		})
	}
	return items
}
