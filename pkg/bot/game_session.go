package bot

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
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
	Word1     string
	Word2     string
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
}

var (
	sessionsMu sync.Mutex
	sessions   = make(map[string]*GameSession)
)

// getSessionKey builds the map key for a user's active game session.
func getSessionKey(chatID, userID int64) string {
	return fmt.Sprintf("%d:%d", chatID, userID)
}

// startNewSession initializes a session with a shuffled deck derived from pairs.
func startNewSession(chatID, userID int64, pairs []db.WordPair) *GameSession {
	now := time.Now()
	deck := make([]Card, 0, len(pairs)*2)
	for _, pair := range pairs {
		deck = append(deck, buildCard(pair, DirectionAToB))
		deck = append(deck, buildCard(pair, DirectionBToA))
	}
	rand.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	session := &GameSession{
		chatID:          chatID,
		userID:          userID,
		startedAt:       now,
		lastActivityAt:  now,
		deck:            deck,
		currentResolved: true,
	}

	key := getSessionKey(chatID, userID)
	sessionsMu.Lock()
	sessions[key] = session
	sessionsMu.Unlock()

	return session
}

// popNextCard dequeues the next card and marks it as the current prompt.
func popNextCard(session *GameSession) (Card, bool) {
	if session == nil || len(session.deck) == 0 {
		return Card{}, false
	}
	card := session.deck[0]
	session.deck = session.deck[1:]
	session.currentCard = &card
	session.currentResolved = false
	session.currentMessageID = 0
	return card, true
}

// requeueCard appends a card to the end of the session deck.
func requeueCard(session *GameSession, card Card) {
	if session == nil {
		return
	}
	session.deck = append(session.deck, card)
}

// finishSession builds the stats payload for a completed game session.
func finishSession(session *GameSession) string {
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
		Word1:     pair.Word1,
		Word2:     pair.Word2,
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
