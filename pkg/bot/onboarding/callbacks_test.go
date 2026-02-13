package onboarding

import "testing"

func TestParseCallbackData(t *testing.T) {
	tests := []struct {
		name string
		data string
		kind CallbackActionKind
		code string
	}{
		{name: "learning", data: BuildLearningCallback("en"), kind: ActionSelectLearning, code: "en"},
		{name: "known", data: BuildKnownCallback("ru"), kind: ActionSelectKnown, code: "ru"},
		{name: "confirm", data: BuildConfirmCallback(), kind: ActionConfirm},
		{name: "back learning", data: BuildBackLearningCallback(), kind: ActionBackLearning},
		{name: "back known", data: BuildBackKnownCallback(), kind: ActionBackKnown},
		{name: "cancel reset", data: BuildCancelResetCallback(), kind: ActionCancelReset},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			action, err := ParseCallbackData(tc.data)
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
			if action.Kind != tc.kind || action.Code != tc.code {
				t.Fatalf("unexpected action: %+v", action)
			}
		})
	}
}

func TestParseCallbackDataRejectsInvalid(t *testing.T) {
	if _, err := ParseCallbackData("o:l:xx"); err == nil {
		t.Fatalf("expected invalid language callback to fail")
	}
	if _, err := ParseCallbackData("s:home"); err == nil {
		t.Fatalf("expected invalid prefix callback to fail")
	}
}
