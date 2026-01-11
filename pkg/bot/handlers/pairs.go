package handlers

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/bot/game"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

func HandleClear(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in handleClear")
		return
	}

	db.DB.Where("user_id = ?", update.Message.From.ID).Delete(&db.WordPair{})
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

	var wordPair db.WordPair
	if err := db.DB.Where("user_id = ?", update.Message.From.ID).Order("RANDOM()").Limit(1).Find(&wordPair).Error; err != nil {
		logger.Error("failed to fetch random word pair for user", "user_id", update.Message.From.ID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to retrieve a word pair. Please try again later.",
		})
		return
	}

	if (wordPair == db.WordPair{}) {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "You have no word pairs saved. Please upload some word pairs first.",
		})
		return
	}

	message := game.PrepareWordPairMessage(wordPair.Word1, wordPair.Word2)

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      message,
		ParseMode: models.ParseModeMarkdown,
	})
	if err != nil {
		logger.Error("failed to send random word pair message", "user_id", update.Message.From.ID, "error", err)
	}
}
