package handlers

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/bot/importexport"
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

	if tryHandleFeedbackCapture(ctx, b, update) {
		return
	}
	if tryHandleOnboardingResetPhrase(ctx, b, update) {
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
				"\\* /review: start a review session\\.\n" +
				"\\* /settings: configure reminders and pair counts\\.\n" +
				"\\* /feedback: send feedback to the admins\\.\n" +
				"\\* /export: download your vocabulary\\.\n" +
				"\\* /clear: remove all uploaded word pairs\\.\n\n" +
				"If you attach a CSV file here\\, I\\'ll upload the word pairs to your account\\. Please refer to [the example](https://raw.githubusercontent.com/smith3v/tg-word-reminder/refs/heads/main/vocabularies/example.csv) for a file format\\, or to [Dutch\\-English vocabulary example](https://raw.githubusercontent.com/smith3v/tg-word-reminder/refs/heads/main/vocabularies/dutch-english.csv)\\.",
			ParseMode: models.ParseModeMarkdown,
		})
		if err != nil {
			logger.Error("failed to send message in defaultHandler", "error", err)
		}
		return
	}

	importexport.HandleDocumentImport(ctx, b, update)
}
