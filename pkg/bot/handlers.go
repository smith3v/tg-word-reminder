// pkg/bot/handlers.go
package bot

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/config"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
	"github.com/smith3v/tg-word-reminder/pkg/ui"
	"gorm.io/gorm"
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
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Say /getpair, /setnum, /setfreq, or /clear to use the bot. If you attach a CSV file, I'll upload the word pairs to your account.",
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

	// Read the CSV file
	reader := csv.NewReader(resp.Body)
	reader.Comma = '\t' // Set the delimiter to tab
	records, err := reader.ReadAll()
	if err != nil {
		logger.Error("failed to read CSV file", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to read the CSV file. Please ensure it is in the correct format.",
		})
		return
	}

	// Process each record
	for _, record := range records {
		if len(record) != 2 {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   fmt.Sprintf("Invalid format in record: %v. Please use 'word1\tword2' format.", record),
			})
			continue
		}
		wordPair := db.WordPair{
			UserID: update.Message.From.ID,
			Word1:  strings.TrimSpace(record[0]),
			Word2:  strings.TrimSpace(record[1]),
		}
		if err := db.DB.Create(&wordPair).Error; err != nil {
			logger.Error("failed to create word pair", "user_id", update.Message.From.ID, "error", err)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   fmt.Sprintf("Failed to upload word pair: %v", record),
			})
		}
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Word pairs uploaded successfully.",
	})
}

func HandleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in HandleStart")
		return
	}

	// Check if user settings already exist
	var settings db.UserSettings
	if err := db.DB.Where("user_id = ?", update.Message.From.ID).First(&settings).Error; err != nil {
		if err == gorm.ErrRecordNotFound { // User settings do not exist
			settings = db.UserSettings{
				UserID:          update.Message.From.ID,
				PairsToSend:     1, // Default value
				RemindersPerDay: 1, // Default value
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
		ChatID:    update.Message.Chat.ID,
		Text:      "Welcome\\!\n\nThis bot helps to learn the word pairs or idioms\\, for instance\\, when you learn a language\\. It sends the messages to you with random idioms a few times a day\\. You can choose how often \\(`/setfreq n`\\) and how many \\(`/setnum m`\\) idioms to send every time\\.\n\nYou have to upload your vocabulary first\\. You can send a CSV file here with the word pairs separated by tabs\\. Please refer to [the example](https://raw.githubusercontent.com/smith3v/tg-word-reminder/refs/heads/main/example.csv) for a file format\\, or to [Dutch\\-English vocabulary](https://raw.githubusercontent.com/smith3v/tg-word-reminder/refs/heads/main/dutch-english.csv)\\. ",
		ParseMode: models.ParseModeMarkdown,
	})
	if err != nil {
		logger.Error("failed to send welcome message", "user_id", update.Message.From.ID, "error", err)
	}
}

func HandleSettings(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in HandleSettings")
		return
	}

	var settings db.UserSettings
	if err := db.DB.Where("user_id = ?", update.Message.From.ID).First(&settings).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			_, sendErr := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "Settings not found. Send /start to initialize your account.",
			})
			if sendErr != nil {
				logger.Error("failed to send missing settings message", "user_id", update.Message.From.ID, "error", sendErr)
			}
			return
		}
		logger.Error("failed to load user settings", "user_id", update.Message.From.ID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to load your settings. Please try again later.",
		})
		return
	}

	text, keyboard, err := ui.RenderHome(settings.PairsToSend, settings.RemindersPerDay)
	if err != nil {
		logger.Error("failed to render settings home", "user_id", update.Message.From.ID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to render settings. Please try again later.",
		})
		return
	}

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        text,
		ReplyMarkup: keyboard,
	}); err != nil {
		logger.Error("failed to send settings message", "user_id", update.Message.From.ID, "error", err)
	}
}

func HandleSettingsCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.CallbackQuery == nil {
		logger.Error("invalid update in HandleSettingsCallback")
		return
	}

	callbackID := update.CallbackQuery.ID
	if callbackID != "" {
		if _, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callbackID,
		}); err != nil {
			logger.Error("failed to answer callback query", "error", err)
		}
	}

	action, err := ui.ParseCallbackData(update.CallbackQuery.Data)
	if err != nil {
		logger.Error("failed to parse settings callback", "data", update.CallbackQuery.Data, "error", err)
		return
	}

	if action.Op != ui.OpNone || (action.Screen != ui.ScreenHome && action.Screen != ui.ScreenPairs && action.Screen != ui.ScreenFrequency) {
		return
	}

	message := update.CallbackQuery.Message
	if message.Type != models.MaybeInaccessibleMessageTypeMessage || message.Message == nil {
		logger.Error("callback query message is inaccessible", "user_id", update.CallbackQuery.From.ID)
		return
	}
	msg := message.Message
	if msg.Chat.ID == 0 {
		logger.Error("callback query message chat ID is missing", "user_id", update.CallbackQuery.From.ID)
		return
	}

	var settings db.UserSettings
	if err := db.DB.Where("user_id = ?", update.CallbackQuery.From.ID).First(&settings).Error; err != nil {
		logger.Error("failed to load user settings", "user_id", update.CallbackQuery.From.ID, "error", err)
		return
	}

	var text string
	var keyboard *models.InlineKeyboardMarkup
	switch action.Screen {
	case ui.ScreenHome:
		text, keyboard, err = ui.RenderHome(settings.PairsToSend, settings.RemindersPerDay)
	case ui.ScreenPairs:
		text, keyboard, err = ui.RenderPairs(settings.PairsToSend)
	case ui.ScreenFrequency:
		text, keyboard, err = ui.RenderFreq(settings.RemindersPerDay)
	}
	if err != nil {
		logger.Error("failed to render settings screen", "user_id", update.CallbackQuery.From.ID, "error", err)
		return
	}

	if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      msg.Chat.ID,
		MessageID:   msg.ID,
		Text:        text,
		ReplyMarkup: keyboard,
	}); err != nil {
		logger.Error("failed to edit settings message", "user_id", update.CallbackQuery.From.ID, "error", err)
	}
}

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

func HandleSetNumOfPairs(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in handleSetPairs")
		return
	}

	parts := strings.Fields(update.Message.Text)
	if len(parts) != 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Please use the format: /setnum <number>\n\nTo set the number of pairs in each reminder.",
		})
		return
	}

	pairsCount, err := strconv.Atoi(parts[1])
	if err != nil || pairsCount <= 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Please provide a valid number of pairs in each reminder.",
		})
		return
	}

	settings := db.UserSettings{UserID: update.Message.From.ID, PairsToSend: pairsCount}
	if err := db.DB.Where("user_id = ?", update.Message.From.ID).Assign(settings).FirstOrCreate(&settings).Error; err != nil {
		logger.Error("failed to update user settings", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to update settings. Please try again.",
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("Number of pairs in each reminder has been set to %d.", pairsCount),
	})
}

func HandleSetFrequency(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in handleSetFrequency")
		return
	}

	parts := strings.Fields(update.Message.Text)
	if len(parts) != 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Please use the format: /setfreq <number>\n\nTo set the frequency of reminders per day.",
		})
		return
	}

	frequency, err := strconv.Atoi(parts[1])
	if err != nil || frequency <= 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Please provide a valid number of reminders per day.",
		})
		return
	}

	settings := db.UserSettings{UserID: update.Message.From.ID, RemindersPerDay: frequency}
	if err := db.DB.Where("user_id = ?", update.Message.From.ID).Assign(settings).FirstOrCreate(&settings).Error; err != nil {
		logger.Error("failed to update user settings", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to update settings. Please try again.",
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("Frequency of reminders has been set to %d per day.", frequency),
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

	message := PrepareWordPairMessage(wordPair.Word1, wordPair.Word2)

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      message,
		ParseMode: models.ParseModeMarkdown,
	})
	if err != nil {
		logger.Error("failed to send random word pair message", "user_id", update.Message.From.ID, "error", err)
	}
}
