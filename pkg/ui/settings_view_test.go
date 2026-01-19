package ui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestRenderHomeButtons(t *testing.T) {
	text, keyboard, err := RenderHome(2, true, false, true, -5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Settings") {
		t.Fatalf("expected settings text, got %q", text)
	}
	if !strings.Contains(text, "Cards per session") {
		t.Fatalf("expected cards per session label, got %q", text)
	}
	if !strings.Contains(text, "Timezone") {
		t.Fatalf("expected timezone label, got %q", text)
	}

	pairsData, _ := BuildPairsCallback()
	slotsData, _ := BuildSlotsCallback()
	tzData, _ := BuildTimezoneCallback()
	closeData, _ := BuildCloseCallback()

	assertButton(t, keyboard, "Cards", pairsData)
	assertButton(t, keyboard, "Reminders", slotsData)
	assertButton(t, keyboard, "Timezone", tzData)
	assertButton(t, keyboard, "Close", closeData)
}

func TestRenderPairsButtons(t *testing.T) {
	text, keyboard, err := RenderPairs(4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Cards per session") {
		t.Fatalf("expected cards per session text, got %q", text)
	}

	decData, _ := BuildPairsDecCallback()
	incData, _ := BuildPairsIncCallback()
	backData, _ := BuildHomeCallback()

	assertButton(t, keyboard, "-1", decData)
	assertButton(t, keyboard, "+1", incData)
	assertButton(t, keyboard, "Back", backData)

	for _, value := range []int{1, 2, 3, 5, 10} {
		setData, _ := BuildPairsSetCallback(value)
		assertButton(t, keyboard, strconv.Itoa(value), setData)
	}
}

func TestRenderSlotsButtons(t *testing.T) {
	text, keyboard, err := RenderSlots(true, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Reminder slots") {
		t.Fatalf("expected slots text, got %q", text)
	}

	morningData, _ := BuildSlotToggleCallback(SlotMorning)
	afternoonData, _ := BuildSlotToggleCallback(SlotAfternoon)
	eveningData, _ := BuildSlotToggleCallback(SlotEvening)
	backData, _ := BuildHomeCallback()

	assertButton(t, keyboard, "Morning ✅", morningData)
	assertButton(t, keyboard, "Afternoon ❌", afternoonData)
	assertButton(t, keyboard, "Evening ✅", eveningData)
	assertButton(t, keyboard, "Back", backData)
}

func TestRenderTimezoneButtons(t *testing.T) {
	text, keyboard, err := RenderTimezone(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Timezone") {
		t.Fatalf("expected timezone text, got %q", text)
	}

	decData, _ := BuildTimezoneDecCallback()
	incData, _ := BuildTimezoneIncCallback()
	backData, _ := BuildHomeCallback()

	assertButton(t, keyboard, "-1", decData)
	assertButton(t, keyboard, "+1", incData)
	assertButton(t, keyboard, "Back", backData)

	for _, value := range []int{-8, -5, 0, 1, 3, 8} {
		setData, _ := BuildTimezoneSetCallback(value)
		label := "UTC" + strconv.Itoa(value)
		if value > 0 {
			label = "UTC+" + strconv.Itoa(value)
		}
		if value == 0 {
			label = "UTC+0"
		}
		assertButton(t, keyboard, label, setData)
	}
}

func assertButton(t *testing.T, keyboard *models.InlineKeyboardMarkup, text, callbackData string) {
	t.Helper()

	if keyboard == nil {
		t.Fatalf("expected keyboard, got nil")
	}

	for _, row := range keyboard.InlineKeyboard {
		for _, button := range row {
			if button.Text == text {
				if button.CallbackData != callbackData {
					t.Fatalf("button %q callback mismatch: got %q want %q", text, button.CallbackData, callbackData)
				}
				return
			}
		}
	}
	t.Fatalf("button %q not found", text)
}
