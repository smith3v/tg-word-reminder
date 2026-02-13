package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/smith3v/tg-word-reminder/pkg/bot/onboarding"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
	"gorm.io/gorm"
)

func TestHandleOnboardingCallbackCancelResetClearsState(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	if err := db.DB.Create(&db.OnboardingState{UserID: 301, AwaitingResetPhrase: true}).Error; err != nil {
		t.Fatalf("failed to seed onboarding state: %v", err)
	}

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestCallbackUpdate(onboarding.BuildCancelResetCallback(), 301, 301, 88)

	HandleOnboardingCallback(context.Background(), b, update)

	var state db.OnboardingState
	err := db.DB.Where("user_id = ?", 301).First(&state).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected onboarding state to be cleared, got err=%v state=%+v", err, state)
	}

	sawEdit := false
	for _, req := range client.requests {
		if strings.Contains(req.path, "editMessageText") && strings.Contains(string(req.body), "Reset canceled. Your data is unchanged.") {
			sawEdit = true
			break
		}
	}
	if !sawEdit {
		t.Fatalf("expected editMessageText with reset canceled text")
	}
}
