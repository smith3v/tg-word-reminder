package ui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestRenderHomeButtons(t *testing.T) {
	text, keyboard, err := RenderHome(2, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Settings") {
		t.Fatalf("expected settings text, got %q", text)
	}

	pairsData, _ := BuildPairsCallback()
	freqData, _ := BuildFrequencyCallback()
	closeData, _ := BuildCloseCallback()

	assertButton(t, keyboard, "Pairs", pairsData)
	assertButton(t, keyboard, "Frequency", freqData)
	assertButton(t, keyboard, "Close", closeData)
}

func TestRenderPairsButtons(t *testing.T) {
	text, keyboard, err := RenderPairs(4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Pairs per reminder") {
		t.Fatalf("expected pairs text, got %q", text)
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

func TestRenderFreqButtons(t *testing.T) {
	text, keyboard, err := RenderFreq(6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Reminder frequency") {
		t.Fatalf("expected frequency text, got %q", text)
	}

	decData, _ := BuildFrequencyDecCallback()
	incData, _ := BuildFrequencyIncCallback()
	backData, _ := BuildHomeCallback()

	assertButton(t, keyboard, "-1", decData)
	assertButton(t, keyboard, "+1", incData)
	assertButton(t, keyboard, "Back", backData)

	for _, value := range []int{1, 2, 3, 5, 10} {
		setData, _ := BuildFrequencySetCallback(value)
		assertButton(t, keyboard, strconv.Itoa(value), setData)
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
