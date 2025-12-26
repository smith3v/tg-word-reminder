// pkg/bot/handlers.go
package bot

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
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
		if update.Message.Text != "" {
			if handled := handleGameTextAttempt(ctx, b, update); handled {
				return
			}
		}
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "Type /start to initialize the bot\\, /getpair to get a random pair\\, /settings to configure your preferences\\, or /clear to clean up your vocabulary\\.\n\nIf you attach a CSV file here\\, I\\'ll upload the word pairs to your account\\. Please refer to [the example](https://raw.githubusercontent.com/smith3v/tg-word-reminder/refs/heads/main/example.csv) for a file format\\, or to [Dutch\\-English vocabulary example](https://raw.githubusercontent.com/smith3v/tg-word-reminder/refs/heads/main/dutch-english.csv)\\.",
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
		Text:      "Welcome\\!\n\nThis bot helps to learn the word pairs or idioms\\, for instance\\, when you learn a language\\. It sends the messages to you with random idioms a few times a day\\. You can configure reminder frequency and pair counts with /settings\\.\n\nTo make it useful, you have to upload your vocabulary first\\. You can submit a CSV file here with the word pairs separated by tabs\\. Please refer to [the example](https://raw.githubusercontent.com/smith3v/tg-word-reminder/refs/heads/main/example.csv) for a file format\\, or to [Dutch\\-English vocabulary](https://raw.githubusercontent.com/smith3v/tg-word-reminder/refs/heads/main/dutch-english.csv)\\. ",
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
	answered := false
	answerCallback := func(text string) {
		if answered || callbackID == "" {
			return
		}
		if _, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callbackID,
			Text:            text,
		}); err != nil {
			logger.Error("failed to answer callback query", "error", err)
		}
		answered = true
	}

	action, err := ui.ParseCallbackData(update.CallbackQuery.Data)
	if err != nil {
		logger.Error("failed to parse settings callback", "data", update.CallbackQuery.Data, "error", err)
		answerCallback("Unknown command")
		return
	}

	message := update.CallbackQuery.Message
	if message.Type != models.MaybeInaccessibleMessageTypeMessage || message.Message == nil {
		logger.Error("callback query message is inaccessible", "user_id", update.CallbackQuery.From.ID)
		answerCallback("Message is not available")
		return
	}
	msg := message.Message
	if msg.Chat.ID == 0 {
		logger.Error("callback query message chat ID is missing", "user_id", update.CallbackQuery.From.ID)
		answerCallback("Message is not available")
		return
	}

	var settings db.UserSettings
	if err := db.DB.Where("user_id = ?", update.CallbackQuery.From.ID).First(&settings).Error; err != nil {
		logger.Error("failed to load user settings", "user_id", update.CallbackQuery.From.ID, "error", err)
		answerCallback("Failed to load settings")
		return
	}

	newSettings, nextScreen, changed, err := ApplyAction(settings, action)
	if err != nil {
		if errors.Is(err, ErrBelowMin) || errors.Is(err, ErrAboveMax) {
			min, max, ok := boundsForScreen(action.Screen)
			if ok {
				if errors.Is(err, ErrBelowMin) {
					answerCallback(fmt.Sprintf("Minimum is %d", min))
				} else {
					answerCallback(fmt.Sprintf("Maximum is %d", max))
				}
			} else {
				answerCallback("Unknown command")
			}
			return
		} else {
			logger.Error("failed to apply settings action", "user_id", update.CallbackQuery.From.ID, "error", err)
			answerCallback("Unknown command")
			return
		}
	}

	if changed {
		if err := db.DB.Save(&newSettings).Error; err != nil {
			logger.Error("failed to save user settings", "user_id", update.CallbackQuery.From.ID, "error", err)
			answerCallback("Failed to save settings")
			return
		}
	}

	if !answered {
		answerCallback("")
	}

	if !changed && action.Op == ui.OpSet {
		return
	}

	var text string
	var keyboard *models.InlineKeyboardMarkup
	switch nextScreen {
	case ui.ScreenHome:
		text, keyboard, err = ui.RenderHome(newSettings.PairsToSend, newSettings.RemindersPerDay)
	case ui.ScreenPairs:
		text, keyboard, err = ui.RenderPairs(newSettings.PairsToSend)
	case ui.ScreenFrequency:
		text, keyboard, err = ui.RenderFreq(newSettings.RemindersPerDay)
	case ui.ScreenClose:
		text = "Settings saved ✅"
		keyboard = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{},
		}
	default:
		logger.Error("unknown settings screen", "screen", nextScreen)
		return
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

func HandleGameStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in HandleGameStart")
		return
	}
	if update.Message.Chat.Type != models.ChatTypePrivate {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "The /game command works only in private chat.",
		})
		return
	}

	pairs, err := selectRandomPairs(update.Message.From.ID, DeckPairs)
	if err != nil {
		logger.Error("failed to fetch word pairs for game", "user_id", update.Message.From.ID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to start the game. Please try again later.",
		})
		return
	}
	if len(pairs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "You have no word pairs saved. Please upload some word pairs first.",
		})
		return
	}

	session := gameManager.StartOrRestart(update.Message.Chat.ID, update.Message.From.ID, pairs)
	if session.currentCard == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No word pairs are available to start a game.",
		})
		return
	}

	msg, err := sendGamePrompt(ctx, b, update.Message.Chat.ID, session.currentCard, session.currentToken, true)
	if err != nil {
		logger.Error("failed to send game prompt", "user_id", update.Message.From.ID, "error", err)
		return
	}
	gameManager.SetCurrentMessageID(session, msg.ID)
}

func handleGameTextAttempt(ctx context.Context, b *bot.Bot, update *models.Update) bool {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		return false
	}
	text := strings.TrimSpace(update.Message.Text)
	if text == "" {
		return false
	}
	if strings.HasPrefix(text, "/game") {
		HandleGameStart(ctx, b, update)
		return true
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	session := gameManager.GetSession(chatID, userID)
	if strings.HasPrefix(text, "/") {
		return session != nil
	}
	if session == nil {
		return false
	}

	result := gameManager.ResolveTextAttempt(chatID, userID, text)
	if !result.handled {
		return true
	}

	revealSuffix := "❌"
	if result.correct {
		revealSuffix = "✅"
	}
	revealText := fmt.Sprintf("%s — %s %s", result.card.Shown, result.card.Expected, revealSuffix)
	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: result.promptMessageID,
		Text:      revealText,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{},
		},
	})
	if err != nil {
		logger.Error("failed to edit game prompt", "user_id", userID, "error", err)
	}

	if result.statsText != "" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   result.statsText,
		})
		return true
	}

	if result.nextCard != nil {
		msg, err := sendGamePrompt(ctx, b, chatID, result.nextCard, result.nextToken, false)
		if err != nil {
			logger.Error("failed to send next game prompt", "user_id", userID, "error", err)
			return true
		}
		gameManager.SetCurrentMessageIDForToken(chatID, userID, result.nextToken, msg.ID)
	}

	return true
}

func sendGamePrompt(ctx context.Context, b *bot.Bot, chatID int64, card *Card, token string, includeHint bool) (*models.Message, error) {
	if card == nil {
		return nil, fmt.Errorf("missing card")
	}
	prompt := fmt.Sprintf("%s → ?", card.Shown)
	if includeHint {
		prompt = fmt.Sprintf("%s\n(reply with the missing word, or tap 👀 to reveal — counts as a miss)", prompt)
	}
	callbackData := fmt.Sprintf("g:r:%s", token)
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text:         "👀",
					CallbackData: callbackData,
				},
			},
		},
	}
	return b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        prompt,
		ReplyMarkup: keyboard,
	})
}
