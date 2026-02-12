package onboarding

import (
	"errors"
	"strings"
)

const (
	CallbackPrefix     = "o:"
	MaxCallbackDataLen = 64
)

type CallbackActionKind string

const (
	ActionSelectLearning CallbackActionKind = "select_learning"
	ActionSelectKnown    CallbackActionKind = "select_known"
	ActionConfirm        CallbackActionKind = "confirm"
	ActionBackLearning   CallbackActionKind = "back_learning"
	ActionBackKnown      CallbackActionKind = "back_known"
)

type CallbackAction struct {
	Kind CallbackActionKind
	Code string
}

var (
	errInvalidCallback = errors.New("invalid onboarding callback")
)

func BuildLearningCallback(code string) string {
	return CallbackPrefix + "l:" + code
}

func BuildKnownCallback(code string) string {
	return CallbackPrefix + "k:" + code
}

func BuildConfirmCallback() string {
	return CallbackPrefix + "confirm"
}

func BuildBackLearningCallback() string {
	return CallbackPrefix + "back:l"
}

func BuildBackKnownCallback() string {
	return CallbackPrefix + "back:k"
}

func ParseCallbackData(data string) (CallbackAction, error) {
	if data == "" || len(data) > MaxCallbackDataLen || !strings.HasPrefix(data, CallbackPrefix) {
		return CallbackAction{}, errInvalidCallback
	}

	payload := strings.TrimPrefix(data, CallbackPrefix)
	parts := strings.Split(payload, ":")
	if len(parts) == 0 {
		return CallbackAction{}, errInvalidCallback
	}

	switch {
	case len(parts) == 2 && parts[0] == "l" && IsSupportedLanguage(parts[1]):
		return CallbackAction{Kind: ActionSelectLearning, Code: parts[1]}, nil
	case len(parts) == 2 && parts[0] == "k" && IsSupportedLanguage(parts[1]):
		return CallbackAction{Kind: ActionSelectKnown, Code: parts[1]}, nil
	case len(parts) == 1 && parts[0] == "confirm":
		return CallbackAction{Kind: ActionConfirm}, nil
	case len(parts) == 2 && parts[0] == "back" && parts[1] == "l":
		return CallbackAction{Kind: ActionBackLearning}, nil
	case len(parts) == 2 && parts[0] == "back" && parts[1] == "k":
		return CallbackAction{Kind: ActionBackKnown}, nil
	default:
		return CallbackAction{}, errInvalidCallback
	}
}
