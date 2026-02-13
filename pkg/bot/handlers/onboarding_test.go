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

func TestHandleOnboardingCallbackStopsWizardWhenInitVocabularyMissing(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	if err := db.DB.Create(&db.OnboardingState{
		UserID:       302,
		Step:         onboarding.StepChooseKnown,
		LearningLang: "en",
	}).Error; err != nil {
		t.Fatalf("failed to seed onboarding state: %v", err)
	}

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestCallbackUpdate(onboarding.BuildKnownCallback("ru"), 302, 302, 89)

	HandleOnboardingCallback(context.Background(), b, update)

	var state db.OnboardingState
	err := db.DB.Where("user_id = ?", 302).First(&state).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected onboarding state to be cleared, got err=%v state=%+v", err, state)
	}

	sawUnavailable := false
	for _, req := range client.requests {
		if strings.Contains(req.path, "editMessageText") && strings.Contains(string(req.body), "unavailable") {
			sawUnavailable = true
			break
		}
	}
	if !sawUnavailable {
		t.Fatalf("expected unavailable onboarding message")
	}
}
