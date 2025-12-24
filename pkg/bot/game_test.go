package bot

import (
	"math/rand"
	"testing"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
)

func TestBuildDeckCreatesTwoCardsPerPair(t *testing.T) {
	pairs := []db.WordPair{
		{ID: 1, Word1: "one", Word2: "uno"},
		{ID: 2, Word1: "two", Word2: "dos"},
		{ID: 3, Word1: "three", Word2: "tres"},
	}
	rng := rand.New(rand.NewSource(1))

	deck := buildDeck(pairs, rng)

	if len(deck) != len(pairs)*2 {
		t.Fatalf("expected %d cards, got %d", len(pairs)*2, len(deck))
	}

	counts := make(map[uint]int)
	directions := make(map[cardDirection]int)
	for _, card := range deck {
		counts[card.pairID]++
		directions[card.direction]++
	}

	for _, pair := range pairs {
		if counts[pair.ID] != 2 {
			t.Fatalf("expected pair %d to appear twice, got %d", pair.ID, counts[pair.ID])
		}
	}
	if directions[cardDirectionAToB] != len(pairs) || directions[cardDirectionBToA] != len(pairs) {
		t.Fatalf("expected equal directions, got %+v", directions)
	}
}

func TestCorrectAnswerRemovesCard(t *testing.T) {
	gm := newGameManager(time.Minute)
	gm.now = func() time.Time { return time.Unix(0, 0) }

	current := Card{shown: "hola", expected: "hello"}
	deck := []Card{{shown: "adios", expected: "bye"}}

	session := &GameSession{
		chatID:         1,
		userID:         2,
		currentCard:    &current,
		deck:           deck,
		lastActivityAt: gm.now(),
	}
	key := sessionKey(session.chatID, session.userID)
	gm.sessions[key] = session

	next, finished, stats := gm.advanceSession(key, resultCorrect)

	if finished {
		t.Fatalf("expected session to continue after correct answer with remaining deck")
	}
	if next == nil || next.shown != "adios" {
		t.Fatalf("expected next card to be drawn from deck, got %#v", next)
	}
	if len(session.deck) != 0 {
		t.Fatalf("expected deck to shrink, got %d", len(session.deck))
	}
	if stats.Correct != 1 || stats.Attempts != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestIncorrectAnswerRequeuesCard(t *testing.T) {
	gm := newGameManager(time.Minute)
	gm.now = func() time.Time { return time.Unix(0, 0) }

	current := Card{shown: "hola", expected: "hello"}
	deck := []Card{{shown: "adios", expected: "bye"}}

	session := &GameSession{chatID: 1, userID: 2, currentCard: &current, deck: deck}
	key := sessionKey(session.chatID, session.userID)
	gm.sessions[key] = session

	next, finished, stats := gm.advanceSession(key, resultIncorrect)

	if finished {
		t.Fatalf("expected session to continue after incorrect answer")
	}
	if next == nil || next.shown != "adios" {
		t.Fatalf("expected next card from front of deck, got %#v", next)
	}
	if len(session.deck) != 1 {
		t.Fatalf("expected deck to contain requeued card, got %d", len(session.deck))
	}
	if session.deck[0].shown != "hola" {
		t.Fatalf("expected current card to be requeued at back, got %s", session.deck[0].shown)
	}
	if stats.Correct != 0 || stats.Attempts != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestRevealRequeuesCard(t *testing.T) {
	gm := newGameManager(time.Minute)
	gm.now = func() time.Time { return time.Unix(0, 0) }

	current := Card{shown: "hola", expected: "hello"}
	deck := []Card{{shown: "adios", expected: "bye"}}

	session := &GameSession{chatID: 1, userID: 2, currentCard: &current, deck: deck}
	key := sessionKey(session.chatID, session.userID)
	gm.sessions[key] = session

	next, finished, stats := gm.advanceSession(key, resultReveal)

	if finished {
		t.Fatalf("expected session to continue after reveal")
	}
	if next == nil || next.shown != "adios" {
		t.Fatalf("expected next card from front of deck, got %#v", next)
	}
	if len(session.deck) != 1 || session.deck[0].shown != "hola" {
		t.Fatalf("expected revealed card requeued to back, got %#v", session.deck)
	}
	if stats.Correct != 0 || stats.Attempts != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestNormalizeAnswer(t *testing.T) {
	input := "  Hola!!!  "
	got := normalizeAnswer(input)
	if got != "hola" {
		t.Fatalf("expected normalized value 'hola', got %q", got)
	}

	spaced := "hola    amigo"
	if normalizeAnswer(spaced) != "hola amigo" {
		t.Fatalf("expected collapsed spaces, got %q", normalizeAnswer(spaced))
	}
}

func TestAccuracyPercent(t *testing.T) {
	cases := []struct {
		correct  int
		attempts int
		expected int
	}{
		{0, 0, 0},
		{3, 5, 60},
		{2, 3, 67},
	}

	for _, tt := range cases {
		if got := accuracyPercent(tt.correct, tt.attempts); got != tt.expected {
			t.Fatalf("accuracyPercent(%d, %d) = %d, want %d", tt.correct, tt.attempts, got, tt.expected)
		}
	}
}

func TestSessionTimedOut(t *testing.T) {
	now := time.Now()
	timeout := 15 * time.Minute

	if sessionTimedOut(now.Add(-14*time.Minute), now, timeout) {
		t.Fatalf("expected session to be active")
	}
	if !sessionTimedOut(now.Add(-16*time.Minute), now, timeout) {
		t.Fatalf("expected session to be timed out")
	}
}
