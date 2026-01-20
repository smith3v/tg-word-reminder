package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/bot/training"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

func HandleReview(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in HandleReview")
		return
	}
	if update.Message.Chat.Type != models.ChatTypePrivate {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "The /review command works only in private chat.",
		})
		return
	}

	now := time.Now().UTC()
	pairs, err := training.SelectSessionPairs(update.Message.From.ID, 10, now)
	if err != nil {
		logger.Error("failed to load review pairs", "user_id", update.Message.From.ID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to start review. Please try again later.",
		})
		return
	}
	if len(pairs) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Nothing to review right now.",
		})
		return
	}

	session := training.DefaultManager.StartOrRestart(update.Message.Chat.ID, update.Message.From.ID, pairs)
	card := session.CurrentPair()
	if card == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Nothing to review right now.",
		})
		return
	}

	prompt := training.BuildPrompt(*card)
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        prompt,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: training.BuildKeyboard(session.CurrentToken()),
	})
	if err != nil {
		logger.Error("failed to send review prompt", "user_id", update.Message.From.ID, "error", err)
		return
	}
	training.DefaultManager.SetCurrentMessageID(session, msg.ID)
	training.DefaultManager.SetCurrentPromptText(session, prompt)
	training.DefaultManager.Touch(update.Message.Chat.ID, update.Message.From.ID)
}

func HandleReviewCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.CallbackQuery == nil {
		logger.Error("invalid update in HandleReviewCallback")
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
			logger.Error("failed to answer review callback query", "error", err)
		}
	}

	token, grade, ok := parseReviewCallback(update.CallbackQuery.Data)
	if !ok {
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

	snapshot, ok := training.DefaultManager.Snapshot(msg.Chat.ID, update.CallbackQuery.From.ID)
	if !ok || snapshot.Token != token || snapshot.MessageID != msg.ID {
		answerCallback("Not active")
		return
	}

	pair := snapshot.Pair
	now := time.Now().UTC()
	training.ApplyGrade(&pair, grade, now)
	if err := db.DB.Save(&pair).Error; err != nil {
		logger.Error("failed to save review grade", "user_id", update.CallbackQuery.From.ID, "error", err)
		answerCallback("Failed to save review")
		return
	}

	updatedText := formatReviewResolvedText(snapshot.PromptText, grade)
	if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    msg.Chat.ID,
		MessageID: msg.ID,
		Text:      updatedText,
		ParseMode: models.ParseModeMarkdown,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{},
		},
	}); err != nil {
		logger.Error("failed to edit review prompt", "user_id", update.CallbackQuery.From.ID, "error", err)
	}

	answerCallback("")
	training.DefaultManager.Touch(msg.Chat.ID, update.CallbackQuery.From.ID)

	nextPair, nextToken := training.DefaultManager.Advance(msg.Chat.ID, update.CallbackQuery.From.ID)
	if nextPair == nil {
		return
	}

	prompt := training.BuildPrompt(*nextPair)
	nextMsg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      msg.Chat.ID,
		Text:        prompt,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: training.BuildKeyboard(nextToken),
	})
	if err != nil {
		logger.Error("failed to send next review prompt", "user_id", update.CallbackQuery.From.ID, "error", err)
		return
	}

	if session := training.DefaultManager.GetSession(msg.Chat.ID, update.CallbackQuery.From.ID); session != nil {
		training.DefaultManager.SetCurrentMessageID(session, nextMsg.ID)
		training.DefaultManager.SetCurrentPromptText(session, prompt)
	}
}

func parseReviewCallback(data string) (string, training.Grade, bool) {
	if !strings.HasPrefix(data, training.ReviewCallbackPrefix) {
		return "", "", false
	}
	parts := strings.Split(data, ":")
	if len(parts) != 4 || parts[0] != "t" || parts[1] != "grade" {
		return "", "", false
	}
	token := parts[2]
	switch parts[3] {
	case string(training.GradeAgain):
		return token, training.GradeAgain, true
	case string(training.GradeHard):
		return token, training.GradeHard, true
	case string(training.GradeGood):
		return token, training.GradeGood, true
	case string(training.GradeEasy):
		return token, training.GradeEasy, true
	default:
		return "", "", false
	}
}

func formatReviewResolvedText(prompt string, grade training.Grade) string {
	label := strings.Title(string(grade))
	if prompt == "" {
		return label
	}
	return fmt.Sprintf("%s\n%s", prompt, label)
}
