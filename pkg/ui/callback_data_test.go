package ui

import (
	"strings"
	"testing"
)

func TestParseCallbackData(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Action
		wantErr bool
	}{
		{
			name:  "home",
			input: "s:home",
			want:  Action{Screen: ScreenHome, Op: OpNone, Value: 0},
		},
		{
			name:  "pairs",
			input: "s:pairs",
			want:  Action{Screen: ScreenPairs, Op: OpNone, Value: 0},
		},
		{
			name:  "slots",
			input: "s:slots",
			want:  Action{Screen: ScreenSlots, Op: OpNone, Value: 0},
		},
		{
			name:  "timezone",
			input: "s:tz",
			want:  Action{Screen: ScreenTimezone, Op: OpNone, Value: 0},
		},
		{
			name:  "close",
			input: "s:close",
			want:  Action{Screen: ScreenClose, Op: OpNone, Value: 0},
		},
		{
			name:  "pairs inc",
			input: "s:pairs:+1",
			want:  Action{Screen: ScreenPairs, Op: OpInc, Value: 1},
		},
		{
			name:  "pairs dec",
			input: "s:pairs:-1",
			want:  Action{Screen: ScreenPairs, Op: OpDec, Value: -1},
		},
		{
			name:  "timezone inc",
			input: "s:tz:+1",
			want:  Action{Screen: ScreenTimezone, Op: OpInc, Value: 1},
		},
		{
			name:  "timezone dec",
			input: "s:tz:-1",
			want:  Action{Screen: ScreenTimezone, Op: OpDec, Value: -1},
		},
		{
			name:  "pairs set",
			input: "s:pairs:set:5",
			want:  Action{Screen: ScreenPairs, Op: OpSet, Value: 5},
		},
		{
			name:  "timezone set",
			input: "s:tz:set:-5",
			want:  Action{Screen: ScreenTimezone, Op: OpSet, Value: -5},
		},
		{
			name:  "slots toggle morning",
			input: "s:slots:toggle:1",
			want:  Action{Screen: ScreenSlots, Op: OpToggle, Value: 1},
		},
		{
			name:    "empty",
			input:   "",
			wantErr: true,
		},
		{
			name:    "missing prefix",
			input:   "home",
			wantErr: true,
		},
		{
			name:    "empty action",
			input:   "s:",
			wantErr: true,
		},
		{
			name:    "unknown action",
			input:   "s:noop",
			wantErr: true,
		},
		{
			name:    "home with op",
			input:   "s:home:+1",
			wantErr: true,
		},
		{
			name:    "pairs set missing value",
			input:   "s:pairs:set",
			wantErr: true,
		},
		{
			name:    "pairs set negative",
			input:   "s:pairs:set:-1",
			wantErr: true,
		},
		{
			name:    "timezone set non-numeric",
			input:   "s:tz:set:abc",
			wantErr: true,
		},
		{
			name:    "slots toggle invalid",
			input:   "s:slots:toggle:4",
			wantErr: true,
		},
		{
			name:    "pairs set non-numeric",
			input:   "s:pairs:set:abc",
			wantErr: true,
		},
		{
			name:    "pairs invalid op",
			input:   "s:pairs:+2",
			wantErr: true,
		},
		{
			name:    "extra parts",
			input:   "s:tz:+1:extra",
			wantErr: true,
		},
		{
			name:    "too long",
			input:   "s:" + strings.Repeat("a", MaxCallbackDataLen),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCallbackData(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected action: got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBuilderCallbacks(t *testing.T) {
	tests := []struct {
		name  string
		build func() (string, error)
		want  Action
	}{
		{
			name:  "home",
			build: BuildHomeCallback,
			want:  Action{Screen: ScreenHome, Op: OpNone, Value: 0},
		},
		{
			name:  "pairs",
			build: BuildPairsCallback,
			want:  Action{Screen: ScreenPairs, Op: OpNone, Value: 0},
		},
		{
			name:  "slots",
			build: BuildSlotsCallback,
			want:  Action{Screen: ScreenSlots, Op: OpNone, Value: 0},
		},
		{
			name:  "timezone",
			build: BuildTimezoneCallback,
			want:  Action{Screen: ScreenTimezone, Op: OpNone, Value: 0},
		},
		{
			name:  "close",
			build: BuildCloseCallback,
			want:  Action{Screen: ScreenClose, Op: OpNone, Value: 0},
		},
		{
			name:  "pairs inc",
			build: BuildPairsIncCallback,
			want:  Action{Screen: ScreenPairs, Op: OpInc, Value: 1},
		},
		{
			name:  "pairs dec",
			build: BuildPairsDecCallback,
			want:  Action{Screen: ScreenPairs, Op: OpDec, Value: -1},
		},
		{
			name:  "timezone inc",
			build: BuildTimezoneIncCallback,
			want:  Action{Screen: ScreenTimezone, Op: OpInc, Value: 1},
		},
		{
			name:  "timezone dec",
			build: BuildTimezoneDecCallback,
			want:  Action{Screen: ScreenTimezone, Op: OpDec, Value: -1},
		},
		{
			name:  "pairs set",
			build: func() (string, error) { return BuildPairsSetCallback(5) },
			want:  Action{Screen: ScreenPairs, Op: OpSet, Value: 5},
		},
		{
			name:  "timezone set",
			build: func() (string, error) { return BuildTimezoneSetCallback(-5) },
			want:  Action{Screen: ScreenTimezone, Op: OpSet, Value: -5},
		},
		{
			name:  "slots toggle",
			build: func() (string, error) { return BuildSlotToggleCallback(SlotMorning) },
			want:  Action{Screen: ScreenSlots, Op: OpToggle, Value: SlotMorning},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.build()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(data) > MaxCallbackDataLen {
				t.Fatalf("callback data too long: %d", len(data))
			}
			got, err := ParseCallbackData(data)
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected action: got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBuildSetCallbackNegativeValue(t *testing.T) {
	if _, err := BuildPairsSetCallback(-1); err == nil {
		t.Fatalf("expected error for negative value")
	}
	if _, err := BuildTimezoneSetCallback(-5); err != nil {
		t.Fatalf("expected timezone negative to be allowed")
	}
}
