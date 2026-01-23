package handlers

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
	"gorm.io/gorm"
)

func HandleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in HandleStart")
		return
	}

	if tryHandleFeedbackCapture(ctx, b, update) {
		return
	}

	// Check if user settings already exist
	var settings db.UserSettings
	if err := db.DB.Where("user_id = ?", update.Message.From.ID).First(&settings).Error; err != nil {
		if err == gorm.ErrRecordNotFound { // User settings do not exist
			settings = db.UserSettings{
				UserID:                 update.Message.From.ID,
				PairsToSend:            5,
				RemindersPerDay:        1,
				ReminderMorning:        false,
				ReminderAfternoon:      false,
				ReminderEvening:        true,
				TimezoneOffsetHours:    0,
				MissedTrainingSessions: 0,
				TrainingPaused:         false,
			}
			if err := db.DB.Create(&settings).Error; err != nil {
				logger.Error("failed to create user settings", "user_id", update.Message.From.ID, "error", err)
				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   "Failed to create your settings. Please try again later.",
				})
				return
			}
		} else {
			logger.Error("failed to check user settings", "error", err)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "An error occurred while checking your settings. Please try again later.",
			})
			return
		}
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: "Welcome\\!\n\n" +
			"This bot helps you practice word pairs with short quizzes and reminders\\.\n" +
			"Start by uploading your vocabulary as a CSV file \\(comma\\, tab\\, or semicolon separated\\)\\.\n" +
			"See the [example format](https://raw.githubusercontent.com/smith3v/tg-word-reminder/refs/heads/main/example.csv) or the [Dutch\\-English sample](https://raw.githubusercontent.com/smith3v/tg-word-reminder/refs/heads/main/dutch-english.csv)\\.\n\n" +
			"Use /settings to adjust reminder frequency and pair count\\, /getpair for a quick random pair\\, /export to download your vocabulary\\, or /game to start a quiz session\\.",
		ParseMode: models.ParseModeMarkdown,
	})
	if err != nil {
		logger.Error("failed to send welcome message", "user_id", update.Message.From.ID, "error", err)
	}
}
