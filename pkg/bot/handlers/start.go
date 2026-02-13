package handlers

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/bot/onboarding"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

func HandleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in HandleStart")
		return
	}

	if tryHandleFeedbackCapture(ctx, b, update) {
		return
	}

	hasData, err := onboarding.HasExistingUserData(update.Message.From.ID)
	if err != nil {
		logger.Error("failed to check existing onboarding data", "user_id", update.Message.From.ID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to start onboarding. Please try again later.",
		})
		return
	}
	if hasData {
		if err := onboarding.SetAwaitingResetPhrase(update.Message.From.ID); err != nil {
			logger.Error("failed to set onboarding reset phrase state", "user_id", update.Message.From.ID, "error", err)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "Failed to start onboarding. Please try again later.",
			})
			return
		}
		text, keyboard := onboarding.RenderResetWarningPrompt()
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      update.Message.Chat.ID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		return
	}
	if err := sendOnboardingLearningPrompt(ctx, b, update.Message.Chat.ID, update.Message.From.ID); err != nil {
		logger.Error("failed to start onboarding wizard", "user_id", update.Message.From.ID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to start onboarding. Please try again later.",
		})
	}
}
