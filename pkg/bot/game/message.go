package game

import (
	"fmt"
	"math/rand"

	"github.com/go-telegram/bot"
)

// PrepareWordPairMessage formats a word pair message with a random order of the words, hiding one under a spoiler
func PrepareWordPairMessage(word1, word2 string) string {
	if rand.Intn(2) == 0 {
		return fmt.Sprintf("%s  ||_%s_||\n", bot.EscapeMarkdown(word1), bot.EscapeMarkdown(word2))
	}
	return fmt.Sprintf("_%s_  ||%s||\n", bot.EscapeMarkdown(word2), bot.EscapeMarkdown(word1))
}
