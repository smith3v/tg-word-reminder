package handlers

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/bot/importexport"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

func HandleExport(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in handleExport")
		return
	}
	if tryHandleFeedbackCapture(ctx, b, update) {
		return
	}
	if update.Message.Chat.Type != models.ChatTypePrivate {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "The /export command works only in private chat.",
		})
		return
	}

	var pairs []db.WordPair
	if err := db.DB.Where("user_id = ?", update.Message.From.ID).Order("word1 ASC, id ASC").Find(&pairs).Error; err != nil {
		logger.Error("failed to fetch word pairs for export", "user_id", update.Message.From.ID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to export your vocabulary. Please try again later.",
		})
		return
	}
	if len(pairs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "You have no vocabulary to export.",
		})
		return
	}

	importexport.SortPairsForExport(pairs)
	data, err := importexport.BuildExportCSV(pairs)
	if err != nil {
		logger.Error("failed to build export CSV", "user_id", update.Message.From.ID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to export your vocabulary. Please try again later.",
		})
		return
	}

	filename := importexport.ExportFilename(time.Now())
	caption := fmt.Sprintf("Your vocabulary export (%d cards).", len(pairs))
	_, err = b.SendDocument(ctx, &bot.SendDocumentParams{
		ChatID: update.Message.Chat.ID,
		Document: &models.InputFileUpload{
			Filename: filename,
			Data:     bytes.NewReader(data),
		},
		Caption: caption,
	})
	if err != nil {
		logger.Error("failed to send export document", "user_id", update.Message.From.ID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to export your vocabulary. Please try again later.",
		})
	}
}
