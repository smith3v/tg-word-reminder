package training

import (
	"errors"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const trainingSessionTTL = 24 * time.Hour

func LoadTrainingSession(chatID, userID int64, now time.Time) (*db.TrainingSession, error) {
	if db.DB == nil {
		return nil, nil
	}
	var session db.TrainingSession
	err := db.DB.
		Where("chat_id = ? AND user_id = ? AND expires_at > ?", chatID, userID, now).
		First(&session).Error
	if err == nil {
		return &session, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, err
}

func UpsertTrainingSession(session *db.TrainingSession) error {
	if session == nil || db.DB == nil {
		return nil
	}
	if session.LastActivityAt.IsZero() {
		session.LastActivityAt = time.Now().UTC()
	}
	session.ExpiresAt = session.LastActivityAt.Add(trainingSessionTTL)

	return db.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "chat_id"},
			{Name: "user_id"},
		},
		UpdateAll: true,
	}).Create(session).Error
}

func DeleteTrainingSession(chatID, userID int64) error {
	if db.DB == nil {
		return nil
	}
	return db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).
		Delete(&db.TrainingSession{}).Error
}

func ExpireTrainingSession(chatID, userID int64, _ string) error {
	return DeleteTrainingSession(chatID, userID)
}
