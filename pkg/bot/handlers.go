// pkg/bot/handlers.go
package bot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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
			ChatID: update.Message.Chat.ID,
			Text: "Commands:\n" +
				"\\* /start: initialize your account\\.\n" +
				"\\* /getpair: get a random word pair\\.\n" +
				"\\* /game: start a quiz session\\.\n" +
				"\\* /settings: configure reminders and pair counts\\.\n" +
				"\\* /export: download your vocabulary\\.\n\n" +
				"\\* /clear: remove all uploaded word pairs\\.\n" +
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

	pairs, skipped, err := parseVocabularyCSV(data)
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

	inserted, updated, err := upsertWordPairs(update.Message.From.ID, pairs)
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
		ChatID: update.Message.Chat.ID,
		Text: "Welcome\\!\n\n" +
			"This bot helps you practice word pairs with short quizzes and reminders\\.\n" +
			"Start by uploading your vocabulary as a CSV file \\(comma\\, tab\\, or semicolon separated\\)\\.\n" +
			"See the [example format](https://raw.githubusercontent.com/smith3v/tg-word-reminder/refs/heads/main/example.csv) or the [Dutch\\-English sample](https://raw.githubusercontent.com/smith3v/tg-word-reminder/refs/heads/main/dutch-english.csv)\\.\n\n" +
			"Use /settings to adjust reminder frequency and pair count\\, /getpair for a quick random pair\\, /export to download your vocabulary\\, or /game to start a quiz session\\.",
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

func HandleExport(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in handleExport")
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

	sortPairsForExport(pairs)
	data, err := buildExportCSV(pairs)
	if err != nil {
		logger.Error("failed to build export CSV", "user_id", update.Message.From.ID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to export your vocabulary. Please try again later.",
		})
		return
	}

	filename := exportFilename(time.Now())
	caption := fmt.Sprintf("Your vocabulary export (%d pairs).", len(pairs))
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

func HandleGameCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.CallbackQuery == nil {
		logger.Error("invalid update in HandleGameCallback")
		return
	}

	callbackID := update.CallbackQuery.ID
	answerCallback := func(text string) {
		if callbackID == "" {
			return
		}
		if _, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callbackID,
			Text:            text,
		}); err != nil {
			logger.Error("failed to answer game callback query", "error", err)
		}
	}

	data := update.CallbackQuery.Data
	if !strings.HasPrefix(data, "g:r:") {
		answerCallback("Not active")
		return
	}
	token := strings.TrimPrefix(data, "g:r:")
	if token == "" {
		answerCallback("Not active")
		return
	}

	message := update.CallbackQuery.Message
	if message.Type != models.MaybeInaccessibleMessageTypeMessage || message.Message == nil {
		answerCallback("Message missing")
		return
	}
	msg := message.Message
	if msg.Chat.ID == 0 {
		answerCallback("Message missing")
		return
	}

	chatID := msg.Chat.ID
	userID := update.CallbackQuery.From.ID
	result := gameManager.ResolveRevealAttempt(chatID, userID, token, msg.ID)
	if !result.handled {
		notice := result.notice
		if notice == "" {
			notice = "Not active"
		}
		answerCallback(notice)
		return
	}

	revealText := formatGameRevealText(result.card, "👀")
	if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: result.promptMessageID,
		Text:      revealText,
		ParseMode: models.ParseModeMarkdown,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{},
		},
	}); err != nil {
		logger.Error("failed to edit game reveal", "user_id", userID, "error", err)
	}

	answerCallback("")

	if result.statsText != "" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   result.statsText,
		})
		return
	}

	if result.nextCard != nil {
		nextMsg, err := sendGamePrompt(ctx, b, chatID, result.nextCard, result.nextToken, false)
		if err != nil {
			logger.Error("failed to send next game prompt", "user_id", userID, "error", err)
			return
		}
		gameManager.SetCurrentMessageIDForToken(chatID, userID, result.nextToken, nextMsg.ID)
	}
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
	revealText := formatGameRevealText(result.card, revealSuffix)
	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: result.promptMessageID,
		Text:      revealText,
		ParseMode: models.ParseModeMarkdown,
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

func formatGameRevealText(card Card, suffix string) string {
	shown := bot.EscapeMarkdown(card.Shown)
	expected := bot.EscapeMarkdown(card.Expected)
	return fmt.Sprintf("%s — ||%s|| %s", shown, expected, suffix)
}
