package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/bot/game"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

func HandleGameStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in HandleGameStart")
		return
	}

	if tryHandleFeedbackCapture(ctx, b, update) {
		return
	}

	if update.Message.Chat.Type != models.ChatTypePrivate {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "The /game command works only in private chat.",
		})
		return
	}

	pairs, err := game.SelectRandomPairs(update.Message.From.ID, game.DeckPairs)
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

	session := game.DefaultManager.StartOrRestart(update.Message.Chat.ID, update.Message.From.ID, pairs)
	if session.CurrentCard() == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No word pairs are available to start a game.",
		})
		return
	}

	msg, err := sendGamePrompt(ctx, b, update.Message.Chat.ID, session.CurrentCard(), session.CurrentToken(), true)
	if err != nil {
		logger.Error("failed to send game prompt", "user_id", update.Message.From.ID, "error", err)
		return
	}
	game.DefaultManager.SetCurrentMessageID(session, msg.ID)
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
	result := game.DefaultManager.ResolveRevealAttempt(chatID, userID, token, msg.ID)
	if !result.Handled {
		notice := result.Notice
		if notice == "" {
			notice = "Not active"
		}
		answerCallback(notice)
		return
	}

	revealText := formatGameRevealText(result.Card, "👀")
	if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: result.PromptMessageID,
		Text:      revealText,
		ParseMode: models.ParseModeMarkdown,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{},
		},
	}); err != nil {
		logger.Error("failed to edit game reveal", "user_id", userID, "error", err)
	}

	answerCallback("")

	if result.StatsText != "" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   result.StatsText,
		})
		return
	}

	if result.NextCard != nil {
		nextMsg, err := sendGamePrompt(ctx, b, chatID, result.NextCard, result.NextToken, false)
		if err != nil {
			logger.Error("failed to send next game prompt", "user_id", userID, "error", err)
			return
		}
		game.DefaultManager.SetCurrentMessageIDForToken(chatID, userID, result.NextToken, nextMsg.ID)
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
	session := game.DefaultManager.GetSession(chatID, userID)
	if strings.HasPrefix(text, "/") {
		return session != nil
	}
	if session == nil {
		return false
	}

	result := game.DefaultManager.ResolveTextAttempt(chatID, userID, text)
	if !result.Handled {
		return true
	}

	revealSuffix := "❌"
	if result.Correct {
		revealSuffix = "✅"
	}
	revealText := formatGameRevealText(result.Card, revealSuffix)
	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: result.PromptMessageID,
		Text:      revealText,
		ParseMode: models.ParseModeMarkdown,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{},
		},
	})
	if err != nil {
		logger.Error("failed to edit game prompt", "user_id", userID, "error", err)
	}

	if result.StatsText != "" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   result.StatsText,
		})
		return true
	}

	if result.NextCard != nil {
		msg, err := sendGamePrompt(ctx, b, chatID, result.NextCard, result.NextToken, false)
		if err != nil {
			logger.Error("failed to send next game prompt", "user_id", userID, "error", err)
			return true
		}
		game.DefaultManager.SetCurrentMessageIDForToken(chatID, userID, result.NextToken, msg.ID)
	}

	return true
}

func sendGamePrompt(ctx context.Context, b *bot.Bot, chatID int64, card *game.Card, token string, includeHint bool) (*models.Message, error) {
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

func formatGameRevealText(card game.Card, suffix string) string {
	shown := bot.EscapeMarkdown(card.Shown)
	expected := bot.EscapeMarkdown(card.Expected)
	return fmt.Sprintf("%s — ||%s|| %s", shown, expected, suffix)
}
