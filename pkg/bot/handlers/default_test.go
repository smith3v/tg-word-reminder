package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

func TestDefaultHandlerSendsHelpForText(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("hello", 100)

	DefaultHandler(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Commands:") {
		t.Fatalf("expected commands message, got %q", got)
	}
}

func TestDefaultHandlerConsumesResetPhraseFlow(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	if err := db.DB.Create(&db.WordPair{UserID: 200, Word1: "hola", Word2: "hello"}).Error; err != nil {
		t.Fatalf("failed to seed pair: %v", err)
	}
	if err := db.DB.Create(&db.OnboardingState{UserID: 200, AwaitingResetPhrase: true}).Error; err != nil {
		t.Fatalf("failed to seed onboarding state: %v", err)
	}

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("RESET MY DATA", 200)

	DefaultHandler(context.Background(), b, update)

	var count int64
	if err := db.DB.Model(&db.WordPair{}).Where("user_id = ?", 200).Count(&count).Error; err != nil {
		t.Fatalf("failed to count pairs: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected pairs to be wiped, got %d", count)
	}

	var state db.OnboardingState
	if err := db.DB.Where("user_id = ?", 200).First(&state).Error; err != nil {
		t.Fatalf("expected onboarding state to exist after restart: %v", err)
	}
	if state.Step != "choose_learning" || state.AwaitingResetPhrase {
		t.Fatalf("expected onboarding wizard step after reset, got %+v", state)
	}

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Choose the language you are learning") {
		t.Fatalf("expected onboarding prompt after reset, got %q", got)
	}
}
