package bot

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

const (
	DeckPairs         = 5
	InactivityTimeout = 15 * time.Minute
	SweeperInterval   = 1 * time.Minute
)

// CardDirection describes which side of the pair is shown.
type CardDirection string

const (
	DirectionAToB CardDirection = "A_to_B"
	DirectionBToA CardDirection = "B_to_A"
)

// Card is a single prompt in the game deck.
type Card struct {
	PairID    uint
	Direction CardDirection
	Shown     string
	Expected  string
}

// GameSession tracks a single user's active game state.
type GameSession struct {
	chatID int64
	userID int64

	startedAt      time.Time
	lastActivityAt time.Time
	correctCount   int
	attemptCount   int

	deck []Card

	currentCard      *Card
	currentMessageID int
	currentResolved  bool
	currentToken     string
}

// GameManager manages active game sessions with thread-safe access.
type GameManager struct {
	mu       sync.Mutex
	sessions map[string]*GameSession
	now      func() time.Time
}

// MessageSender abstracts message delivery for timeout sweeps.
type MessageSender interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

// NewGameManager initializes a manager with an injectable clock.
func NewGameManager(now func() time.Time) *GameManager {
	if now == nil {
		now = time.Now
	}
	return &GameManager{
		sessions: make(map[string]*GameSession),
		now:      now,
	}
}

var gameManager = NewGameManager(nil)

// StartGameSweeper starts the inactivity sweeper for game sessions.
func StartGameSweeper(ctx context.Context, sender MessageSender) {
	gameManager.StartSweeper(ctx, sender)
}

// getSessionKey builds the map key for a user's active game session.
func getSessionKey(chatID, userID int64) string {
	return fmt.Sprintf("%d:%d", chatID, userID)
}

// StartOrRestart initializes or replaces a session and sets the first prompt.
func (m *GameManager) StartOrRestart(chatID, userID int64, vocabPairs []db.WordPair) *GameSession {
	now := m.now()
	pairs := samplePairs(vocabPairs, DeckPairs)
	deck := buildDeck(pairs)
	shuffleDeck(deck)

	session := &GameSession{
		chatID:          chatID,
		userID:          userID,
		startedAt:       now,
		lastActivityAt:  now,
		deck:            deck,
		currentResolved: true,
	}

	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	m.sessions[key] = session
	if len(deck) > 0 {
		m.nextPromptLocked(session)
	}
	m.mu.Unlock()

	return session
}

// StartSweeper periodically expires inactive sessions until ctx is canceled.
func (m *GameManager) StartSweeper(ctx context.Context, sender MessageSender) {
	if sender == nil {
		return
	}
	ticker := time.NewTicker(SweeperInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.SweepInactive(ctx, sender)
		}
	}
}

// SweepInactive expires idle sessions and sends stats without holding the lock.
func (m *GameManager) SweepInactive(ctx context.Context, sender MessageSender) {
	if sender == nil {
		return
	}
	expired := m.collectInactive(m.now())
	for _, session := range expired {
		if err := sender.SendMessage(ctx, session.chatID, session.statsText); err != nil {
			logger.Error("failed to send game timeout stats", "chat_id", session.chatID, "error", err)
		}
	}
}

// selectRandomPairs loads up to limit random pairs for the user, matching /getpair's source.
func selectRandomPairs(userID int64, limit int) ([]db.WordPair, error) {
	var pairs []db.WordPair
	query := db.DB.Where("user_id = ?", userID).Order("RANDOM()")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&pairs).Error; err != nil {
		return nil, err
	}
	return pairs, nil
}

// buildDeck expands pairs into two-direction cards without shuffling.
func buildDeck(pairs []db.WordPair) []Card {
	deck := make([]Card, 0, len(pairs)*2)
	for _, pair := range pairs {
		deck = append(deck, buildCard(pair, DirectionAToB))
		deck = append(deck, buildCard(pair, DirectionBToA))
	}
	return deck
}

// samplePairs returns up to limit distinct pairs, choosing randomly when necessary.
func samplePairs(pairs []db.WordPair, limit int) []db.WordPair {
	if limit <= 0 || len(pairs) <= limit {
		return pairs
	}
	perm := rand.Perm(len(pairs))
	selected := make([]db.WordPair, 0, limit)
	for i := 0; i < limit; i++ {
		selected = append(selected, pairs[perm[i]])
	}
	return selected
}

// shuffleDeck randomizes card order in place.
func shuffleDeck(deck []Card) {
	rand.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})
}

// NextPrompt dequeues the next card and updates current prompt fields.
func (m *GameManager) NextPrompt(session *GameSession) (Card, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nextPromptLocked(session)
}

// ResolveCorrect marks the current prompt as correct and discards it.
func (m *GameManager) ResolveCorrect(session *GameSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if session == nil || session.currentCard == nil || session.currentResolved {
		return
	}
	session.attemptCount++
	session.correctCount++
	session.currentResolved = true
	session.lastActivityAt = m.now()
}

// ResolveMissRequeue marks the prompt as missed and requeues it.
func (m *GameManager) ResolveMissRequeue(session *GameSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if session == nil || session.currentCard == nil || session.currentResolved {
		return
	}
	card := *session.currentCard
	session.attemptCount++
	session.currentResolved = true
	session.lastActivityAt = m.now()
	session.deck = append(session.deck, card)
}

// FinishStats builds the stats payload for a completed game session.
func (m *GameManager) FinishStats(session *GameSession) string {
	return formatStats(session)
}

// SetCurrentMessageID stores the Telegram message ID for the current prompt.
func (m *GameManager) SetCurrentMessageID(session *GameSession, messageID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if session == nil {
		return
	}
	session.currentMessageID = messageID
}

// SetCurrentMessageIDForToken stores the message ID if the token matches the active prompt.
func (m *GameManager) SetCurrentMessageIDForToken(chatID, userID int64, token string, messageID int) {
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	defer m.mu.Unlock()
	session := m.sessions[key]
	if session == nil {
		return
	}
	if token != "" && session.currentToken != token {
		return
	}
	session.currentMessageID = messageID
}

// GetSession returns the current session for a user, if any.
func (m *GameManager) GetSession(chatID, userID int64) *GameSession {
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[key]
}

type attemptResult struct {
	handled         bool
	correct         bool
	promptMessageID int
	card            Card
	nextCard        *Card
	nextToken       string
	statsText       string
}

type revealResult struct {
	handled         bool
	notice          string
	promptMessageID int
	card            Card
	nextCard        *Card
	nextToken       string
	statsText       string
}

// ResolveTextAttempt applies a typed answer and advances the session if possible.
func (m *GameManager) ResolveTextAttempt(chatID, userID int64, userText string) attemptResult {
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	defer m.mu.Unlock()

	session := m.sessions[key]
	if session == nil || session.currentCard == nil || session.currentResolved || session.currentMessageID == 0 {
		return attemptResult{}
	}

	result := attemptResult{
		handled:         true,
		promptMessageID: session.currentMessageID,
		card:            *session.currentCard,
	}

	result.correct = matchesExpected(userText, session.currentCard.Expected, strings.Contains(session.currentCard.Shown, ","))

	session.attemptCount++
	if result.correct {
		session.correctCount++
	}
	session.lastActivityAt = m.now()
	session.currentResolved = true

	if !result.correct {
		session.deck = append(session.deck, *session.currentCard)
	}

	if len(session.deck) == 0 {
		result.statsText = formatStats(session)
		delete(m.sessions, key)
		return result
	}

	card := session.deck[0]
	session.deck = session.deck[1:]
	session.currentCard = &card
	session.currentResolved = false
	session.currentMessageID = 0
	session.currentToken = m.nextTokenLocked()
	result.nextCard = &card
	result.nextToken = session.currentToken

	return result
}

// ResolveRevealAttempt validates and applies a reveal callback.
func (m *GameManager) ResolveRevealAttempt(chatID, userID int64, token string, messageID int) revealResult {
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	defer m.mu.Unlock()

	session := m.sessions[key]
	if session == nil {
		return revealResult{notice: "Not active"}
	}
	if session.currentCard == nil || session.currentResolved {
		return revealResult{notice: "Already resolved"}
	}
	if session.currentToken != token || session.currentMessageID != messageID {
		return revealResult{notice: "Not active"}
	}

	result := revealResult{
		handled:         true,
		promptMessageID: session.currentMessageID,
		card:            *session.currentCard,
	}

	session.attemptCount++
	session.lastActivityAt = m.now()
	session.currentResolved = true
	session.deck = append(session.deck, *session.currentCard)

	card := session.deck[0]
	session.deck = session.deck[1:]
	session.currentCard = &card
	session.currentResolved = false
	session.currentMessageID = 0
	session.currentToken = m.nextTokenLocked()
	result.nextCard = &card
	result.nextToken = session.currentToken

	return result
}

type expiredSession struct {
	chatID    int64
	statsText string
}

func (m *GameManager) nextPromptLocked(session *GameSession) (Card, bool) {
	if session == nil || len(session.deck) == 0 {
		return Card{}, false
	}
	card := session.deck[0]
	session.deck = session.deck[1:]
	session.currentCard = &card
	session.currentResolved = false
	session.currentMessageID = 0
	session.currentToken = m.nextTokenLocked()
	return card, true
}

func (m *GameManager) nextTokenLocked() string {
	return strconv.FormatInt(rand.Int63(), 36)
}

func (m *GameManager) collectInactive(now time.Time) []expiredSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	expired := make([]expiredSession, 0)
	for key, session := range m.sessions {
		if session == nil {
			delete(m.sessions, key)
			continue
		}
		if now.Sub(session.lastActivityAt) > InactivityTimeout {
			expired = append(expired, expiredSession{
				chatID:    session.chatID,
				statsText: formatStats(session),
			})
			delete(m.sessions, key)
		}
	}
	return expired
}

func formatStats(session *GameSession) string {
	if session == nil {
		return "Game over!\nYou got 0 correct answers.\nAccuracy: 0% (0/0)"
	}
	accuracy := 0
	if session.attemptCount > 0 {
		accuracy = int(math.Round(float64(session.correctCount) * 100 / float64(session.attemptCount)))
	}
	return fmt.Sprintf(
		"Game over!\nYou got %d correct answers.\nAccuracy: %d%% (%d/%d)",
		session.correctCount,
		accuracy,
		session.correctCount,
		session.attemptCount,
	)
}

func buildCard(pair db.WordPair, direction CardDirection) Card {
	card := Card{
		PairID:    pair.ID,
		Direction: direction,
	}
	switch direction {
	case DirectionAToB:
		card.Shown = pair.Word1
		card.Expected = pair.Word2
	case DirectionBToA:
		card.Shown = pair.Word2
		card.Expected = pair.Word1
	}
	return card
}

// normalizeAnswer normalizes text for strict comparison.
func normalizeAnswer(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ToLower(trimmed)
	trimmed = strings.Join(strings.Fields(trimmed), " ")
	trimmed = strings.TrimRightFunc(trimmed, func(r rune) bool {
		switch r {
		case '.', ',', '!', '?':
			return true
		default:
			return false
		}
	})
	return strings.TrimSpace(trimmed)
}

func matchesExpected(userText, expected string, promptHasComma bool) bool {
	if strings.Contains(expected, ",") {
		if promptHasComma && strings.Contains(userText, ",") {
			return matchesCommaList(userText, expected)
		}
		return matchesAnyCommaToken(userText, expected)
	}
	return normalizeAnswer(userText) == normalizeAnswer(expected)
}

func matchesAnyCommaToken(userText, expected string) bool {
	normalizedUser := normalizeAnswer(userText)
	if normalizedUser == "" {
		return false
	}
	tokens, ok := splitCommaTokens(expected)
	if !ok {
		return false
	}
	for _, token := range tokens {
		if normalizedUser == token {
			return true
		}
	}
	return false
}

func matchesCommaList(userText, expected string) bool {
	userTokens, ok := splitCommaTokens(userText)
	if !ok {
		return false
	}
	expectedTokens, ok := splitCommaTokens(expected)
	if !ok {
		return false
	}
	if len(userTokens) != len(expectedTokens) || len(userTokens) == 0 {
		return false
	}
	expectedCounts := make(map[string]int, len(expectedTokens))
	for _, token := range expectedTokens {
		expectedCounts[token]++
	}
	for _, token := range userTokens {
		expectedCounts[token]--
		if expectedCounts[token] < 0 {
			return false
		}
	}
	for _, count := range expectedCounts {
		if count != 0 {
			return false
		}
	}
	return true
}

func splitCommaTokens(input string) ([]string, bool) {
	parts := strings.Split(input, ",")
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		token := normalizeAnswer(part)
		if token == "" {
			return nil, false
		}
		tokens = append(tokens, token)
	}
	return tokens, true
}
