package bot

import (
	"fmt"
	"testing"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
)

type testClock struct {
	t time.Time
}

func (c *testClock) Now() time.Time {
	return c.t
}

func (c *testClock) Advance(d time.Duration) {
	c.t = c.t.Add(d)
}

func TestGetSessionKey(t *testing.T) {
	got := getSessionKey(10, 20)
	if got != "10:20" {
		t.Fatalf("expected key to be %q, got %q", "10:20", got)
	}
}

func TestStartOrRestartCreatesDeckAndSetsCurrent(t *testing.T) {
	clock := &testClock{t: time.Date(2024, 2, 1, 8, 0, 0, 0, time.UTC)}
	manager := NewGameManager(clock.Now)

	pairs := []db.WordPair{
		{ID: 1, UserID: 10, Word1: "hola", Word2: "adios"},
		{ID: 2, UserID: 10, Word1: "uno", Word2: "dos"},
	}

	session := manager.StartOrRestart(100, 200, pairs)
	if session == nil {
		t.Fatalf("expected session to be created")
	}
	if session.chatID != 100 || session.userID != 200 {
		t.Fatalf("unexpected session identifiers: chat=%d user=%d", session.chatID, session.userID)
	}
	if !session.startedAt.Equal(clock.t) || !session.lastActivityAt.Equal(clock.t) {
		t.Fatalf("expected timestamps to be initialized from clock")
	}
	if session.correctCount != 0 || session.attemptCount != 0 {
		t.Fatalf("expected counters to start at zero")
	}
	if session.currentCard == nil || session.currentResolved {
		t.Fatalf("expected a current card and unresolved state")
	}
	if session.currentMessageID != 0 || session.currentToken == "" {
		t.Fatalf("expected prompt metadata to be initialized")
	}
	expectedTotal := len(pairs) * 2
	if len(session.deck) != expectedTotal-1 {
		t.Fatalf("expected deck size %d, got %d", expectedTotal-1, len(session.deck))
	}

	key := getSessionKey(100, 200)
	manager.mu.Lock()
	stored := manager.sessions[key]
	manager.mu.Unlock()
	if stored != session {
		t.Fatalf("expected session stored in map")
	}

	expectedCounts := make(map[string]int)
	for _, card := range buildDeck(pairs) {
		cardKey := fmt.Sprintf("%d:%s:%s:%s", card.PairID, card.Direction, card.Shown, card.Expected)
		expectedCounts[cardKey]++
	}
	seenCounts := make(map[string]int)
	for _, card := range session.deck {
		cardKey := fmt.Sprintf("%d:%s:%s:%s", card.PairID, card.Direction, card.Shown, card.Expected)
		seenCounts[cardKey]++
	}
	current := *session.currentCard
	currentKey := fmt.Sprintf("%d:%s:%s:%s", current.PairID, current.Direction, current.Shown, current.Expected)
	seenCounts[currentKey]++
	if len(expectedCounts) != len(seenCounts) {
		t.Fatalf("expected card set size to match after start")
	}
	for key, count := range expectedCounts {
		if seenCounts[key] != count {
			t.Fatalf("card %s count mismatch", key)
		}
	}
}

func TestNextPromptDequeuesAndSetsCurrent(t *testing.T) {
	manager := NewGameManager(time.Now)
	card1 := Card{
		PairID:    1,
		Direction: DirectionAToB,
		Shown:     "a",
		Expected:  "b",
	}
	card2 := Card{
		PairID:    2,
		Direction: DirectionBToA,
		Shown:     "d",
		Expected:  "c",
	}
	session := &GameSession{
		deck:            []Card{card1, card2},
		currentResolved: true,
	}

	got, ok := manager.NextPrompt(session)
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
	if session.currentResolved || session.currentMessageID != 0 || session.currentToken == "" {
		t.Fatalf("expected prompt to be unresolved with zero message ID")
	}
}

func TestResolveCorrectUpdatesCounts(t *testing.T) {
	clock := &testClock{t: time.Date(2024, 2, 1, 8, 0, 0, 0, time.UTC)}
	manager := NewGameManager(clock.Now)
	card := Card{
		PairID:    1,
		Direction: DirectionAToB,
		Shown:     "a",
		Expected:  "b",
	}
	session := &GameSession{
		currentCard:     &card,
		currentResolved: false,
	}

	clock.Advance(2 * time.Minute)
	manager.ResolveCorrect(session)

	if session.attemptCount != 1 || session.correctCount != 1 {
		t.Fatalf("expected counts to increment, got attempts=%d correct=%d", session.attemptCount, session.correctCount)
	}
	if !session.currentResolved {
		t.Fatalf("expected current card to be marked resolved")
	}
	if !session.lastActivityAt.Equal(clock.t) {
		t.Fatalf("expected lastActivityAt to update")
	}
}

func TestResolveMissRequeueUpdatesCountsAndDeck(t *testing.T) {
	clock := &testClock{t: time.Date(2024, 2, 1, 8, 0, 0, 0, time.UTC)}
	manager := NewGameManager(clock.Now)
	card := Card{
		PairID:    3,
		Direction: DirectionAToB,
		Shown:     "x",
		Expected:  "y",
	}
	session := &GameSession{
		deck:            []Card{},
		currentCard:     &card,
		currentResolved: false,
	}

	clock.Advance(5 * time.Minute)
	manager.ResolveMissRequeue(session)

	if session.attemptCount != 1 || session.correctCount != 0 {
		t.Fatalf("expected miss counts to update, got attempts=%d correct=%d", session.attemptCount, session.correctCount)
	}
	if !session.currentResolved {
		t.Fatalf("expected current card to be marked resolved")
	}
	if len(session.deck) != 1 || session.deck[0] != card {
		t.Fatalf("expected card to be requeued")
	}
	if !session.lastActivityAt.Equal(clock.t) {
		t.Fatalf("expected lastActivityAt to update")
	}
}

func TestFinishStatsFormatsStats(t *testing.T) {
	manager := NewGameManager(time.Now)

	message := manager.FinishStats(nil)
	expected := "Game over!\nYou got 0 correct answers.\nAccuracy: 0% (0/0)"
	if message != expected {
		t.Fatalf("unexpected message: %q", message)
	}

	session := &GameSession{
		correctCount: 2,
		attemptCount: 3,
	}
	message = manager.FinishStats(session)
	expected = "Game over!\nYou got 2 correct answers.\nAccuracy: 67% (2/3)"
	if message != expected {
		t.Fatalf("unexpected stats message: %q", message)
	}
}

func TestSelectRandomPairsReturnsDistinctPairs(t *testing.T) {
	setupTestDB(t)

	userID := int64(800)
	otherID := int64(801)
	var pairs []db.WordPair
	for i := 0; i < 8; i++ {
		pairs = append(pairs, db.WordPair{
			UserID: userID,
			Word1:  fmt.Sprintf("word-%d", i),
			Word2:  fmt.Sprintf("term-%d", i),
		})
	}
	for i := 0; i < 3; i++ {
		pairs = append(pairs, db.WordPair{
			UserID: otherID,
			Word1:  fmt.Sprintf("other-%d", i),
			Word2:  fmt.Sprintf("else-%d", i),
		})
	}
	if err := db.DB.Create(&pairs).Error; err != nil {
		t.Fatalf("failed to seed pairs: %v", err)
	}

	selected, err := selectRandomPairs(userID, DeckPairs)
	if err != nil {
		t.Fatalf("failed to select pairs: %v", err)
	}
	if len(selected) != DeckPairs {
		t.Fatalf("expected %d pairs, got %d", DeckPairs, len(selected))
	}
	seen := make(map[uint]struct{})
	for _, pair := range selected {
		if pair.UserID != userID {
			t.Fatalf("expected user_id %d, got %d", userID, pair.UserID)
		}
		if _, exists := seen[pair.ID]; exists {
			t.Fatalf("expected distinct pairs, got duplicate ID %d", pair.ID)
		}
		seen[pair.ID] = struct{}{}
	}
}

func TestBuildDeckCreatesTwoCardsPerPair(t *testing.T) {
	pairs := []db.WordPair{
		{ID: 1, Word1: "uno", Word2: "one"},
		{ID: 2, Word1: "dos", Word2: "two"},
	}
	deck := buildDeck(pairs)
	if len(deck) != len(pairs)*2 {
		t.Fatalf("expected %d cards, got %d", len(pairs)*2, len(deck))
	}
	counts := make(map[string]int)
	for _, card := range deck {
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

func TestShuffleRetainsAllCards(t *testing.T) {
	pairs := []db.WordPair{
		{ID: 1, Word1: "uno", Word2: "one"},
		{ID: 2, Word1: "dos", Word2: "two"},
		{ID: 3, Word1: "tres", Word2: "three"},
	}
	deck := buildDeck(pairs)
	before := make(map[string]int)
	for _, card := range deck {
		key := fmt.Sprintf("%d:%s:%s:%s", card.PairID, card.Direction, card.Shown, card.Expected)
		before[key]++
	}

	shuffleDeck(deck)

	after := make(map[string]int)
	for _, card := range deck {
		key := fmt.Sprintf("%d:%s:%s:%s", card.PairID, card.Direction, card.Shown, card.Expected)
		after[key]++
	}
	if len(before) != len(after) {
		t.Fatalf("expected card set size to match after shuffle")
	}
	for key, count := range before {
		if after[key] != count {
			t.Fatalf("card %s count changed after shuffle", key)
		}
	}
}

func TestBuildCardSetsShownAndExpectedByDirection(t *testing.T) {
	pair := db.WordPair{ID: 7, Word1: "hola", Word2: "adios"}

	card := buildCard(pair, DirectionAToB)
	if card.PairID != pair.ID || card.Direction != DirectionAToB {
		t.Fatalf("unexpected card metadata: %+v", card)
	}
	if card.Shown != pair.Word1 || card.Expected != pair.Word2 {
		t.Fatalf("expected A_to_B mapping, got %+v", card)
	}

	card = buildCard(pair, DirectionBToA)
	if card.PairID != pair.ID || card.Direction != DirectionBToA {
		t.Fatalf("unexpected card metadata: %+v", card)
	}
	if card.Shown != pair.Word2 || card.Expected != pair.Word1 {
		t.Fatalf("expected B_to_A mapping, got %+v", card)
	}
}
