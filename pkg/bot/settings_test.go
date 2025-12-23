package bot

import (
	"errors"
	"testing"

	"github.com/smith3v/tg-word-reminder/pkg/db"
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

func TestApplyActionFrequencySetPreset(t *testing.T) {
	settings := db.UserSettings{UserID: 1, PairsToSend: 2, RemindersPerDay: 3}

	next, screen, changed, err := ApplyAction(settings, ui.Action{Screen: ui.ScreenFrequency, Op: ui.OpSet, Value: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if screen != ui.ScreenFrequency {
		t.Fatalf("expected frequency screen, got %v", screen)
	}
	if !changed {
		t.Fatalf("expected settings to change")
	}
	if next.RemindersPerDay != 10 {
		t.Fatalf("expected reminders to be 10, got %d", next.RemindersPerDay)
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

func TestApplyActionFrequencyAboveMax(t *testing.T) {
	settings := db.UserSettings{UserID: 1, PairsToSend: 2, RemindersPerDay: MaxRemindersPerDay}

	_, screen, changed, err := ApplyAction(settings, ui.Action{Screen: ui.ScreenFrequency, Op: ui.OpInc})
	if !errors.Is(err, ErrAboveMax) {
		t.Fatalf("expected ErrAboveMax, got %v", err)
	}
	if screen != ui.ScreenFrequency {
		t.Fatalf("expected frequency screen, got %v", screen)
	}
	if changed {
		t.Fatalf("expected no change")
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
