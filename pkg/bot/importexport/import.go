package importexport

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/config"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

func HandleDocumentImport(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.Document == nil || update.Message.From == nil {
		logger.Error("invalid update in HandleDocumentImport")
		return
	}
	if update.Message.Chat.ID == 0 {
		logger.Error("chat ID is zero in HandleDocumentImport")
		return
	}

	doc := update.Message.Document
	logger.Info("Uploading file", "file_name", doc.FileName, "UserID", update.Message.From.ID)

	// Check if the file is a CSV
	if !strings.HasSuffix(doc.FileName, ".csv") {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "The uploaded file is not a CSV. Please upload a valid CSV file.",
		})
		return
	}

	// Download the file
	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: doc.FileID})
	if err != nil {
		logger.Error("failed to get file", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to download the file. Please try again.",
		})
		return
	}

	// Construct the file URL
	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", config.AppConfig.Telegram.Token, file.FilePath)

	// Open the file
	resp, err := http.Get(fileURL)
	if err != nil {
		logger.Error("failed to open file", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to open the file. Please try again.",
		})
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("failed to read CSV file", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to read the CSV file. Please try again.",
		})
		return
	}

	pairs, skipped, err := ParseVocabularyCSV(data)
	if err != nil {
		logger.Error("failed to parse CSV file", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to read the CSV file. Please ensure it is in the correct format.",
		})
		return
	}
	if len(pairs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No valid cards found to import.",
		})
		return
	}

	inserted, updated, err := UpsertWordPairs(update.Message.From.ID, pairs)
	if err != nil {
		logger.Error("failed to import word pairs", "user_id", update.Message.From.ID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to import your cards. Please try again later.",
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("Imported %d new cards, updated %d cards, skipped %d rows.", inserted, updated, skipped),
	})
}
