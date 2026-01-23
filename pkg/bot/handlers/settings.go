package handlers

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
	"github.com/smith3v/tg-word-reminder/pkg/ui"
	"gorm.io/gorm"
)

const (
	MinPairsPerReminder = 0
	MaxPairsPerReminder = 10

	MinTimezoneOffset = -12
	MaxTimezoneOffset = 14
)

var (
	ErrBelowMin      = errors.New("value below minimum")
	ErrAboveMax      = errors.New("value above maximum")
	ErrInvalidAction = errors.New("invalid settings action")
)

func HandleSettings(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in HandleSettings")
		return
	}

	if tryHandleFeedbackCapture(ctx, b, update) {
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

	text, keyboard, err := ui.RenderHome(
		settings.PairsToSend,
		settings.ReminderMorning,
		settings.ReminderAfternoon,
		settings.ReminderEvening,
		settings.TimezoneOffsetHours,
	)
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
		}
		logger.Error("failed to apply settings action", "user_id", update.CallbackQuery.From.ID, "error", err)
		answerCallback("Unknown command")
		return
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
		text, keyboard, err = ui.RenderHome(
			newSettings.PairsToSend,
			newSettings.ReminderMorning,
			newSettings.ReminderAfternoon,
			newSettings.ReminderEvening,
			newSettings.TimezoneOffsetHours,
		)
	case ui.ScreenPairs:
		text, keyboard, err = ui.RenderPairs(newSettings.PairsToSend)
	case ui.ScreenSlots:
		text, keyboard, err = ui.RenderSlots(newSettings.ReminderMorning, newSettings.ReminderAfternoon, newSettings.ReminderEvening)
	case ui.ScreenTimezone:
		text, keyboard, err = ui.RenderTimezone(newSettings.TimezoneOffsetHours)
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

func ApplyAction(settings db.UserSettings, action ui.Action) (db.UserSettings, ui.Screen, bool, error) {
	switch action.Screen {
	case ui.ScreenHome:
		if action.Op != ui.OpNone {
			return settings, ui.ScreenHome, false, ErrInvalidAction
		}
		return settings, ui.ScreenHome, false, nil
	case ui.ScreenClose:
		if action.Op != ui.OpNone {
			return settings, ui.ScreenClose, false, ErrInvalidAction
		}
		return settings, ui.ScreenClose, false, nil
	case ui.ScreenPairs:
		next, changed, err := applyValue(settings.PairsToSend, action, MinPairsPerReminder, MaxPairsPerReminder)
		if err != nil {
			return settings, ui.ScreenPairs, false, err
		}
		newSettings := settings
		newSettings.PairsToSend = next
		return newSettings, ui.ScreenPairs, changed, nil
	case ui.ScreenSlots:
		if action.Op == ui.OpNone {
			return settings, ui.ScreenSlots, false, nil
		}
		if action.Op != ui.OpToggle {
			return settings, ui.ScreenSlots, false, ErrInvalidAction
		}
		newSettings := settings
		switch action.Value {
		case ui.SlotMorning:
			newSettings.ReminderMorning = !settings.ReminderMorning
		case ui.SlotAfternoon:
			newSettings.ReminderAfternoon = !settings.ReminderAfternoon
		case ui.SlotEvening:
			newSettings.ReminderEvening = !settings.ReminderEvening
		default:
			return settings, ui.ScreenSlots, false, ErrInvalidAction
		}
		return newSettings, ui.ScreenSlots, true, nil
	case ui.ScreenTimezone:
		next, changed, err := applyValue(settings.TimezoneOffsetHours, action, MinTimezoneOffset, MaxTimezoneOffset)
		if err != nil {
			return settings, ui.ScreenTimezone, false, err
		}
		newSettings := settings
		newSettings.TimezoneOffsetHours = next
		return newSettings, ui.ScreenTimezone, changed, nil
	default:
		return settings, ui.ScreenHome, false, ErrInvalidAction
	}
}

func applyValue(current int, action ui.Action, min, max int) (int, bool, error) {
	switch action.Op {
	case ui.OpNone:
		return current, false, nil
	case ui.OpInc:
		next := current + 1
		return clampValue(current, next, min, max)
	case ui.OpDec:
		next := current - 1
		return clampValue(current, next, min, max)
	case ui.OpSet:
		return clampValue(current, action.Value, min, max)
	default:
		return current, false, ErrInvalidAction
	}
}

func clampValue(current, next, min, max int) (int, bool, error) {
	if next < min {
		return current, false, ErrBelowMin
	}
	if next > max {
		return current, false, ErrAboveMax
	}
	if next == current {
		return current, false, nil
	}
	return next, true, nil
}

func boundsForScreen(screen ui.Screen) (int, int, bool) {
	switch screen {
	case ui.ScreenPairs:
		return MinPairsPerReminder, MaxPairsPerReminder, true
	case ui.ScreenTimezone:
		return MinTimezoneOffset, MaxTimezoneOffset, true
	default:
		return 0, 0, false
	}
}
