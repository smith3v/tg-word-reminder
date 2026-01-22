package game

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/go-telegram/bot"
)

var messageRand = rand.New(rand.NewSource(time.Now().UnixNano()))

// PrepareWordPairMessage formats a word pair message with a random order of the words, hiding one under a spoiler
func PrepareWordPairMessage(word1, word2 string) string {
	if messageRand.Intn(2) == 0 {
		return fmt.Sprintf("%s  ||_%s_||\n", bot.EscapeMarkdown(word1), bot.EscapeMarkdown(word2))
	}
	return fmt.Sprintf("_%s_  ||%s||\n", bot.EscapeMarkdown(word2), bot.EscapeMarkdown(word1))
}
