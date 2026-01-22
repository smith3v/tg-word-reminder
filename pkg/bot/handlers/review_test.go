package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/bot/training"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

func TestHandleReviewNoPairs(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	training.ResetDefaultManager(time.Now)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/review", 3001)
	update.Message.Chat.Type = models.ChatTypePrivate

	HandleReview(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Nothing to review") {
		t.Fatalf("expected empty review message, got %q", got)
	}
}

func TestHandleReviewCallbackUpdatesPair(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	training.ResetDefaultManager(time.Now)

	if err := db.DB.Create(&db.UserSettings{
		UserID:                 3002,
		MissedTrainingSessions: 2,
		TrainingPaused:         true,
		PairsToSend:            2,
	}).Error; err != nil {
		t.Fatalf("failed to seed user settings: %v", err)
	}

	now := time.Now().UTC().Add(-time.Hour)
	pairs := []db.WordPair{
		{UserID: 3002, Word1: "hola", Word2: "adios", SrsState: "new", SrsDueAt: now},
		{UserID: 3002, Word1: "uno", Word2: "one", SrsState: "new", SrsDueAt: now},
	}
	for _, pair := range pairs {
		if err := db.DB.Create(&pair).Error; err != nil {
			t.Fatalf("failed to seed pair: %v", err)
		}
	}

	client := newMockClient()
	client.response = `{"ok":true,"result":{"message_id":55}}`
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/review", 3002)
	update.Message.Chat.Type = models.ChatTypePrivate

	HandleReview(context.Background(), b, update)

	snapshot, ok := training.DefaultManager.Snapshot(3002, 3002)
	if !ok {
		t.Fatalf("expected active review session")
	}

	callback := newTestCallbackUpdate(fmt.Sprintf("t:grade:%s:good", snapshot.Token), 3002, 3002, snapshot.MessageID)
	HandleReviewCallback(context.Background(), b, callback)

	var updated db.WordPair
	if err := db.DB.Where("id = ?", snapshot.Pair.ID).First(&updated).Error; err != nil {
		t.Fatalf("failed to load updated pair: %v", err)
	}
	if updated.SrsState != "learning" || updated.SrsStep != 1 {
		t.Fatalf("expected learning step 1 after good grade, got %+v", updated)
	}
	if updated.SrsLastReviewedAt == nil {
		t.Fatalf("expected last reviewed timestamp to be set")
	}

	var settings db.UserSettings
	if err := db.DB.Where("user_id = ?", 3002).First(&settings).Error; err != nil {
		t.Fatalf("failed to load settings: %v", err)
	}
	if settings.MissedTrainingSessions != 0 || settings.TrainingPaused {
		t.Fatalf("expected engagement reset, got %+v", settings)
	}
	if settings.LastTrainingEngagedAt == nil {
		t.Fatalf("expected engagement timestamp to be set")
	}

	sendCount := 0
	editCount := 0
	for _, req := range client.requests {
		if strings.Contains(req.path, "sendMessage") {
			sendCount++
		}
		if strings.Contains(req.path, "editMessageText") {
			editCount++
		}
	}
	if sendCount < 2 {
		t.Fatalf("expected at least two sendMessage calls, got %d", sendCount)
	}
	if editCount < 1 {
		t.Fatalf("expected editMessageText call, got %d", editCount)
	}
}

func TestHandleReviewMarksEngagementOnStart(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	training.ResetDefaultManager(time.Now)

	if err := db.DB.Create(&db.UserSettings{
		UserID:                 3003,
		MissedTrainingSessions: 1,
		TrainingPaused:         true,
		PairsToSend:            1,
	}).Error; err != nil {
		t.Fatalf("failed to seed user settings: %v", err)
	}
	if err := db.DB.Create(&db.WordPair{
		UserID:   3003,
		Word1:    "hola",
		Word2:    "adios",
		SrsState: "new",
		SrsDueAt: time.Now().Add(-time.Minute),
	}).Error; err != nil {
		t.Fatalf("failed to seed word pair: %v", err)
	}

	client := newMockClient()
	client.response = `{"ok":true,"result":{"message_id":99}}`
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/review", 3003)
	update.Message.Chat.Type = models.ChatTypePrivate

	HandleReview(context.Background(), b, update)

	var settings db.UserSettings
	if err := db.DB.Where("user_id = ?", 3003).First(&settings).Error; err != nil {
		t.Fatalf("failed to load settings: %v", err)
	}
	if settings.MissedTrainingSessions != 0 || settings.TrainingPaused {
		t.Fatalf("expected engagement reset on start, got %+v", settings)
	}
	if settings.LastTrainingEngagedAt == nil {
		t.Fatalf("expected engagement timestamp to be set")
	}
}

func TestHandleReviewCompletionMessage(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	training.ResetDefaultManager(time.Now)

	if err := db.DB.Create(&db.WordPair{
		UserID:   3004,
		Word1:    "hola",
		Word2:    "adios",
		SrsState: "new",
		SrsDueAt: time.Now().Add(-time.Minute),
	}).Error; err != nil {
		t.Fatalf("failed to seed word pair: %v", err)
	}
	if err := db.DB.Create(&db.UserSettings{
		UserID:      3004,
		PairsToSend: 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed user settings: %v", err)
	}

	client := newMockClient()
	client.response = `{"ok":true,"result":{"message_id":101}}`
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/review", 3004)
	update.Message.Chat.Type = models.ChatTypePrivate

	HandleReview(context.Background(), b, update)

	snapshot, ok := training.DefaultManager.Snapshot(3004, 3004)
	if !ok {
		t.Fatalf("expected active review session")
	}

	callback := newTestCallbackUpdate(fmt.Sprintf("t:grade:%s:good", snapshot.Token), 3004, 3004, snapshot.MessageID)
	HandleReviewCallback(context.Background(), b, callback)

	sendCount := 0
	for _, req := range client.requests {
		if strings.Contains(req.path, "sendMessage") {
			sendCount++
		}
	}
	if sendCount != 1 {
		t.Fatalf("expected only the initial prompt, got %d sendMessage calls", sendCount)
	}
}

func TestHandleOverdueCallbackCatchUp(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	training.ResetDefaultManager(time.Now)

	if err := db.DB.Create(&db.UserSettings{
		UserID:      4001,
		PairsToSend: 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}
	if err := db.DB.Create(&db.WordPair{
		UserID:   4001,
		Word1:    "hola",
		Word2:    "adios",
		SrsState: "new",
		SrsDueAt: time.Now().Add(-time.Minute),
	}).Error; err != nil {
		t.Fatalf("failed to seed word pair: %v", err)
	}

	client := newMockClient()
	client.response = `{"ok":true,"result":{"message_id":77}}`
	b := newTestTelegramBot(t, client)

	token := training.DefaultOverdue.Start(4001, 4001)
	training.DefaultOverdue.BindMessage(4001, 4001, token, 10)
	update := newTestCallbackUpdate("t:overdue:"+token+":catch", 4001, 4001, 10)

	HandleOverdueCallback(context.Background(), b, update)

	sendCount := 0
	for _, req := range client.requests {
		if strings.Contains(req.path, "sendMessage") {
			sendCount++
		}
	}
	if sendCount < 1 {
		t.Fatalf("expected sendMessage for catch-up")
	}
}

func TestHandleOverdueCallbackSnoozeEndsSession(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	training.ResetDefaultManager(time.Now)

	if err := db.DB.Create(&db.UserSettings{
		UserID:      4002,
		PairsToSend: 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}
	if err := db.DB.Create(&db.WordPair{
		UserID:   4002,
		Word1:    "hola",
		Word2:    "adios",
		SrsState: "new",
		SrsDueAt: time.Now().Add(-time.Minute),
	}).Error; err != nil {
		t.Fatalf("failed to seed word pair: %v", err)
	}

	session := training.DefaultManager.StartOrRestart(4002, 4002, []db.WordPair{
		{UserID: 4002, Word1: "hola", Word2: "adios", SrsState: "new"},
	})
	training.DefaultManager.SetCurrentMessageID(session, 22)

	client := newMockClient()
	b := newTestTelegramBot(t, client)

	token := training.DefaultOverdue.Start(4002, 4002)
	training.DefaultOverdue.BindMessage(4002, 4002, token, 22)
	update := newTestCallbackUpdate("t:overdue:"+token+":snooze1d", 4002, 4002, 22)

	HandleOverdueCallback(context.Background(), b, update)

	if session := training.DefaultManager.GetSession(4002, 4002); session != nil {
		t.Fatalf("expected session to end after snooze")
	}
}
