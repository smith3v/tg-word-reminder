package bot

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

type cardDirection int

const (
	cardDirectionAToB cardDirection = iota
	cardDirectionBToA
)

type cardResult int

const (
	resultCorrect cardResult = iota
	resultIncorrect
	resultReveal
)

const (
	GameCallbackPrefix = "g:"
	revealCallback     = GameCallbackPrefix + "reveal"
)

var (
	gameManager = newGameManager(15 * time.Minute)

	multiSpacePattern   = regexp.MustCompile(`\s+`)
	trailingPunctuation = regexp.MustCompile(`[.!?,]+$`)
)

type Card struct {
	pairID    uint
	direction cardDirection
	shown     string
	expected  string
}

type GameSession struct {
	chatID int64
	userID int64

	startedAt      time.Time
	lastActivityAt time.Time

	deck             []Card
	currentCard      *Card
	currentResolved  bool
	currentMessageID int

	correctCount int
	attemptCount int
}

type GameManager struct {
	mu         sync.Mutex
	sessions   map[string]*GameSession
	timeout    time.Duration
	now        func() time.Time
	randSource func() *rand.Rand
}

func newGameManager(timeout time.Duration) *GameManager {
	return &GameManager{
		sessions: make(map[string]*GameSession),
		timeout:  timeout,
		now:      time.Now,
		randSource: func() *rand.Rand {
			return rand.New(rand.NewSource(time.Now().UnixNano()))
		},
	}
}

func sessionKey(chatID, userID int64) string {
	return fmt.Sprintf("%d:%d", chatID, userID)
}

func StartGameSweeper(ctx context.Context, b *bot.Bot) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			gameManager.sweepInactiveSessions(ctx, b)
		}
	}
}

func HandleGame(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil {
		logger.Error("invalid update in HandleGame")
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	if chatID == 0 || userID == 0 {
		logger.Error("chat or user missing in HandleGame")
		return
	}

	if err := gameManager.startSession(ctx, b, chatID, userID); err != nil {
		logger.Error("failed to start game session", "user_id", userID, "error", err)
	}
}

func HandleGameAnswer(ctx context.Context, b *bot.Bot, update *models.Update) bool {
	if update == nil || update.Message == nil || update.Message.From == nil {
		return false
	}
	if update.Message.Chat.ID == 0 || update.Message.From.ID == 0 {
		return false
	}
	if update.Message.Text == "" {
		return false
	}
	handled, err := gameManager.handleAnswer(ctx, b, update.Message.Chat.ID, update.Message.From.ID, update.Message.Text)
	if err != nil {
		logger.Error("failed to handle game answer", "user_id", update.Message.From.ID, "error", err)
	}
	return handled
}

func HandleGameReveal(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.CallbackQuery == nil {
		logger.Error("invalid update in HandleGameReveal")
		return
	}

	if update.CallbackQuery.Data != revealCallback {
		return
	}

	if update.CallbackQuery.Message.Type != models.MaybeInaccessibleMessageTypeMessage || update.CallbackQuery.Message.Message == nil {
		logger.Error("callback query message is inaccessible", "user_id", update.CallbackQuery.From.ID)
		return
	}

	msg := update.CallbackQuery.Message.Message
	handled, err := gameManager.handleReveal(ctx, b, msg.Chat.ID, update.CallbackQuery.From.ID, msg.ID)
	if err != nil {
		logger.Error("failed to handle reveal callback", "user_id", update.CallbackQuery.From.ID, "error", err)
	}

	if _, ansErr := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: update.CallbackQuery.ID}); ansErr != nil {
		logger.Error("failed to answer reveal callback", "error", ansErr)
	}

	if !handled {
		if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "No active game session. Send /game to start a new one.",
		}); err != nil {
			logger.Error("failed to notify about missing session", "error", err)
		}
	}
}

func (gm *GameManager) startSession(ctx context.Context, b *bot.Bot, chatID, userID int64) error {
	var wordPairs []db.WordPair
	if err := db.DB.Where("user_id = ?", userID).Order("RANDOM()").Limit(5).Find(&wordPairs).Error; err != nil {
		return fmt.Errorf("failed to load word pairs: %w", err)
	}

	if len(wordPairs) == 0 {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "You have no word pairs saved. Upload some first to play the game.",
		})
		return err
	}

	randGen := gm.randSource()
	cards := buildDeck(wordPairs, randGen)
	if len(cards) == 0 {
		return errors.New("empty deck generated")
	}

	key := sessionKey(chatID, userID)

	gm.mu.Lock()
	_, restarting := gm.sessions[key]
	now := gm.now()
	session := &GameSession{
		chatID:         chatID,
		userID:         userID,
		startedAt:      now,
		lastActivityAt: now,
		deck:           cards,
	}

	firstCard := session.deck[0]
	session.deck = session.deck[1:]
	session.currentCard = &firstCard

	gm.sessions[key] = session
	gm.mu.Unlock()

	if restarting {
		if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Starting a new game!",
		}); err != nil {
			logger.Error("failed to send restart notification", "user_id", userID, "error", err)
		}
	}

	return gm.sendPrompt(ctx, b, session)
}

func (gm *GameManager) handleAnswer(ctx context.Context, b *bot.Bot, chatID, userID int64, answer string) (bool, error) {
	key := sessionKey(chatID, userID)
	gm.mu.Lock()
	session := gm.sessions[key]
	if session == nil || session.currentCard == nil {
		gm.mu.Unlock()
		return false, nil
	}
	if session.currentResolved {
		gm.mu.Unlock()
		return true, nil
	}

	card := *session.currentCard
	messageID := session.currentMessageID
	gm.mu.Unlock()

	result := resultIncorrect
	if normalizeAnswer(answer) == normalizeAnswer(card.expected) {
		result = resultCorrect
	}

	if err := gm.resolveCurrent(ctx, b, key, session, card, messageID, result); err != nil {
		return true, err
	}

	return true, nil
}

func (gm *GameManager) handleReveal(ctx context.Context, b *bot.Bot, chatID, userID int64, messageID int) (bool, error) {
	key := sessionKey(chatID, userID)
	gm.mu.Lock()
	session := gm.sessions[key]
	if session == nil || session.currentCard == nil {
		gm.mu.Unlock()
		return false, nil
	}
	if session.currentResolved || session.currentMessageID != messageID {
		gm.mu.Unlock()
		return true, nil
	}

	card := *session.currentCard
	gm.mu.Unlock()

	if err := gm.resolveCurrent(ctx, b, key, session, card, messageID, resultReveal); err != nil {
		return true, err
	}
	return true, nil
}

func (gm *GameManager) resolveCurrent(ctx context.Context, b *bot.Bot, key string, session *GameSession, card Card, messageID int, result cardResult) error {
	switch result {
	case resultCorrect:
		if err := gm.editPrompt(ctx, b, session.chatID, messageID, card, "✅"); err != nil {
			logger.Error("failed to edit prompt after correct answer", "error", err)
		}
	case resultIncorrect:
		if err := gm.editPrompt(ctx, b, session.chatID, messageID, card, "❌"); err != nil {
			logger.Error("failed to edit prompt after incorrect answer", "error", err)
		}
	case resultReveal:
		if err := gm.editPrompt(ctx, b, session.chatID, messageID, card, "👀"); err != nil {
			logger.Error("failed to edit prompt after reveal", "error", err)
		}
	}

	nextCard, finished, stats := gm.advanceSession(key, result)

	if finished {
		return gm.sendStats(ctx, b, session.chatID, stats)
	}

	session.currentCard = nextCard
	return gm.sendPrompt(ctx, b, session)
}

func (gm *GameManager) advanceSession(key string, result cardResult) (*Card, bool, GameStats) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	session := gm.sessions[key]
	if session == nil || session.currentCard == nil {
		return nil, true, GameStats{}
	}

	session.currentResolved = true
	session.attemptCount++
	session.lastActivityAt = gm.now()

	current := *session.currentCard

	if result == resultCorrect {
		session.correctCount++
	} else {
		session.deck = append(session.deck, current)
	}

	if len(session.deck) == 0 {
		delete(gm.sessions, key)
		return nil, true, GameStats{Correct: session.correctCount, Attempts: session.attemptCount}
	}

	next := session.deck[0]
	session.deck = session.deck[1:]
	session.currentCard = &next
	session.currentResolved = false

	return &next, false, GameStats{Correct: session.correctCount, Attempts: session.attemptCount}
}

func (gm *GameManager) sendPrompt(ctx context.Context, b *bot.Bot, session *GameSession) error {
	if session.currentCard == nil {
		return errors.New("no card to prompt")
	}

	prompt := formatPrompt(*session.currentCard)
	keyboard := &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "👀", CallbackData: revealCallback},
		},
	}}

	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      session.chatID,
		Text:        prompt,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		return err
	}

	gm.mu.Lock()
	session.currentMessageID = msg.ID
	session.currentResolved = false
	gm.mu.Unlock()

	return nil
}

func (gm *GameManager) editPrompt(ctx context.Context, b *bot.Bot, chatID int64, messageID int, card Card, marker string) error {
	if messageID == 0 {
		return errors.New("message id is missing")
	}

	resolved := fmt.Sprintf("*%s — %s %s*", bot.EscapeMarkdown(card.shown), bot.EscapeMarkdown(card.expected), marker)
	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        resolved,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}},
	})
	return err
}

func (gm *GameManager) sendStats(ctx context.Context, b *bot.Bot, chatID int64, stats GameStats) error {
	accuracy := accuracyPercent(stats.Correct, stats.Attempts)
	text := fmt.Sprintf("*Game over!*\nYou got *%d* correct answers.\nAccuracy: *%d%%* (%d/%d)", stats.Correct, accuracy, stats.Correct, stats.Attempts)
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeMarkdown,
	})
	return err
}

func (gm *GameManager) sweepInactiveSessions(ctx context.Context, b *bot.Bot) {
	now := gm.now()

	gm.mu.Lock()
	expiredKeys := make([]string, 0)
	statsByKey := make(map[string]GameStats)
	chatByKey := make(map[string]int64)

	for key, session := range gm.sessions {
		if now.Sub(session.lastActivityAt) > gm.timeout {
			statsByKey[key] = GameStats{Correct: session.correctCount, Attempts: session.attemptCount}
			chatByKey[key] = session.chatID
			expiredKeys = append(expiredKeys, key)
			delete(gm.sessions, key)
		}
	}
	gm.mu.Unlock()

	for _, key := range expiredKeys {
		if err := gm.sendStats(ctx, b, chatByKey[key], statsByKey[key]); err != nil {
			logger.Error("failed to send timeout stats", "key", key, "error", err)
		}
	}
}

func buildDeck(pairs []db.WordPair, rng *rand.Rand) []Card {
	cards := make([]Card, 0, len(pairs)*2)
	for _, pair := range pairs {
		cards = append(cards, Card{
			pairID:    pair.ID,
			direction: cardDirectionAToB,
			shown:     pair.Word1,
			expected:  pair.Word2,
		})
		cards = append(cards, Card{
			pairID:    pair.ID,
			direction: cardDirectionBToA,
			shown:     pair.Word2,
			expected:  pair.Word1,
		})
	}

	rng.Shuffle(len(cards), func(i, j int) {
		cards[i], cards[j] = cards[j], cards[i]
	})

	return cards
}

func formatPrompt(card Card) string {
	return fmt.Sprintf("Translate: *%s* → ?\n_(reply with the missing word, or tap 👀 to reveal — counts as a miss)_", bot.EscapeMarkdown(card.shown))
}

func normalizeAnswer(input string) string {
	trimmed := strings.TrimSpace(input)
	lowered := strings.ToLower(trimmed)
	collapsed := multiSpacePattern.ReplaceAllString(lowered, " ")
	stripped := trailingPunctuation.ReplaceAllString(collapsed, "")
	return stripped
}

type GameStats struct {
	Correct  int
	Attempts int
}

func accuracyPercent(correct, attempts int) int {
	if attempts == 0 {
		return 0
	}
	return int(0.5 + 100*float64(correct)/float64(attempts))
}

func sessionTimedOut(lastActivity time.Time, now time.Time, timeout time.Duration) bool {
	return now.Sub(lastActivity) > timeout
}
