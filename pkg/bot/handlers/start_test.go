package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

func TestHandleStartCreatesSettings(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/start", 202)

	HandleStart(context.Background(), b, update)

	var settings db.UserSettings
	if err := db.DB.Where("user_id = ?", 202).First(&settings).Error; err != nil {
		t.Fatalf("failed to load settings: %v", err)
	}
	if settings.PairsToSend != 1 || settings.RemindersPerDay != 1 {
		t.Fatalf("expected default settings, got %+v", settings)
	}

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Welcome") {
		t.Fatalf("expected welcome message, got %q", got)
	}
}

func TestHandleStartKeepsExistingSettings(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	seed := db.UserSettings{UserID: 203, PairsToSend: 3, RemindersPerDay: 4}
	if err := db.DB.Create(&seed).Error; err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/start", 203)

	HandleStart(context.Background(), b, update)

	var settings db.UserSettings
	if err := db.DB.Where("user_id = ?", 203).First(&settings).Error; err != nil {
		t.Fatalf("failed to load settings: %v", err)
	}
	if settings.PairsToSend != 3 || settings.RemindersPerDay != 4 {
		t.Fatalf("expected settings to remain unchanged, got %+v", settings)
	}
}
