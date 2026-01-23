package handlers

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
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

func TestTryHandleFeedbackCapture(t *testing.T) {
	logger.SetLogLevel(logger.ERROR)

	original := config.AppConfig
	t.Cleanup(func() { config.AppConfig = original })
	config.AppConfig.Feedback = config.FeedbackConfig{
		Enabled:        true,
		AdminIDs:       []int64{901, 902},
		TimeoutMinutes: 5,
	}

	feedback.ResetDefaultManager(nil)
	start := time.Now().UTC()
	feedback.DefaultManager.Start(401, 401, start, 5*time.Minute)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("I like this", 401)
	update.Message.Chat.Type = models.ChatTypePrivate
	update.Message.ID = 55

	if handled := tryHandleFeedbackCapture(context.Background(), b, update); !handled {
		t.Fatalf("expected feedback capture to handle message")
	}

	if ok := feedback.DefaultManager.Consume(401, 401, time.Now().UTC()); ok {
		t.Fatalf("expected pending feedback to be cleared")
	}

	sendCount := 0
	forwardCount := 0
	summaryCount := 0
	confirmCount := 0
	for _, req := range client.requests {
		if strings.Contains(req.path, "sendMessage") {
			sendCount++
			text := messageTextFromRequest(t, req)
			if strings.Contains(text, "Feedback received") {
				summaryCount++
				if !strings.Contains(text, "> I like this") {
					t.Fatalf("expected summary to quote feedback, got %q", text)
				}
				if !strings.Contains(text, "ID 401") {
					t.Fatalf("expected summary to include user ID, got %q", text)
				}
			} else if strings.Contains(text, "Thanks for your feedback!") {
				confirmCount++
			}
		}
		if strings.Contains(req.path, "forwardMessage") {
			forwardCount++
		}
	}
	if sendCount != 3 {
		t.Fatalf("expected 3 sendMessage requests, got %d", sendCount)
	}
	if forwardCount != 2 {
		t.Fatalf("expected 2 forwardMessage requests, got %d", forwardCount)
	}
	if summaryCount != 2 {
		t.Fatalf("expected 2 summary messages, got %d", summaryCount)
	}
	if confirmCount != 1 {
		t.Fatalf("expected 1 confirmation message, got %d", confirmCount)
	}
}

func messageTextFromRequest(t *testing.T, req recordedRequest) string {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(req.contentType)
	if err != nil {
		t.Fatalf("failed to parse media type: %v", err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		t.Fatalf("unexpected media type: %s", mediaType)
	}

	reader := multipart.NewReader(bytes.NewReader(req.body), params["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read multipart part: %v", err)
		}
		if part.FormName() == "text" {
			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("failed to read text part: %v", err)
			}
			return string(data)
		}
	}
	t.Fatalf("text field not found in request")
	return ""
}
