package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/bot/feedback"
	"github.com/smith3v/tg-word-reminder/pkg/config"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

const defaultFeedbackTimeoutMinutes = 5

func HandleFeedback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in HandleFeedback")
		return
	}

	if update.Message.Chat.Type != models.ChatTypePrivate {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "The /feedback command works only in private chat.",
		})
		return
	}

	cfg := config.AppConfig.Feedback
	if !cfg.Enabled || len(cfg.AdminIDs) == 0 {
		logger.Error("feedback is not configured", "user_id", update.Message.From.ID)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Feedback is not configured yet. Please try again later.",
		})
		return
	}

	timeoutMinutes := cfg.TimeoutMinutes
	if timeoutMinutes <= 0 {
		timeoutMinutes = defaultFeedbackTimeoutMinutes
	}
	timeout := time.Duration(timeoutMinutes) * time.Minute

	feedback.DefaultManager.Start(update.Message.From.ID, update.Message.Chat.ID, time.Now().UTC(), timeout)

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("Please send your feedback within %d minutes (attachments are ok).", timeoutMinutes),
	})
	if err != nil {
		logger.Error("failed to send feedback prompt", "user_id", update.Message.From.ID, "error", err)
	}
}
