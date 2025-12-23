package ui

import (
	"fmt"

	"github.com/go-telegram/bot/models"
)

func RenderHome(pairsPerReminder, remindersPerDay int) (string, *models.InlineKeyboardMarkup, error) {
	pairsData, err := BuildPairsCallback()
	if err != nil {
		return "", nil, err
	}
	freqData, err := BuildFrequencyCallback()
	if err != nil {
		return "", nil, err
	}
	closeData, err := BuildCloseCallback()
	if err != nil {
		return "", nil, err
	}

	text := fmt.Sprintf(
		"Settings\n- Pairs per reminder: %d\n- Frequency per day: %d",
		pairsPerReminder,
		remindersPerDay,
	)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Pairs", CallbackData: pairsData},
				{Text: "Frequency", CallbackData: freqData},
			},
			{
				{Text: "Close", CallbackData: closeData},
			},
		},
	}

	return text, keyboard, nil
}

func RenderPairs(current int) (string, *models.InlineKeyboardMarkup, error) {
	decData, err := BuildPairsDecCallback()
	if err != nil {
		return "", nil, err
	}
	incData, err := BuildPairsIncCallback()
	if err != nil {
		return "", nil, err
	}
	backData, err := BuildHomeCallback()
	if err != nil {
		return "", nil, err
	}

	text := fmt.Sprintf("Pairs per reminder\nCurrent value: %d", current)
	keyboard, err := buildAdjustKeyboard(decData, incData, backData, BuildPairsSetCallback)
	if err != nil {
		return "", nil, err
	}

	return text, keyboard, nil
}

func RenderFreq(current int) (string, *models.InlineKeyboardMarkup, error) {
	decData, err := BuildFrequencyDecCallback()
	if err != nil {
		return "", nil, err
	}
	incData, err := BuildFrequencyIncCallback()
	if err != nil {
		return "", nil, err
	}
	backData, err := BuildHomeCallback()
	if err != nil {
		return "", nil, err
	}

	text := fmt.Sprintf("Reminder frequency\nCurrent value: %d", current)
	keyboard, err := buildAdjustKeyboard(decData, incData, backData, BuildFrequencySetCallback)
	if err != nil {
		return "", nil, err
	}

	return text, keyboard, nil
}

func buildAdjustKeyboard(decData, incData, backData string, setCallback func(int) (string, error)) (*models.InlineKeyboardMarkup, error) {
	preset1, err := setCallback(1)
	if err != nil {
		return nil, err
	}
	preset2, err := setCallback(2)
	if err != nil {
		return nil, err
	}
	preset3, err := setCallback(3)
	if err != nil {
		return nil, err
	}
	preset5, err := setCallback(5)
	if err != nil {
		return nil, err
	}
	preset10, err := setCallback(10)
	if err != nil {
		return nil, err
	}

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "-1", CallbackData: decData},
				{Text: "+1", CallbackData: incData},
			},
			{
				{Text: "1", CallbackData: preset1},
				{Text: "2", CallbackData: preset2},
				{Text: "3", CallbackData: preset3},
			},
			{
				{Text: "5", CallbackData: preset5},
				{Text: "10", CallbackData: preset10},
			},
			{
				{Text: "Back", CallbackData: backData},
			},
		},
	}

	return keyboard, nil
}
