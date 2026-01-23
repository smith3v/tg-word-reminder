package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/bot/feedback"
	"github.com/smith3v/tg-word-reminder/pkg/config"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

func TestHandleFeedbackPrivateChat(t *testing.T) {
	logger.SetLogLevel(logger.ERROR)

	original := config.AppConfig
	t.Cleanup(func() { config.AppConfig = original })
	config.AppConfig.Feedback = config.FeedbackConfig{
		Enabled:        true,
		AdminIDs:       []int64{999},
		TimeoutMinutes: 5,
	}

	feedback.ResetDefaultManager(nil)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/feedback", 301)
	update.Message.Chat.Type = models.ChatTypePrivate

	HandleFeedback(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "within 5 minutes") {
		t.Fatalf("expected feedback prompt, got %q", got)
	}

	if ok := feedback.DefaultManager.Consume(301, 301, time.Now().UTC()); !ok {
		t.Fatalf("expected pending feedback to be created")
	}
}

func TestHandleFeedbackNonPrivateChat(t *testing.T) {
	logger.SetLogLevel(logger.ERROR)

	original := config.AppConfig
	t.Cleanup(func() { config.AppConfig = original })
	config.AppConfig.Feedback = config.FeedbackConfig{
		Enabled:        true,
		AdminIDs:       []int64{999},
		TimeoutMinutes: 5,
	}

	feedback.ResetDefaultManager(nil)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/feedback", 302)
	update.Message.Chat.Type = models.ChatTypeGroup

	HandleFeedback(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "private chat") {
		t.Fatalf("expected private chat warning, got %q", got)
	}

	if ok := feedback.DefaultManager.Consume(302, 302, time.Now().UTC()); ok {
		t.Fatalf("expected no pending feedback in non-private chat")
	}
}

func TestHandleFeedbackMissingAdmins(t *testing.T) {
	logger.SetLogLevel(logger.ERROR)

	original := config.AppConfig
	t.Cleanup(func() { config.AppConfig = original })
	config.AppConfig.Feedback = config.FeedbackConfig{
		Enabled:        true,
		AdminIDs:       []int64{},
		TimeoutMinutes: 5,
	}

	feedback.ResetDefaultManager(nil)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/feedback", 303)
	update.Message.Chat.Type = models.ChatTypePrivate

	HandleFeedback(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "not configured") {
		t.Fatalf("expected not configured message, got %q", got)
	}

	if ok := feedback.DefaultManager.Consume(303, 303, time.Now().UTC()); ok {
		t.Fatalf("expected no pending feedback when admins are missing")
	}
}
