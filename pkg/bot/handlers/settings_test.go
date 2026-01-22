package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
	"github.com/smith3v/tg-word-reminder/pkg/ui"
)

func TestApplyActionNavigation(t *testing.T) {
	settings := db.UserSettings{UserID: 1, PairsToSend: 2, RemindersPerDay: 3}

	next, screen, changed, err := ApplyAction(settings, ui.Action{Screen: ui.ScreenHome, Op: ui.OpNone})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if screen != ui.ScreenHome {
		t.Fatalf("expected home screen, got %v", screen)
	}
	if changed {
		t.Fatalf("expected no change")
	}
	if next != settings {
		t.Fatalf("settings should be unchanged")
	}
}

func TestApplyActionSlotsNavigation(t *testing.T) {
	settings := db.UserSettings{UserID: 1, ReminderMorning: true}

	next, screen, changed, err := ApplyAction(settings, ui.Action{Screen: ui.ScreenSlots, Op: ui.OpNone})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if screen != ui.ScreenSlots {
		t.Fatalf("expected slots screen, got %v", screen)
	}
	if changed {
		t.Fatalf("expected no change")
	}
	if next != settings {
		t.Fatalf("settings should be unchanged")
	}
}

func TestApplyActionPairsIncrement(t *testing.T) {
	settings := db.UserSettings{UserID: 1, PairsToSend: 2, RemindersPerDay: 3}

	next, screen, changed, err := ApplyAction(settings, ui.Action{Screen: ui.ScreenPairs, Op: ui.OpInc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if screen != ui.ScreenPairs {
		t.Fatalf("expected pairs screen, got %v", screen)
	}
	if !changed {
		t.Fatalf("expected settings to change")
	}
	if next.PairsToSend != 3 {
		t.Fatalf("expected pairs to be 3, got %d", next.PairsToSend)
	}
}

func TestApplyActionPairsSetPreset(t *testing.T) {
	settings := db.UserSettings{UserID: 1, PairsToSend: 2, RemindersPerDay: 3}

	next, screen, changed, err := ApplyAction(settings, ui.Action{Screen: ui.ScreenPairs, Op: ui.OpSet, Value: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if screen != ui.ScreenPairs {
		t.Fatalf("expected pairs screen, got %v", screen)
	}
	if !changed {
		t.Fatalf("expected settings to change")
	}
	if next.PairsToSend != 5 {
		t.Fatalf("expected pairs to be 5, got %d", next.PairsToSend)
	}
}

func TestApplyActionTimezoneSetPreset(t *testing.T) {
	settings := db.UserSettings{UserID: 1, PairsToSend: 2, TimezoneOffsetHours: 0}

	next, screen, changed, err := ApplyAction(settings, ui.Action{Screen: ui.ScreenTimezone, Op: ui.OpSet, Value: -5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if screen != ui.ScreenTimezone {
		t.Fatalf("expected timezone screen, got %v", screen)
	}
	if !changed {
		t.Fatalf("expected settings to change")
	}
	if next.TimezoneOffsetHours != -5 {
		t.Fatalf("expected timezone to be -5, got %d", next.TimezoneOffsetHours)
	}
}

func TestApplyActionPairsBelowMin(t *testing.T) {
	settings := db.UserSettings{UserID: 1, PairsToSend: MinPairsPerReminder, RemindersPerDay: 3}

	_, screen, changed, err := ApplyAction(settings, ui.Action{Screen: ui.ScreenPairs, Op: ui.OpDec})
	if !errors.Is(err, ErrBelowMin) {
		t.Fatalf("expected ErrBelowMin, got %v", err)
	}
	if screen != ui.ScreenPairs {
		t.Fatalf("expected pairs screen, got %v", screen)
	}
	if changed {
		t.Fatalf("expected no change")
	}
}

func TestApplyActionPairsSetAboveMax(t *testing.T) {
	settings := db.UserSettings{UserID: 1, PairsToSend: 2, RemindersPerDay: 3}

	_, screen, changed, err := ApplyAction(settings, ui.Action{Screen: ui.ScreenPairs, Op: ui.OpSet, Value: MaxPairsPerReminder + 1})
	if !errors.Is(err, ErrAboveMax) {
		t.Fatalf("expected ErrAboveMax, got %v", err)
	}
	if screen != ui.ScreenPairs {
		t.Fatalf("expected pairs screen, got %v", screen)
	}
	if changed {
		t.Fatalf("expected no change")
	}
}

func TestApplyActionTimezoneAboveMax(t *testing.T) {
	settings := db.UserSettings{UserID: 1, PairsToSend: 2, TimezoneOffsetHours: MaxTimezoneOffset}

	_, screen, changed, err := ApplyAction(settings, ui.Action{Screen: ui.ScreenTimezone, Op: ui.OpInc})
	if !errors.Is(err, ErrAboveMax) {
		t.Fatalf("expected ErrAboveMax, got %v", err)
	}
	if screen != ui.ScreenTimezone {
		t.Fatalf("expected timezone screen, got %v", screen)
	}
	if changed {
		t.Fatalf("expected no change")
	}
}

func TestApplyActionSlotsToggle(t *testing.T) {
	settings := db.UserSettings{UserID: 1, ReminderMorning: false}

	next, screen, changed, err := ApplyAction(settings, ui.Action{Screen: ui.ScreenSlots, Op: ui.OpToggle, Value: ui.SlotMorning})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if screen != ui.ScreenSlots {
		t.Fatalf("expected slots screen, got %v", screen)
	}
	if !changed {
		t.Fatalf("expected settings to change")
	}
	if !next.ReminderMorning {
		t.Fatalf("expected reminder morning to toggle on, got %+v", next)
	}
}

func TestApplyActionClose(t *testing.T) {
	settings := db.UserSettings{UserID: 1, PairsToSend: 2, RemindersPerDay: 3}

	_, screen, changed, err := ApplyAction(settings, ui.Action{Screen: ui.ScreenClose, Op: ui.OpNone})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if screen != ui.ScreenClose {
		t.Fatalf("expected close screen, got %v", screen)
	}
	if changed {
		t.Fatalf("expected no change")
	}
}

func TestApplyActionInvalid(t *testing.T) {
	settings := db.UserSettings{UserID: 1, PairsToSend: 2, RemindersPerDay: 3}

	_, _, _, err := ApplyAction(settings, ui.Action{Screen: ui.ScreenHome, Op: ui.OpInc})
	if !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("expected ErrInvalidAction, got %v", err)
	}
}

func TestHandleSettingsMissingSettings(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/settings", 300)

	HandleSettings(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Settings not found") {
		t.Fatalf("expected missing settings message, got %q", got)
	}
}

func TestHandleSettingsSendsHome(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	seed := db.UserSettings{UserID: 301, PairsToSend: 2, RemindersPerDay: 3, ReminderEvening: true, TimezoneOffsetHours: 0}
	if err := db.DB.Create(&seed).Error; err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/settings", 301)

	HandleSettings(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Settings") {
		t.Fatalf("expected settings text, got %q", got)
	}
	if !strings.Contains(got, "Cards per session") {
		t.Fatalf("expected cards label, got %q", got)
	}
	if !strings.Contains(got, "Timezone") {
		t.Fatalf("expected timezone label, got %q", got)
	}
}

func TestHandleSettingsCallbackUpdatesPairs(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	seed := db.UserSettings{UserID: 302, PairsToSend: 1, RemindersPerDay: 1}
	if err := db.DB.Create(&seed).Error; err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	data, err := ui.BuildPairsIncCallback()
	if err != nil {
		t.Fatalf("failed to build callback: %v", err)
	}

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestCallbackUpdate(data, 302, 302, 50)

	HandleSettingsCallback(context.Background(), b, update)

	var settings db.UserSettings
	if err := db.DB.Where("user_id = ?", 302).First(&settings).Error; err != nil {
		t.Fatalf("failed to load settings: %v", err)
	}
	if settings.PairsToSend != 2 {
		t.Fatalf("expected pairs to increment, got %d", settings.PairsToSend)
	}

	last := client.requests[len(client.requests)-1]
	if !strings.Contains(last.path, "editMessageText") {
		t.Fatalf("expected editMessageText request, got %s", last.path)
	}
}

func TestHandleSettingsCallbackUpdatesTimezone(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	seed := db.UserSettings{UserID: 303, TimezoneOffsetHours: 0}
	if err := db.DB.Create(&seed).Error; err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	data, err := ui.BuildTimezoneSetCallback(-5)
	if err != nil {
		t.Fatalf("failed to build callback: %v", err)
	}

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestCallbackUpdate(data, 303, 303, 50)

	HandleSettingsCallback(context.Background(), b, update)

	var settings db.UserSettings
	if err := db.DB.Where("user_id = ?", 303).First(&settings).Error; err != nil {
		t.Fatalf("failed to load settings: %v", err)
	}
	if settings.TimezoneOffsetHours != -5 {
		t.Fatalf("expected timezone to update, got %d", settings.TimezoneOffsetHours)
	}
}
