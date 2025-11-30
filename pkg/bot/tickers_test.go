package bot

import (
	"testing"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
)

func TestIsWithinReminderWindow(t *testing.T) {
	testLoc := time.FixedZone("Test/Zone", 0)
	stubAmsterdamLocation(t, testLoc)

	cases := []struct {
		name string
		hour int
		want bool
	}{
		{"before window", reminderWindowStartHour - 1, false},
		{"at window start", reminderWindowStartHour, true},
		{"just before end", reminderWindowEndHour - 1, true},
		{"at window end", reminderWindowEndHour, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ts := time.Date(2024, time.January, 1, tc.hour, 0, 0, 0, testLoc)
			got := isWithinReminderWindow(ts)
			if got != tc.want {
				t.Fatalf("isWithinReminderWindow(%v) = %v, want %v", ts, got, tc.want)
			}
		})
	}
}

func TestCreateUserTickerFiltersTicks(t *testing.T) {
	fake := newTestTicker(4)
	stubTickerFactory(t, fake)

	testLoc := time.FixedZone("Test/Zone", 0)
	stubAmsterdamLocation(t, testLoc)

	user := db.UserSettings{RemindersPerDay: reminderWindowHours}
	ut := createUserTicker(user)
	t.Cleanup(ut.stopTicker)

	allowed := time.Date(2024, time.January, 1, reminderWindowStartHour+1, 0, 0, 0, testLoc)
	fake.emit(allowed)
	select {
	case <-ut.channel:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected tick within window to be delivered")
	}

	blocked := time.Date(2024, time.January, 1, reminderWindowEndHour+1, 0, 0, 0, testLoc)
	fake.emit(blocked)
	select {
	case <-ut.channel:
		t.Fatalf("unexpected tick outside window delivered")
	case <-time.After(50 * time.Millisecond):
	}

	ut.stopTicker()
	if !fake.stopInvoked() {
		t.Fatalf("expected stop to propagate to underlying ticker")
	}

	fake.emit(allowed)
	select {
	case <-ut.channel:
		t.Fatalf("tick delivered after ticker stopped")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestCreateUserTickerDefaultsToSingleReminder(t *testing.T) {
	fake := newTestTicker(1)
	stubTickerFactory(t, fake)

	ut := createUserTicker(db.UserSettings{RemindersPerDay: 0})
	t.Cleanup(ut.stopTicker)

	if got := fake.lastDuration(); got != reminderWindowDuration {
		t.Fatalf("ticker requested duration %v, want %v", got, reminderWindowDuration)
	}
}

func stubTickerFactory(t *testing.T, fake *testTicker) {
	t.Helper()
	prev := tickerFactory
	tickerFactory = fake.factory()
	t.Cleanup(func() {
		tickerFactory = prev
	})
}

func stubAmsterdamLocation(t *testing.T, loc *time.Location) {
	t.Helper()
	prev := amsterdamLocation
	amsterdamLocation = loc
	t.Cleanup(func() {
		amsterdamLocation = prev
	})
}

type testTicker struct {
	ch         chan time.Time
	duration   time.Duration
	stopCalled bool
}

func newTestTicker(buffer int) *testTicker {
	return &testTicker{
		ch: make(chan time.Time, buffer),
	}
}

func (tt *testTicker) factory() func(time.Duration) tickerHandle {
	return func(d time.Duration) tickerHandle {
		tt.duration = d
		return tickerHandle{
			C: tt.ch,
			stop: func() {
				tt.stopCalled = true
			},
		}
	}
}

func (tt *testTicker) emit(ts time.Time) {
	tt.ch <- ts
}

func (tt *testTicker) stopInvoked() bool {
	return tt.stopCalled
}

func (tt *testTicker) lastDuration() time.Duration {
	return tt.duration
}
