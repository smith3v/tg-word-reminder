package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/bot/importexport"
	"github.com/smith3v/tg-word-reminder/pkg/config"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

func DefaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil {
		logger.Error("received invalid update in defaultHandler")
		return
	}

	// Check if Chat is zero value
	if update.Message.Chat.ID == 0 {
		logger.Error("chat ID is zero in defaultHandler")
		return
	}

	// Check if the message contains a document (file)
	if update.Message.Document == nil {
		if update.Message.Text != "" {
			if handled := handleGameTextAttempt(ctx, b, update); handled {
				return
			}
		}
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: "Commands:\n" +
				"\\* /start: initialize your account\\.\n" +
				"\\* /getpair: get a random word pair\\.\n" +
				"\\* /game: start a quiz session\\.\n" +
				"\\* /settings: configure reminders and pair counts\\.\n" +
				"\\* /export: download your vocabulary\\.\n" +
				"\\* /clear: remove all uploaded word pairs\\.\n\n" +
				"If you attach a CSV file here\\, I\\'ll upload the word pairs to your account\\. Please refer to [the example](https://raw.githubusercontent.com/smith3v/tg-word-reminder/refs/heads/main/example.csv) for a file format\\, or to [Dutch\\-English vocabulary example](https://raw.githubusercontent.com/smith3v/tg-word-reminder/refs/heads/main/dutch-english.csv)\\.",
			ParseMode: models.ParseModeMarkdown,
		})
		if err != nil {
			logger.Error("failed to send message in defaultHandler", "error", err)
		}
		return
	}

	logger.Info("Uploading file", "file_name", update.Message.Document.FileName, "UserID", update.Message.From.ID)

	// Check if the file is a CSV
	if !strings.HasSuffix(update.Message.Document.FileName, ".csv") {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "The uploaded file is not a CSV. Please upload a valid CSV file.",
		})
		return
	}

	// Download the file
	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: update.Message.Document.FileID})
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

	pairs, skipped, err := importexport.ParseVocabularyCSV(data)
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
			Text:   "No valid word pairs found to import.",
		})
		return
	}

	inserted, updated, err := importexport.UpsertWordPairs(update.Message.From.ID, pairs)
	if err != nil {
		logger.Error("failed to import word pairs", "user_id", update.Message.From.ID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to import your word pairs. Please try again later.",
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("Imported %d new pairs, updated %d pairs, skipped %d rows.", inserted, updated, skipped),
	})
}
