// pkg/db/models.go
package db

import (
	"time"

	"gorm.io/datatypes"
)

const SrsNewRankMax = 1000000000

type WordPair struct {
	ID                uint      `gorm:"primaryKey"`
	UserID            int64     `gorm:"index;uniqueIndex:idx_user_word1;index:idx_user_due"` // To keep pairs separate for each user
	Word1             string    `gorm:"not null;uniqueIndex:idx_user_word1"`
	Word2             string    `gorm:"not null"`
	SrsState          string    `gorm:"not null;default:new"`
	SrsDueAt          time.Time `gorm:"index:idx_user_due"`
	SrsLastReviewedAt *time.Time
	SrsIntervalDays   int     `gorm:"not null;default:0"`
	SrsEase           float64 `gorm:"not null;default:2.5"`
	SrsStep           int     `gorm:"not null;default:0"`
	SrsReps           int     `gorm:"not null;default:0"`
	SrsLapses         int     `gorm:"not null;default:0"`
	SrsNewRank        int     `gorm:"not null;default:0"`
}

type UserSettings struct {
	ID                     uint  `gorm:"primaryKey"`
	UserID                 int64 `gorm:"index"`
	PairsToSend            int   `gorm:"default:1"` // Default to sending 1 pair
	RemindersPerDay        int   `gorm:"default:1"` // Deprecated: retained for migration during development
	ReminderMorning        bool  `gorm:"not null;default:false"`
	ReminderAfternoon      bool  `gorm:"not null;default:false"`
	ReminderEvening        bool  `gorm:"not null;default:false"`
	TimezoneOffsetHours    int   `gorm:"not null;default:0"`
	MissedTrainingSessions int   `gorm:"not null;default:0"`
	TrainingPaused         bool  `gorm:"not null;default:false"`
	LastTrainingSentAt     *time.Time
	LastTrainingEngagedAt  *time.Time
}

type GameSession struct {
	ID              uint      `gorm:"primaryKey"`
	UserID          int64     `gorm:"index"`
	SessionDate     time.Time `gorm:"type:date;not null"`
	StartedAt       time.Time `gorm:"not null"`
	EndedAt         *time.Time
	DurationSeconds *int
	EndedReason     *string
	CorrectCount    int `gorm:"not null;default:0"`
	AttemptCount    int `gorm:"not null;default:0"`
}

type TrainingSession struct {
	ID               uint           `gorm:"primaryKey"`
	ChatID           int64          `gorm:"index;uniqueIndex:idx_training_session_user_chat"`
	UserID           int64          `gorm:"index;uniqueIndex:idx_training_session_user_chat"`
	PairIDs          datatypes.JSON `gorm:"not null"`
	CurrentIndex     int            `gorm:"not null;default:0"`
	CurrentToken     string         `gorm:"not null;default:''"`
	CurrentMessageID int            `gorm:"not null;default:0"`
	LastActivityAt   time.Time      `gorm:"not null"`
	ExpiresAt        time.Time      `gorm:"not null"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type GameSessionState struct {
	ID               uint           `gorm:"primaryKey"`
	ChatID           int64          `gorm:"index;uniqueIndex:idx_game_session_state_user_chat"`
	UserID           int64          `gorm:"index;uniqueIndex:idx_game_session_state_user_chat"`
	PairIDs          datatypes.JSON `gorm:"not null"`
	CurrentIndex     int            `gorm:"not null;default:0"`
	CurrentToken     string         `gorm:"not null;default:''"`
	CurrentMessageID int            `gorm:"not null;default:0"`
	ScoreCorrect     int            `gorm:"not null;default:0"`
	ScoreAttempted   int            `gorm:"not null;default:0"`
	LastActivityAt   time.Time      `gorm:"not null"`
	ExpiresAt        time.Time      `gorm:"not null"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (GameSessionState) TableName() string {
	return "game_session_states"
}
