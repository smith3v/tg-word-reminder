package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

func TestHandleExportRejectsNonPrivateChat(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/export", 400)
	update.Message.Chat.Type = models.ChatTypeGroup

	HandleExport(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "only in private chat") {
		t.Fatalf("expected private chat warning, got %q", got)
	}
}

func TestHandleExportEmptyVocabulary(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/export", 401)
	update.Message.Chat.Type = models.ChatTypePrivate

	HandleExport(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "no vocabulary") {
		t.Fatalf("expected empty vocabulary message, got %q", got)
	}
}

func TestHandleExportSendsDocument(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	pairs := []db.WordPair{
		{UserID: 402, Word1: "hello", Word2: "world"},
		{UserID: 402, Word1: "uno", Word2: "one"},
	}
	if err := db.DB.Create(&pairs).Error; err != nil {
		t.Fatalf("failed to seed pairs: %v", err)
	}

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/export", 402)
	update.Message.Chat.Type = models.ChatTypePrivate

	HandleExport(context.Background(), b, update)

	caption, _ := client.lastMultipartField(t, "caption")
	if caption != "Your vocabulary export (2 pairs)." {
		t.Fatalf("unexpected caption: %q", caption)
	}
	_, filename := client.lastMultipartField(t, "document")
	if !strings.HasPrefix(filename, "vocabulary-") || !strings.HasSuffix(filename, ".csv") {
		t.Fatalf("unexpected filename: %q", filename)
	}
}
