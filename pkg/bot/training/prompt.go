package training

import (
	"fmt"
	"math/rand"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/db"
)

const ReviewCallbackPrefix = "t:grade:"

func BuildPrompt(pair db.WordPair) string {
	shown := formatPromptSide(pair.Word1, false)
	expected := formatPromptSide(pair.Word2, true)
	if rand.Intn(2) == 0 {
		shown = formatPromptSide(pair.Word2, true)
		expected = formatPromptSide(pair.Word1, false)
	}
	return fmt.Sprintf("%s → ||%s||", shown, expected)
}

func BuildKeyboard(token string) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Again", CallbackData: fmt.Sprintf("%s%s:again", ReviewCallbackPrefix, token)},
				{Text: "Hard", CallbackData: fmt.Sprintf("%s%s:hard", ReviewCallbackPrefix, token)},
				{Text: "Good", CallbackData: fmt.Sprintf("%s%s:good", ReviewCallbackPrefix, token)},
				{Text: "Easy", CallbackData: fmt.Sprintf("%s%s:easy", ReviewCallbackPrefix, token)},
			},
		},
	}
}

func formatPromptSide(text string, italic bool) string {
	escaped := bot.EscapeMarkdown(text)
	if !italic {
		return escaped
	}
	return fmt.Sprintf("_%s_", escaped)
}
