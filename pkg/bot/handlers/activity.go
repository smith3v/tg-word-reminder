package handlers

import (
	"context"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

type ActivityTracker struct {
	mu      sync.Mutex
	pending map[int64]struct{}
}

func NewActivityTracker() *ActivityTracker {
	return &ActivityTracker{pending: make(map[int64]struct{})}
}

func (t *ActivityTracker) Touch(userID int64) {
	if t == nil || userID == 0 {
		return
	}
	t.mu.Lock()
	if t.pending == nil {
		t.pending = make(map[int64]struct{})
	}
	t.pending[userID] = struct{}{}
	t.mu.Unlock()
}

func (t *ActivityTracker) Flush(ctx context.Context) error {
	if t == nil {
		return nil
	}

	ids := t.snapshot()
	if len(ids) == 0 {
		return nil
	}

	updates := map[string]any{
		"training_paused":          false,
		"missed_training_sessions": 0,
	}
	if err := db.DB.WithContext(ctx).
		Model(&db.UserSettings{}).
		Where("user_id IN ?", ids).
		Updates(updates).Error; err != nil {
		return err
	}

	t.clear(ids)
	return nil
}

func (t *ActivityTracker) snapshot() []int64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.pending) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(t.pending))
	for id := range t.pending {
		ids = append(ids, id)
	}
	return ids
}

func (t *ActivityTracker) clear(ids []int64) {
	if len(ids) == 0 {
		return
	}
	t.mu.Lock()
	for _, id := range ids {
		delete(t.pending, id)
	}
	t.mu.Unlock()
}

func ActivityMiddleware(tracker *ActivityTracker) bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			if update != nil {
				if update.Message != nil && update.Message.From != nil {
					tracker.Touch(update.Message.From.ID)
				}
				if update.CallbackQuery != nil && update.CallbackQuery.From.ID != 0 {
					tracker.Touch(update.CallbackQuery.From.ID)
				}
			}
			next(ctx, b, update)
		}
	}
}

func StartActivityFlusher(ctx context.Context, tracker *ActivityTracker) {
	if tracker == nil {
		return
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := tracker.Flush(ctx); err != nil {
				logger.Error("failed to flush activity touches", "error", err)
			}
		}
	}
}
