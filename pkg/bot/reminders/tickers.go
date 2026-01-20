package reminders

import (
	"context"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/bot/training"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

const (
	slotMorningHour   = 8
	slotAfternoonHour = 13
	slotEveningHour   = 20
)

func StartPeriodicMessages(ctx context.Context, b *bot.Bot) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			processReminders(ctx, b, now.UTC())
		}
	}
}

func processReminders(ctx context.Context, b *bot.Bot, now time.Time) {
	var users []db.UserSettings
	if err := db.DB.Find(&users).Error; err != nil {
		logger.Error("failed to fetch users for reminders", "error", err)
		return
	}

	for _, user := range users {
		handleUserReminder(ctx, b, user, now)
	}
}

func handleUserReminder(ctx context.Context, b *bot.Bot, user db.UserSettings, now time.Time) {
	if user.TrainingPaused {
		return
	}
	_, ok := latestDueSlot(now, user)
	if !ok {
		return
	}

	missed := computeMissedCount(user)
	if missed >= 3 {
		if !user.TrainingPaused {
			user.TrainingPaused = true
			user.MissedTrainingSessions = missed
			if err := db.DB.Save(&user).Error; err != nil {
				logger.Error("failed to pause reminders", "user_id", user.UserID, "error", err)
				return
			}
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: user.UserID,
				Text:   "Paused reminders due to inactivity.",
			})
		}
		return
	}

	sent, err := sendTrainingSession(ctx, b, user, now)
	if err != nil {
		logger.Error("failed to send training session", "user_id", user.UserID, "error", err)
		return
	}
	if !sent {
		return
	}

	user.LastTrainingSentAt = &now
	user.MissedTrainingSessions = missed
	if err := db.DB.Save(&user).Error; err != nil {
		logger.Error("failed to update reminder state", "user_id", user.UserID, "error", err)
	}
}

func sendTrainingSession(ctx context.Context, b *bot.Bot, user db.UserSettings, now time.Time) (bool, error) {
	if user.PairsToSend <= 0 {
		return false, nil
	}
	pairs, err := training.SelectSessionPairs(user.UserID, user.PairsToSend, now)
	if err != nil {
		return false, err
	}
	if len(pairs) == 0 {
		return false, nil
	}

	session := training.DefaultManager.StartOrRestart(user.UserID, user.UserID, pairs)
	card := session.CurrentPair()
	if card == nil {
		return false, nil
	}

	prompt := training.BuildPrompt(*card)
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      user.UserID,
		Text:        prompt,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: training.BuildKeyboard(session.CurrentToken()),
	})
	if err != nil {
		return false, err
	}

	training.DefaultManager.SetCurrentMessageID(session, msg.ID)
	training.DefaultManager.SetCurrentPromptText(session, prompt)
	return true, nil
}

func latestDueSlot(now time.Time, user db.UserSettings) (time.Time, bool) {
	offset := time.Duration(user.TimezoneOffsetHours) * time.Hour
	localNow := now.Add(offset)
	year, month, day := localNow.Date()

	var latest time.Time
	consider := func(enabled bool, hour int) {
		if !enabled {
			return
		}
		localSlot := time.Date(year, month, day, hour, 0, 0, 0, time.UTC)
		slotUTC := localSlot.Add(-offset)
		if now.Before(slotUTC) {
			return
		}
		if user.LastTrainingSentAt != nil && !user.LastTrainingSentAt.Before(slotUTC) {
			return
		}
		if latest.IsZero() || slotUTC.After(latest) {
			latest = slotUTC
		}
	}

	consider(user.ReminderMorning, slotMorningHour)
	consider(user.ReminderAfternoon, slotAfternoonHour)
	consider(user.ReminderEvening, slotEveningHour)

	if latest.IsZero() {
		return time.Time{}, false
	}
	return latest, true
}

func computeMissedCount(user db.UserSettings) int {
	missed := user.MissedTrainingSessions
	if user.LastTrainingSentAt == nil {
		return missed
	}
	if user.LastTrainingEngagedAt == nil || user.LastTrainingEngagedAt.Before(*user.LastTrainingSentAt) {
		return missed + 1
	}
	return 0
}
