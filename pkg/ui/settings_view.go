package ui

import (
	"fmt"
	"strings"

	"github.com/go-telegram/bot/models"
)

func RenderHome(pairsPerReminder int, morning, afternoon, evening bool, timezoneOffset int) (string, *models.InlineKeyboardMarkup, error) {
	pairsData, err := BuildPairsCallback()
	if err != nil {
		return "", nil, err
	}
	slotsData, err := BuildSlotsCallback()
	if err != nil {
		return "", nil, err
	}
	tzData, err := BuildTimezoneCallback()
	if err != nil {
		return "", nil, err
	}
	closeData, err := BuildCloseCallback()
	if err != nil {
		return "", nil, err
	}

	slotsLabel := formatSlotSummary(morning, afternoon, evening)
	text := fmt.Sprintf(
		"Settings\n- Cards per session: %d\n- Reminders: %s\n- Timezone: UTC%+d",
		pairsPerReminder,
		slotsLabel,
		timezoneOffset,
	)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Cards", CallbackData: pairsData},
				{Text: "Reminders", CallbackData: slotsData},
			},
			{
				{Text: "Timezone", CallbackData: tzData},
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

	text := fmt.Sprintf("Cards per session\nCurrent value: %d", current)
	keyboard, err := buildAdjustKeyboard(decData, incData, backData, BuildPairsSetCallback)
	if err != nil {
		return "", nil, err
	}

	return text, keyboard, nil
}

func RenderSlots(morning, afternoon, evening bool) (string, *models.InlineKeyboardMarkup, error) {
	morningData, err := BuildSlotToggleCallback(SlotMorning)
	if err != nil {
		return "", nil, err
	}
	afternoonData, err := BuildSlotToggleCallback(SlotAfternoon)
	if err != nil {
		return "", nil, err
	}
	eveningData, err := BuildSlotToggleCallback(SlotEvening)
	if err != nil {
		return "", nil, err
	}
	backData, err := BuildHomeCallback()
	if err != nil {
		return "", nil, err
	}

	text := fmt.Sprintf(
		"Reminder slots\nMorning: %s\nAfternoon: %s\nEvening: %s",
		formatToggle(morning),
		formatToggle(afternoon),
		formatToggle(evening),
	)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: toggleLabel("Morning", morning), CallbackData: morningData},
				{Text: toggleLabel("Afternoon", afternoon), CallbackData: afternoonData},
			},
			{
				{Text: toggleLabel("Evening", evening), CallbackData: eveningData},
			},
			{
				{Text: "Back", CallbackData: backData},
			},
		},
	}

	return text, keyboard, nil
}

func RenderTimezone(current int) (string, *models.InlineKeyboardMarkup, error) {
	decData, err := BuildTimezoneDecCallback()
	if err != nil {
		return "", nil, err
	}
	incData, err := BuildTimezoneIncCallback()
	if err != nil {
		return "", nil, err
	}
	backData, err := BuildHomeCallback()
	if err != nil {
		return "", nil, err
	}

	text := fmt.Sprintf("Timezone\nCurrent value: UTC%+d", current)

	presetMinus8, err := BuildTimezoneSetCallback(-8)
	if err != nil {
		return "", nil, err
	}
	presetMinus5, err := BuildTimezoneSetCallback(-5)
	if err != nil {
		return "", nil, err
	}
	presetZero, err := BuildTimezoneSetCallback(0)
	if err != nil {
		return "", nil, err
	}
	presetPlus1, err := BuildTimezoneSetCallback(1)
	if err != nil {
		return "", nil, err
	}
	presetPlus3, err := BuildTimezoneSetCallback(3)
	if err != nil {
		return "", nil, err
	}
	presetPlus8, err := BuildTimezoneSetCallback(8)
	if err != nil {
		return "", nil, err
	}

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "-1", CallbackData: decData},
				{Text: "+1", CallbackData: incData},
			},
			{
				{Text: "UTC-8", CallbackData: presetMinus8},
				{Text: "UTC-5", CallbackData: presetMinus5},
				{Text: "UTC+0", CallbackData: presetZero},
			},
			{
				{Text: "UTC+1", CallbackData: presetPlus1},
				{Text: "UTC+3", CallbackData: presetPlus3},
				{Text: "UTC+8", CallbackData: presetPlus8},
			},
			{
				{Text: "Back", CallbackData: backData},
			},
		},
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

func formatSlotSummary(morning, afternoon, evening bool) string {
	parts := []string{}
	if morning {
		parts = append(parts, "Morning")
	}
	if afternoon {
		parts = append(parts, "Afternoon")
	}
	if evening {
		parts = append(parts, "Evening")
	}
	if len(parts) == 0 {
		return "off"
	}
	return strings.Join(parts, ", ")
}

func formatToggle(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func toggleLabel(label string, enabled bool) string {
	if enabled {
		return fmt.Sprintf("%s ✅", label)
	}
	return fmt.Sprintf("%s ❌", label)
}
