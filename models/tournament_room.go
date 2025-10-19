package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// TournamentRoom represents a tournament bracket room
type TournamentRoom struct {
	RoomID           string    `gorm:"primaryKey;size:8"`
	CreatorID        uint      `gorm:"not null"`
	TreeSize         int       `gorm:"not null"` // 16, 32, 64, 128
	CategoryID       *uint     `gorm:"index"`
	VotedOnly        bool      `gorm:"default:false"`
	VotedRatio       *float64  `gorm:"default:null"` // Ratio of voted songs (0.0-1.0), null if VotedOnly is true
	CoversOnly       bool      `gorm:"default:false"`
	VideoSyncEnabled bool      `gorm:"default:true"`
	TreeState        TreeState `gorm:"type:jsonb"` // Store the entire tree structure as JSON
	CurrentMatchID   *string   `gorm:"index"`      // Current active match ID
	Status           string    `gorm:"default:'setup'"` // setup, in_progress, completed
	CreatedAt        time.Time
	LastActive       time.Time `gorm:"index"`

	// Relationships
	Creator  User      `gorm:"foreignKey:CreatorID;references:UserID"`
	Category *Category `gorm:"foreignKey:CategoryID;references:CategoryID;constraint:OnDelete:SET NULL"`
}

// TreeState represents the tournament bracket structure
type TreeState struct {
	Rounds []Round `json:"rounds"` // Array of rounds, starting from round 1 (first matches)
}

// Round represents a single round in the tournament
type Round struct {
	RoundNumber int     `json:"round_number"` // 1, 2, 3, etc. (finals is the last round)
	Matches     []Match `json:"matches"`
}

// Match represents a single match between two songs or the result of previous matches
type Match struct {
	MatchID     string       `json:"match_id"`     // Unique match identifier (e.g., "r1m1", "r2m1")
	Song1       *MatchSong   `json:"song1"`        // First song/competitor
	Song2       *MatchSong   `json:"song2"`        // Second song/competitor
	Winner      *MatchSong   `json:"winner"`       // Winner of the match (null if not yet played)
	UserPicks   []UserPick   `json:"user_picks"`   // User votes for this match
	Status      string       `json:"status"`       // pending, in_progress, completed
	CompletedAt *time.Time   `json:"completed_at"` // When the match was completed
}

// MatchSong represents a song in a match (can be actual song or reference to winner of previous match)
type MatchSong struct {
	SongID           *uint   `json:"song_id"`             // Actual song ID (null if waiting for previous match)
	SongTitle        string  `json:"song_title"`          // Song title
	SongTitleEnglish string  `json:"song_title_english"`  // English title
	Artists          string  `json:"artists"`             // Artist names (comma-separated)
	ThumbnailURL     string  `json:"thumbnail_url"`       // Thumbnail
	SourceURL        string  `json:"source_url"`          // Source URL for video
	EmbedURL         string  `json:"embed_url"`           // Embed URL for video
	FromMatchID      *string `json:"from_match_id"`       // If this is a winner from another match
	AverageRating    float64 `json:"average_rating"`      // Average rating (used for tiebreaker)
	CategoryName     string  `json:"category_name"`       // Category name
	IsCover          bool    `json:"is_cover"`            // Whether this is a cover
}

// UserPick represents a user's pick for a match
type UserPick struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	PickedSongID *uint `json:"picked_song_id"` // Which song they picked (1 or 2 based on position)
	PickedAt time.Time `json:"picked_at"`
}

// Scan implements sql.Scanner for TreeState
func (ts *TreeState) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, ts)
}

// Value implements driver.Valuer for TreeState
func (ts TreeState) Value() (driver.Value, error) {
	return json.Marshal(ts)
}
