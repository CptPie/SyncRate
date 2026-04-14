package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// QuizRoom represents a quiz game room
type QuizRoom struct {
	RoomID           string    `gorm:"primaryKey;size:8"`
	CreatorID        uint      `gorm:"not null"`
	MaxRounds        *int      `gorm:"default:null"`        // nil = endless
	QuestionType     string    `gorm:"not null"`             // title, artist, both
	QuizStyle        string    `gorm:"not null"`             // fixed, progressive
	FixedTimeLimit   int       `gorm:"default:30"`           // seconds for fixed mode (1, 5, 10, 30, 60)
	CategoryID       *uint     `gorm:"index"`
	VotedOnly        bool      `gorm:"default:false"`
	VotedRatio       *float64  `gorm:"default:null"`
	CoversOnly       bool      `gorm:"default:false"`
	FuzzyInput       bool      `gorm:"default:true"`         // use fuzzy search dropdowns vs free text
	OneOfArtists     bool      `gorm:"default:false"`        // correct if at least one artist matches
	QuizState        QuizState `gorm:"type:jsonb"`
	Status           string    `gorm:"default:'setup'"`      // setup, in_progress
	CreatedAt        time.Time
	LastActive       time.Time `gorm:"index"`

	// Relationships
	Creator  User      `gorm:"foreignKey:CreatorID;references:UserID"`
	Category *Category `gorm:"foreignKey:CategoryID;references:CategoryID;constraint:OnDelete:SET NULL"`
}

// QuizState holds all rounds played so far
type QuizState struct {
	Rounds       []QuizRound   `json:"rounds"`
	Scores       []PlayerScore `json:"scores"`
	CurrentRound int           `json:"current_round"` // 1-indexed, 0 = not started
	CurrentStage int           `json:"current_stage"` // index into ProgressiveStages (0-4), only for progressive mode
	StageReady   map[string]bool `json:"stage_ready"` // user_id -> ready to advance the current stage
}

// QuizRound represents a single quiz round
type QuizRound struct {
	RoundNumber    int             `json:"round_number"`
	Song           QuizSong        `json:"song"`
	AudioStartTime int            `json:"audio_start_time"` // seconds into the video
	UserGuesses    []UserGuess     `json:"user_guesses"`
	Status         string          `json:"status"` // waiting, playing, completed
	CompletedAt    *time.Time      `json:"completed_at"`
}

// QuizSong holds the correct answer data for a round
type QuizSong struct {
	SongID           uint     `json:"song_id"`
	TitleOriginal    string   `json:"title_original"`
	TitleEnglish     string   `json:"title_english"`
	Artists          []QuizArtist `json:"artists"`
	Units            []QuizUnit   `json:"units"`
	ThumbnailURL     string   `json:"thumbnail_url"`
	SourceURL        string   `json:"source_url"`
	EmbedURL         string   `json:"embed_url"`
	CategoryName     string   `json:"category_name"`
	IsCover          bool     `json:"is_cover"`
	AverageRating    float64  `json:"average_rating"`
}

// QuizArtist stores artist info for answer matching
type QuizArtist struct {
	ArtistID     uint   `json:"artist_id"`
	NameOriginal string `json:"name_original"`
	NameEnglish  string `json:"name_english"`
}

// QuizUnit stores unit/group info for answer matching
type QuizUnit struct {
	UnitID       uint   `json:"unit_id"`
	NameOriginal string `json:"name_original"`
	NameEnglish  string `json:"name_english"`
}

// UserGuess tracks a user's guess for a round
type UserGuess struct {
	UserID         string     `json:"user_id"`
	Username       string     `json:"username"`
	TitleGuess     string     `json:"title_guess"`
	ArtistGuess    string     `json:"artist_guess"`
	TitleCorrect   bool       `json:"title_correct"`
	ArtistCorrect  bool       `json:"artist_correct"`
	TitleStage     int        `json:"title_stage"`   // stage at which title was guessed correctly (0 = not guessed)
	ArtistStage    int        `json:"artist_stage"`  // stage at which artist was guessed correctly (0 = not guessed)
	TitlePoints      int        `json:"title_points"`
	ArtistPoints     int        `json:"artist_points"`
	TotalPoints      int        `json:"total_points"`
	TitleGuessStage  int        `json:"title_guess_stage"`  // last stage where title was guessed (1-indexed, 0 = not yet)
	ArtistGuessStage int        `json:"artist_guess_stage"` // last stage where artist was guessed (1-indexed, 0 = not yet)
	GuessedAt        *time.Time `json:"guessed_at"`
}

// PlayerScore tracks cumulative scores
type PlayerScore struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Score    int    `json:"score"`
}

// ProgressiveStages defines the time steps and points for progressive mode
var ProgressiveStages = []struct {
	Seconds int
	Points  int
}{
	{1, 10},
	{5, 8},
	{10, 5},
	{30, 3},
	{60, 1},
}

// Scan implements sql.Scanner for QuizState
func (qs *QuizState) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, qs)
}

// Value implements driver.Valuer for QuizState
func (qs QuizState) Value() (driver.Value, error) {
	return json.Marshal(qs)
}
