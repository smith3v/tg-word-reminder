package ui

import (
	"errors"
	"strconv"
	"strings"
)

const (
	CallbackPrefix     = "s:"
	MaxCallbackDataLen = 64
)

type Screen string

const (
	ScreenHome      Screen = "home"
	ScreenPairs     Screen = "pairs"
	ScreenFrequency Screen = "freq"
	ScreenSlots     Screen = "slots"
	ScreenTimezone  Screen = "tz"
	ScreenClose     Screen = "close"
)

type Operation string

const (
	OpNone   Operation = ""
	OpInc    Operation = "+1"
	OpDec    Operation = "-1"
	OpSet    Operation = "set"
	OpToggle Operation = "toggle"
)

type Action struct {
	Screen Screen
	Op     Operation
	Value  int
}

var (
	errInvalidPrefix       = errors.New("invalid callback prefix")
	errInvalidAction       = errors.New("invalid callback action")
	errInvalidOperation    = errors.New("invalid callback operation")
	errInvalidValue        = errors.New("invalid callback value")
	errCallbackDataTooLong = errors.New("callback data too long")
)

func BuildHomeCallback() (string, error) {
	return buildSimpleCallback(ScreenHome)
}

func BuildPairsCallback() (string, error) {
	return buildSimpleCallback(ScreenPairs)
}

func BuildSlotsCallback() (string, error) {
	return buildSimpleCallback(ScreenSlots)
}

func BuildTimezoneCallback() (string, error) {
	return buildSimpleCallback(ScreenTimezone)
}

func BuildCloseCallback() (string, error) {
	return buildSimpleCallback(ScreenClose)
}

func BuildPairsIncCallback() (string, error) {
	return buildAdjustCallback(ScreenPairs, OpInc)
}

func BuildPairsDecCallback() (string, error) {
	return buildAdjustCallback(ScreenPairs, OpDec)
}

func BuildPairsSetCallback(value int) (string, error) {
	return buildSetCallback(ScreenPairs, value)
}

const (
	SlotMorning   = 1
	SlotAfternoon = 2
	SlotEvening   = 3
)

func BuildSlotToggleCallback(slot int) (string, error) {
	return buildToggleCallback(ScreenSlots, slot)
}

func BuildTimezoneIncCallback() (string, error) {
	return buildAdjustCallback(ScreenTimezone, OpInc)
}

func BuildTimezoneDecCallback() (string, error) {
	return buildAdjustCallback(ScreenTimezone, OpDec)
}

func BuildTimezoneSetCallback(value int) (string, error) {
	return buildSetCallbackSigned(ScreenTimezone, value)
}

func ParseCallbackData(data string) (Action, error) {
	if data == "" {
		return Action{}, errInvalidAction
	}
	if len(data) > MaxCallbackDataLen {
		return Action{}, errCallbackDataTooLong
	}
	if !strings.HasPrefix(data, CallbackPrefix) {
		return Action{}, errInvalidPrefix
	}

	parts := strings.Split(data, ":")
	if len(parts) < 2 || parts[0] != "s" {
		return Action{}, errInvalidPrefix
	}

	switch len(parts) {
	case 2:
		return parseSimpleAction(parts[1])
	case 3:
		return parseAdjustAction(parts[1], parts[2])
	case 4:
		screen, err := parseScreen(parts[1])
		if err != nil {
			return Action{}, err
		}
		switch Operation(parts[2]) {
		case OpSet:
			if screen == ScreenTimezone {
				return parseSetActionSigned(screen, parts[3])
			}
			return parseSetAction(screen, parts[3])
		case OpToggle:
			return parseToggleAction(screen, parts[3])
		default:
			return Action{}, errInvalidOperation
		}
	default:
		return Action{}, errInvalidAction
	}
}

func buildSimpleCallback(screen Screen) (string, error) {
	data := CallbackPrefix + string(screen)
	return validateCallbackData(data)
}

func buildAdjustCallback(screen Screen, op Operation) (string, error) {
	if screen != ScreenPairs && screen != ScreenFrequency && screen != ScreenTimezone {
		return "", errInvalidAction
	}
	if op != OpInc && op != OpDec {
		return "", errInvalidOperation
	}
	data := CallbackPrefix + string(screen) + ":" + string(op)
	return validateCallbackData(data)
}

func buildSetCallback(screen Screen, value int) (string, error) {
	if screen != ScreenPairs && screen != ScreenFrequency {
		return "", errInvalidAction
	}
	if value < 0 {
		return "", errInvalidValue
	}
	data := CallbackPrefix + string(screen) + ":" + string(OpSet) + ":" + strconv.Itoa(value)
	return validateCallbackData(data)
}

func buildSetCallbackSigned(screen Screen, value int) (string, error) {
	if screen != ScreenTimezone {
		return "", errInvalidAction
	}
	data := CallbackPrefix + string(screen) + ":" + string(OpSet) + ":" + strconv.Itoa(value)
	return validateCallbackData(data)
}

func buildToggleCallback(screen Screen, slot int) (string, error) {
	if screen != ScreenSlots {
		return "", errInvalidAction
	}
	if slot != SlotMorning && slot != SlotAfternoon && slot != SlotEvening {
		return "", errInvalidValue
	}
	data := CallbackPrefix + string(screen) + ":" + string(OpToggle) + ":" + strconv.Itoa(slot)
	return validateCallbackData(data)
}

func validateCallbackData(data string) (string, error) {
	if data == "" {
		return "", errInvalidAction
	}
	if len(data) > MaxCallbackDataLen {
		return "", errCallbackDataTooLong
	}
	return data, nil
}

func parseSimpleAction(screenPart string) (Action, error) {
	screen, err := parseScreen(screenPart)
	if err != nil {
		return Action{}, err
	}
	return Action{Screen: screen, Op: OpNone, Value: 0}, nil
}

func parseAdjustAction(screenPart, opPart string) (Action, error) {
	screen, err := parseScreen(screenPart)
	if err != nil {
		return Action{}, err
	}
	if screen != ScreenPairs && screen != ScreenFrequency && screen != ScreenTimezone {
		return Action{}, errInvalidAction
	}
	switch Operation(opPart) {
	case OpInc:
		return Action{Screen: screen, Op: OpInc, Value: 1}, nil
	case OpDec:
		return Action{Screen: screen, Op: OpDec, Value: -1}, nil
	default:
		return Action{}, errInvalidOperation
	}
}

func parseSetAction(screen Screen, valuePart string) (Action, error) {
	if screen != ScreenPairs && screen != ScreenFrequency {
		return Action{}, errInvalidAction
	}
	if !isASCIIUnsignedInt(valuePart) {
		return Action{}, errInvalidValue
	}
	value, err := strconv.Atoi(valuePart)
	if err != nil {
		return Action{}, errInvalidValue
	}
	return Action{Screen: screen, Op: OpSet, Value: value}, nil
}

func parseSetActionSigned(screen Screen, valuePart string) (Action, error) {
	if screen != ScreenTimezone {
		return Action{}, errInvalidAction
	}
	if !isASCIISignedInt(valuePart) {
		return Action{}, errInvalidValue
	}
	value, err := strconv.Atoi(valuePart)
	if err != nil {
		return Action{}, errInvalidValue
	}
	return Action{Screen: screen, Op: OpSet, Value: value}, nil
}

func parseToggleAction(screen Screen, valuePart string) (Action, error) {
	if screen != ScreenSlots {
		return Action{}, errInvalidAction
	}
	if !isASCIIUnsignedInt(valuePart) {
		return Action{}, errInvalidValue
	}
	value, err := strconv.Atoi(valuePart)
	if err != nil {
		return Action{}, errInvalidValue
	}
	if value != SlotMorning && value != SlotAfternoon && value != SlotEvening {
		return Action{}, errInvalidValue
	}
	return Action{Screen: screen, Op: OpToggle, Value: value}, nil
}

func parseScreen(screenPart string) (Screen, error) {
	switch Screen(screenPart) {
	case ScreenHome:
		return ScreenHome, nil
	case ScreenPairs:
		return ScreenPairs, nil
	case ScreenFrequency:
		return ScreenFrequency, nil
	case ScreenSlots:
		return ScreenSlots, nil
	case ScreenTimezone:
		return ScreenTimezone, nil
	case ScreenClose:
		return ScreenClose, nil
	default:
		return "", errInvalidAction
	}
}

func isASCIIUnsignedInt(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func isASCIISignedInt(value string) bool {
	if value == "" {
		return false
	}
	start := 0
	if value[0] == '-' {
		if len(value) == 1 {
			return false
		}
		start = 1
	}
	for i := start; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}
