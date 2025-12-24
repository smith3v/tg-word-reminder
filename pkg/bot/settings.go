package bot

import (
	"errors"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/ui"
)

const (
	MinPairsPerReminder = 0
	MaxPairsPerReminder = 10

	MinRemindersPerDay = 0
	MaxRemindersPerDay = 10
)

var (
	ErrBelowMin      = errors.New("value below minimum")
	ErrAboveMax      = errors.New("value above maximum")
	ErrInvalidAction = errors.New("invalid settings action")
)

func ApplyAction(settings db.UserSettings, action ui.Action) (db.UserSettings, ui.Screen, bool, error) {
	switch action.Screen {
	case ui.ScreenHome:
		if action.Op != ui.OpNone {
			return settings, ui.ScreenHome, false, ErrInvalidAction
		}
		return settings, ui.ScreenHome, false, nil
	case ui.ScreenClose:
		if action.Op != ui.OpNone {
			return settings, ui.ScreenClose, false, ErrInvalidAction
		}
		return settings, ui.ScreenClose, false, nil
	case ui.ScreenPairs:
		next, changed, err := applyValue(settings.PairsToSend, action, MinPairsPerReminder, MaxPairsPerReminder)
		if err != nil {
			return settings, ui.ScreenPairs, false, err
		}
		newSettings := settings
		newSettings.PairsToSend = next
		return newSettings, ui.ScreenPairs, changed, nil
	case ui.ScreenFrequency:
		next, changed, err := applyValue(settings.RemindersPerDay, action, MinRemindersPerDay, MaxRemindersPerDay)
		if err != nil {
			return settings, ui.ScreenFrequency, false, err
		}
		newSettings := settings
		newSettings.RemindersPerDay = next
		return newSettings, ui.ScreenFrequency, changed, nil
	default:
		return settings, ui.ScreenHome, false, ErrInvalidAction
	}
}

func applyValue(current int, action ui.Action, min, max int) (int, bool, error) {
	switch action.Op {
	case ui.OpNone:
		return current, false, nil
	case ui.OpInc:
		next := current + 1
		return clampValue(current, next, min, max)
	case ui.OpDec:
		next := current - 1
		return clampValue(current, next, min, max)
	case ui.OpSet:
		return clampValue(current, action.Value, min, max)
	default:
		return current, false, ErrInvalidAction
	}
}

func clampValue(current, next, min, max int) (int, bool, error) {
	if next < min {
		return current, false, ErrBelowMin
	}
	if next > max {
		return current, false, ErrAboveMax
	}
	if next == current {
		return current, false, nil
	}
	return next, true, nil
}

func boundsForScreen(screen ui.Screen) (int, int, bool) {
	switch screen {
	case ui.ScreenPairs:
		return MinPairsPerReminder, MaxPairsPerReminder, true
	case ui.ScreenFrequency:
		return MinRemindersPerDay, MaxRemindersPerDay, true
	default:
		return 0, 0, false
	}
}
