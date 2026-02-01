package game

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const gameSessionTTL = 24 * time.Hour

type persistedCard struct {
	PairID    uint          `json:"pair_id"`
	Direction CardDirection `json:"direction"`
}

func LoadGameSessionState(chatID, userID int64, now time.Time) (*db.GameSessionState, error) {
	if db.DB == nil {
		return nil, nil
	}
	var session db.GameSessionState
	err := db.DB.
		Where("chat_id = ? AND user_id = ? AND expires_at > ?", chatID, userID, now).
		First(&session).Error
	if err == nil {
		return &session, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, err
}

func SessionPairIDs(row *db.GameSessionState) ([]uint, error) {
	if row == nil || len(row.PairIDs) == 0 {
		return nil, nil
	}
	var cards []persistedCard
	if err := json.Unmarshal(row.PairIDs, &cards); err != nil {
		return nil, err
	}
	seen := make(map[uint]struct{}, len(cards))
	ids := make([]uint, 0, len(cards))
	for _, card := range cards {
		if card.PairID == 0 {
			continue
		}
		if _, ok := seen[card.PairID]; ok {
			continue
		}
		seen[card.PairID] = struct{}{}
		ids = append(ids, card.PairID)
	}
	return ids, nil
}

func (m *GameManager) StartFromPersisted(row *db.GameSessionState, pairs []db.WordPair) (*GameSession, error) {
	if row == nil {
		return nil, errors.New("nil game session state")
	}
	var persisted []persistedCard
	if err := json.Unmarshal(row.PairIDs, &persisted); err != nil {
		return nil, err
	}
	if len(persisted) == 0 {
		return nil, errors.New("empty deck")
	}

	lookup := make(map[uint]db.WordPair, len(pairs))
	for _, pair := range pairs {
		if pair.ID != 0 {
			lookup[pair.ID] = pair
		}
	}
	deck := make([]Card, 0, len(persisted))
	for _, entry := range persisted {
		pair, ok := lookup[entry.PairID]
		if !ok {
			continue
		}
		deck = append(deck, buildCard(pair, entry.Direction))
	}
	if len(deck) == 0 {
		return nil, errors.New("no cards available")
	}
	if row.CurrentIndex < 0 || row.CurrentIndex >= len(deck) {
		return nil, errors.New("current index out of range")
	}

	session := &GameSession{
		chatID:           row.ChatID,
		userID:           row.UserID,
		sessionID:        row.SessionID,
		startedAt:        row.LastActivityAt,
		lastActivityAt:   row.LastActivityAt,
		correctCount:     row.ScoreCorrect,
		attemptCount:     row.ScoreAttempted,
		currentCard:      &deck[row.CurrentIndex],
		currentMessageID: row.CurrentMessageID,
		currentResolved:  false,
		currentToken:     row.CurrentToken,
		deck:             append([]Card(nil), deck[row.CurrentIndex+1:]...),
	}

	key := getSessionKey(row.ChatID, row.UserID)
	m.mu.Lock()
	m.sessions[key] = session
	m.mu.Unlock()
	return session, nil
}

func persistGameSessionState(session *GameSession) {
	if session == nil || db.DB == nil {
		return
	}
	state, err := buildGameSessionState(session)
	if err != nil {
		logger.Error("failed to build game session state", "user_id", session.userID, "error", err)
		return
	}
	if err := upsertGameSessionState(state); err != nil {
		logger.Error("failed to persist game session state", "user_id", session.userID, "error", err)
	}
}

func buildGameSessionState(session *GameSession) (*db.GameSessionState, error) {
	if session == nil {
		return nil, errors.New("nil session")
	}
	deck := buildPersistedDeck(session)
	raw, err := json.Marshal(deck)
	if err != nil {
		return nil, err
	}
	lastActivity := session.lastActivityAt
	if lastActivity.IsZero() {
		lastActivity = time.Now().UTC()
	}
	return &db.GameSessionState{
		ChatID:           session.chatID,
		UserID:           session.userID,
		SessionID:        session.sessionID,
		PairIDs:          datatypes.JSON(raw),
		CurrentIndex:     0,
		CurrentToken:     session.currentToken,
		CurrentMessageID: session.currentMessageID,
		ScoreCorrect:     session.correctCount,
		ScoreAttempted:   session.attemptCount,
		LastActivityAt:   lastActivity,
		ExpiresAt:        lastActivity.Add(gameSessionTTL),
	}, nil
}

func buildPersistedDeck(session *GameSession) []persistedCard {
	if session == nil {
		return nil
	}
	deck := make([]persistedCard, 0, len(session.deck)+1)
	if session.currentCard != nil {
		deck = append(deck, persistedCard{
			PairID:    session.currentCard.PairID,
			Direction: session.currentCard.Direction,
		})
	}
	for _, card := range session.deck {
		deck = append(deck, persistedCard{
			PairID:    card.PairID,
			Direction: card.Direction,
		})
	}
	return deck
}

func upsertGameSessionState(state *db.GameSessionState) error {
	if state == nil || db.DB == nil {
		return nil
	}
	if state.LastActivityAt.IsZero() {
		state.LastActivityAt = time.Now().UTC()
	}
	state.ExpiresAt = state.LastActivityAt.Add(gameSessionTTL)
	return db.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "chat_id"},
			{Name: "user_id"},
		},
		UpdateAll: true,
	}).Create(state).Error
}

func deleteGameSessionState(chatID, userID int64) {
	if db.DB == nil {
		return
	}
	if err := db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).
		Delete(&db.GameSessionState{}).Error; err != nil {
		logger.Error("failed to delete game session state", "user_id", userID, "error", err)
	}
}

func DeleteGameSessionState(chatID, userID int64) error {
	if db.DB == nil {
		return nil
	}
	return db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).
		Delete(&db.GameSessionState{}).Error
}
