package onboarding

import (
	"strings"
	"testing"
)

func TestRenderConfirmationPromptWithNoEligiblePairs(t *testing.T) {
	text, keyboard := RenderConfirmationPrompt("en", "ru", 0)

	if !strings.Contains(text, "Eligible cards: 0") {
		t.Fatalf("expected eligible cards count, got %q", text)
	}
	if !strings.Contains(text, "Sorry, we don't have a training set for the chosen languages") {
		t.Fatalf("expected no-training-set guidance, got %q", text)
	}

	if keyboard == nil {
		t.Fatalf("expected keyboard")
	}
	if len(keyboard.InlineKeyboard) != 1 {
		t.Fatalf("expected only Back row, got %d rows", len(keyboard.InlineKeyboard))
	}
	if len(keyboard.InlineKeyboard[0]) != 1 || keyboard.InlineKeyboard[0][0].Text != "Back" {
		t.Fatalf("expected only Back button, got %+v", keyboard.InlineKeyboard)
	}
}

func TestRenderConfirmationPromptWithEligiblePairs(t *testing.T) {
	text, keyboard := RenderConfirmationPrompt("en", "ru", 3)

	if strings.Contains(text, "Sorry, we don't have a training set") {
		t.Fatalf("did not expect no-training-set guidance when cards are available")
	}
	if keyboard == nil {
		t.Fatalf("expected keyboard")
	}
	if len(keyboard.InlineKeyboard) < 2 {
		t.Fatalf("expected Initialize and Back rows, got %d rows", len(keyboard.InlineKeyboard))
	}
	if keyboard.InlineKeyboard[0][0].Text != "Initialize" {
		t.Fatalf("expected Initialize button first, got %+v", keyboard.InlineKeyboard)
	}
}
