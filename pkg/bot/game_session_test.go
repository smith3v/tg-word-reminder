package bot

import (
	"fmt"
	"testing"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
)

func resetSessions() {
	sessionsMu.Lock()
	sessions = make(map[string]*GameSession)
	sessionsMu.Unlock()
}

func TestGetSessionKey(t *testing.T) {
	got := getSessionKey(10, 20)
	if got != "10:20" {
		t.Fatalf("expected key to be %q, got %q", "10:20", got)
	}
}

func TestStartNewSessionCreatesDeckAndStoresSession(t *testing.T) {
	resetSessions()

	pairs := []db.WordPair{
		{ID: 1, UserID: 10, Word1: "hola", Word2: "adios"},
		{ID: 2, UserID: 10, Word1: "uno", Word2: "dos"},
	}

	start := time.Now()
	session := startNewSession(100, 200, pairs)
	if session == nil {
		t.Fatalf("expected session to be created")
	}
	if session.chatID != 100 || session.userID != 200 {
		t.Fatalf("unexpected session identifiers: chat=%d user=%d", session.chatID, session.userID)
	}
	if session.startedAt.Before(start) || session.lastActivityAt.Before(start) {
		t.Fatalf("expected timestamps to be initialized")
	}
	if session.correctCount != 0 || session.attemptCount != 0 {
		t.Fatalf("expected counters to start at zero")
	}
	if session.currentCard != nil || !session.currentResolved {
		t.Fatalf("expected no current card and resolved state")
	}
	if len(session.deck) != len(pairs)*2 {
		t.Fatalf("expected deck size %d, got %d", len(pairs)*2, len(session.deck))
	}

	key := getSessionKey(100, 200)
	sessionsMu.Lock()
	stored := sessions[key]
	sessionsMu.Unlock()
	if stored != session {
		t.Fatalf("expected session stored in map")
	}

	pairByID := map[uint]db.WordPair{
		1: pairs[0],
		2: pairs[1],
	}
	counts := map[string]int{}
	for _, card := range session.deck {
		pair, ok := pairByID[card.PairID]
		if !ok {
			t.Fatalf("unexpected pair ID in deck: %d", card.PairID)
		}
		if card.Word1 != pair.Word1 || card.Word2 != pair.Word2 {
			t.Fatalf("card words do not match pair: %+v", card)
		}
		switch card.Direction {
		case DirectionAToB:
			if card.Shown != card.Word1 || card.Expected != card.Word2 {
				t.Fatalf("A_to_B card mismatch: %+v", card)
			}
		case DirectionBToA:
			if card.Shown != card.Word2 || card.Expected != card.Word1 {
				t.Fatalf("B_to_A card mismatch: %+v", card)
			}
		default:
			t.Fatalf("unexpected card direction: %s", card.Direction)
		}
		key := fmt.Sprintf("%d:%s", card.PairID, card.Direction)
		counts[key]++
	}

	for _, pair := range pairs {
		for _, direction := range []CardDirection{DirectionAToB, DirectionBToA} {
			key := fmt.Sprintf("%d:%s", pair.ID, direction)
			if counts[key] != 1 {
				t.Fatalf("expected one card for %s, got %d", key, counts[key])
			}
		}
	}
}

func TestPopNextCardDequeuesAndSetsCurrent(t *testing.T) {
	card1 := Card{
		PairID:    1,
		Direction: DirectionAToB,
		Word1:     "a",
		Word2:     "b",
		Shown:     "a",
		Expected:  "b",
	}
	card2 := Card{
		PairID:    2,
		Direction: DirectionBToA,
		Word1:     "c",
		Word2:     "d",
		Shown:     "d",
		Expected:  "c",
	}
	session := &GameSession{
		deck: []Card{card1, card2},
	}

	got, ok := popNextCard(session)
	if !ok {
		t.Fatalf("expected card to be dequeued")
	}
	if got != card1 {
		t.Fatalf("expected first card, got %+v", got)
	}
	if len(session.deck) != 1 || session.deck[0] != card2 {
		t.Fatalf("expected remaining deck to contain second card")
	}
	if session.currentCard == nil || *session.currentCard != card1 {
		t.Fatalf("expected current card to be set")
	}
	if session.currentResolved || session.currentMessageID != 0 {
		t.Fatalf("expected current card to be unresolved with zero message ID")
	}
}

func TestPopNextCardReturnsFalseOnEmptyDeck(t *testing.T) {
	session := &GameSession{}
	_, ok := popNextCard(session)
	if ok {
		t.Fatalf("expected empty deck to return ok=false")
	}
}

func TestRequeueCardAppendsToDeck(t *testing.T) {
	card := Card{
		PairID:    3,
		Direction: DirectionAToB,
		Word1:     "x",
		Word2:     "y",
		Shown:     "x",
		Expected:  "y",
	}
	session := &GameSession{
		deck: []Card{},
	}
	requeueCard(session, card)
	if len(session.deck) != 1 || session.deck[0] != card {
		t.Fatalf("expected card to be appended")
	}
}

func TestFinishSessionFormatsStats(t *testing.T) {
	message := finishSession(nil)
	expected := "Game over!\nYou got 0 correct answers.\nAccuracy: 0% (0/0)"
	if message != expected {
		t.Fatalf("unexpected message: %q", message)
	}

	session := &GameSession{
		correctCount: 2,
		attemptCount: 3,
	}
	message = finishSession(session)
	expected = "Game over!\nYou got 2 correct answers.\nAccuracy: 67% (2/3)"
	if message != expected {
		t.Fatalf("unexpected stats message: %q", message)
	}
}
