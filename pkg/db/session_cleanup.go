package db

import (
	"context"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

const SessionCleanupInterval = time.Hour

func CleanupExpiredSessions(now time.Time) (int64, error) {
	if DB == nil {
		return 0, nil
	}
	var deleted int64

	res := DB.Where("expires_at <= ?", now).Delete(&TrainingSession{})
	if res.Error != nil {
		return deleted, res.Error
	}
	deleted += res.RowsAffected

	res = DB.Where("expires_at <= ?", now).Delete(&GameSession{})
	if res.Error != nil {
		return deleted, res.Error
	}
	deleted += res.RowsAffected

	return deleted, nil
}

func StartSessionCleanup(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = SessionCleanupInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := CleanupExpiredSessions(time.Now().UTC()); err != nil {
				logger.Error("failed to cleanup expired sessions", "error", err)
			}
		}
	}
}
