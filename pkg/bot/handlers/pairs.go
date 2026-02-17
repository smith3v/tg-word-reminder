package handlers

import (
	"context"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/bot/training"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
	"gorm.io/gorm"
)

func HandleClear(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in handleClear")
		return
	}

	if tryHandleFeedbackCapture(ctx, b, update) {
		return
	}

	userID := update.Message.From.ID
	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&db.WordPair{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&db.TrainingSession{}).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		logger.Error("failed to clear user data", "user_id", userID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to clear your word pair list. Please try again later.",
		})
		return
	}
	training.DefaultManager.EndAllForUser(userID)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Your word pair list has been cleared.",
	})
}

func HandleGetPair(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in handleGetPair")
		return
	}

	if tryHandleFeedbackCapture(ctx, b, update) {
		return
	}

	now := time.Now().UTC()
	if resumeSession(ctx, b, update.Message.Chat.ID, update.Message.From.ID, now) {
		return
	}
	pairs, err := training.SelectSessionPairs(update.Message.From.ID, 1, now)
	if err != nil {
		logger.Error("failed to load getpair pairs", "user_id", update.Message.From.ID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to start review. Please try again later.",
		})
		return
	}

	if len(pairs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Nothing to review right now.",
		})
		return
	}

	session := training.DefaultManager.StartOrRestart(update.Message.Chat.ID, update.Message.From.ID, pairs)
	card := session.CurrentPair()
	if card == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Nothing to review right now.",
		})
		return
	}

	prompt := training.BuildPrompt(*card)
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        prompt,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: training.BuildKeyboard(session.CurrentToken()),
	})
	if err != nil {
		logger.Error("failed to send getpair prompt", "user_id", update.Message.From.ID, "error", err)
		return
	}
	training.DefaultManager.SetCurrentMessageID(session, msg.ID)
	training.DefaultManager.SetCurrentPromptText(session, prompt)
	training.DefaultManager.Touch(update.Message.Chat.ID, update.Message.From.ID)
}
