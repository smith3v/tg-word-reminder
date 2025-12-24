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
			name:  "frequency",
			input: "s:freq",
			want:  Action{Screen: ScreenFrequency, Op: OpNone, Value: 0},
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
			name:  "frequency inc",
			input: "s:freq:+1",
			want:  Action{Screen: ScreenFrequency, Op: OpInc, Value: 1},
		},
		{
			name:  "frequency dec",
			input: "s:freq:-1",
			want:  Action{Screen: ScreenFrequency, Op: OpDec, Value: -1},
		},
		{
			name:  "pairs set",
			input: "s:pairs:set:5",
			want:  Action{Screen: ScreenPairs, Op: OpSet, Value: 5},
		},
		{
			name:  "frequency set",
			input: "s:freq:set:10",
			want:  Action{Screen: ScreenFrequency, Op: OpSet, Value: 10},
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
			input:   "s:freq:+1:extra",
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
			name:  "frequency",
			build: BuildFrequencyCallback,
			want:  Action{Screen: ScreenFrequency, Op: OpNone, Value: 0},
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
			name:  "frequency inc",
			build: BuildFrequencyIncCallback,
			want:  Action{Screen: ScreenFrequency, Op: OpInc, Value: 1},
		},
		{
			name:  "frequency dec",
			build: BuildFrequencyDecCallback,
			want:  Action{Screen: ScreenFrequency, Op: OpDec, Value: -1},
		},
		{
			name:  "pairs set",
			build: func() (string, error) { return BuildPairsSetCallback(5) },
			want:  Action{Screen: ScreenPairs, Op: OpSet, Value: 5},
		},
		{
			name:  "frequency set",
			build: func() (string, error) { return BuildFrequencySetCallback(10) },
			want:  Action{Screen: ScreenFrequency, Op: OpSet, Value: 10},
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
	if _, err := BuildFrequencySetCallback(-1); err == nil {
		t.Fatalf("expected error for negative value")
	}
}
