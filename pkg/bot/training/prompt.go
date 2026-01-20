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
	shown := pair.Word1
	expected := pair.Word2
	if rand.Intn(2) == 0 {
		shown = pair.Word2
		expected = pair.Word1
	}
	return fmt.Sprintf("%s → ||%s||", bot.EscapeMarkdown(shown), bot.EscapeMarkdown(expected))
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
