package handlers

import (
	"context"
	"strings"
	"testing"

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
	if !strings.Contains(got, "You have no word pairs saved") {
		t.Fatalf("expected no data message, got %q", got)
	}
}

func TestHandleGetPairSendsRandomPair(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	if err := db.DB.Create(&db.WordPair{
		UserID: 505,
		Word1:  "Hola",
		Word2:  "Adios",
	}).Error; err != nil {
		t.Fatalf("failed to seed word pair: %v", err)
	}

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/getpair", 505)

	HandleGetPair(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Hola") || !strings.Contains(got, "Adios") {
		t.Fatalf("expected message to contain both words, got %q", got)
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
