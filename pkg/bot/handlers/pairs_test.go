package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/bot/training"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

func TestHandleGetPairWithoutWords(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/getpair", 404)

	HandleGetPair(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Nothing to review") {
		t.Fatalf("expected no data message, got %q", got)
	}
}

func TestHandleGetPairSendsRandomPair(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	training.ResetDefaultManager(time.Now)

	if err := db.DB.Create(&db.WordPair{
		UserID:   505,
		Word1:    "Hola",
		Word2:    "Adios",
		SrsState: "new",
		SrsDueAt: time.Now().Add(-time.Minute),
	}).Error; err != nil {
		t.Fatalf("failed to seed word pair: %v", err)
	}

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/getpair", 505)

	HandleGetPair(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "_Adios_") || !strings.Contains(got, "||") {
		t.Fatalf("expected message to contain both words, got %q", got)
	}
}

func TestHandleGetPairResumesPersistedSession(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	training.ResetDefaultManager(time.Now)
	training.ResetOverdueManager(time.Now)

	now := time.Date(2025, 1, 5, 10, 0, 0, 0, time.UTC)
	pairs := []db.WordPair{
		{UserID: 707, Word1: "hola", Word2: "adios", SrsState: "new", SrsDueAt: now},
		{UserID: 707, Word1: "uno", Word2: "one", SrsState: "new", SrsDueAt: now},
	}
	if err := db.DB.Create(&pairs).Error; err != nil {
		t.Fatalf("failed to seed pairs: %v", err)
	}

	session := training.DefaultManager.StartOrRestart(707, 707, pairs)
	if session == nil || session.CurrentPair() == nil {
		t.Fatalf("expected in-memory session")
	}

	client := newMockClient()
	client.response = `{"ok":true,"result":{"message_id":77}}`
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/getpair", 707)

	HandleGetPair(context.Background(), b, update)

	sendCount := 0
	for _, req := range client.requests {
		if strings.Contains(req.path, "sendMessage") {
			sendCount++
		}
	}
	if sendCount != 1 {
		t.Fatalf("expected one sendMessage for resumed session, got %d", sendCount)
	}
}

func TestHandleClearRemovesWordPairs(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	userID := int64(606)
	pairs := []db.WordPair{
		{UserID: userID, Word1: "one", Word2: "uno"},
		{UserID: userID, Word1: "two", Word2: "dos"},
	}
	if err := db.DB.Create(&pairs).Error; err != nil {
		t.Fatalf("failed to seed word pairs: %v", err)
	}

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/clear", userID)

	HandleClear(context.Background(), b, update)

	var count int64
	if err := db.DB.Model(&db.WordPair{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count word pairs: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected word pairs to be deleted, found %d", count)
	}

	got := client.lastMessageText(t)
	if !strings.Contains(got, "has been cleared") {
		t.Fatalf("expected confirmation message, got %q", got)
	}
}
