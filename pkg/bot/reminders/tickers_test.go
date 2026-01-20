package reminders

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	telegram "github.com/go-telegram/bot"
	"github.com/smith3v/tg-word-reminder/pkg/bot/training"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

type recordedRequest struct {
	path        string
	method      string
	contentType string
	body        []byte
}

type mockClient struct {
	requests []recordedRequest
	response string
}

func newMockClient() *mockClient {
	return &mockClient{
		response: `{"ok":true,"result":{}}`,
	}
}

func (m *mockClient) Do(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	if err := req.Body.Close(); err != nil {
		return nil, fmt.Errorf("failed to close request body: %w", err)
	}
	m.requests = append(m.requests, recordedRequest{
		path:        req.URL.Path,
		method:      req.Method,
		contentType: req.Header.Get("Content-Type"),
		body:        body,
	})

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(m.response)),
		Header:     make(http.Header),
	}
	return resp, nil
}

func (m *mockClient) lastMessageText(t *testing.T) string {
	t.Helper()
	if len(m.requests) == 0 {
		t.Fatalf("expected at least one recorded request")
	}
	req := m.requests[len(m.requests)-1]

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

func newTestTelegramBot(t *testing.T, client *mockClient) *telegram.Bot {
	t.Helper()
	b, err := telegram.New("test-token",
		telegram.WithSkipGetMe(),
		telegram.WithHTTPClient(time.Second, client),
	)
	if err != nil {
		t.Fatalf("failed to create test bot: %v", err)
	}
	return b
}

func TestLatestDueSlotSelectsMostRecent(t *testing.T) {
	now := time.Date(2025, 1, 2, 14, 30, 0, 0, time.UTC)
	user := db.UserSettings{
		UserID:              1,
		ReminderMorning:     true,
		ReminderAfternoon:   true,
		ReminderEvening:     true,
		TimezoneOffsetHours: 0,
	}

	slot, ok := latestDueSlot(now, user)
	if !ok {
		t.Fatalf("expected due slot")
	}
	expected := time.Date(2025, 1, 2, 13, 0, 0, 0, time.UTC)
	if !slot.Equal(expected) {
		t.Fatalf("expected slot %v, got %v", expected, slot)
	}
}

func TestLatestDueSlotRespectsLastSent(t *testing.T) {
	now := time.Date(2025, 1, 2, 21, 0, 0, 0, time.UTC)
	lastSent := time.Date(2025, 1, 2, 20, 30, 0, 0, time.UTC)
	user := db.UserSettings{
		UserID:              1,
		ReminderMorning:     true,
		ReminderAfternoon:   true,
		ReminderEvening:     true,
		TimezoneOffsetHours: 0,
		LastTrainingSentAt:  &lastSent,
	}

	_, ok := latestDueSlot(now, user)
	if ok {
		t.Fatalf("expected no due slot after evening send")
	}
}

func TestComputeMissedCount(t *testing.T) {
	lastSent := time.Date(2025, 1, 2, 9, 0, 0, 0, time.UTC)
	lastEngaged := time.Date(2025, 1, 2, 10, 0, 0, 0, time.UTC)

	user := db.UserSettings{
		MissedTrainingSessions: 1,
		LastTrainingSentAt:     &lastSent,
	}
	if got := computeMissedCount(user); got != 2 {
		t.Fatalf("expected missed count 2, got %d", got)
	}

	user.LastTrainingEngagedAt = &lastEngaged
	if got := computeMissedCount(user); got != 0 {
		t.Fatalf("expected missed reset to 0, got %d", got)
	}
}

func TestSendTrainingSessionNoPairs(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	training.ResetDefaultManager(time.Now)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	user := db.UserSettings{UserID: 10, PairsToSend: 1}

	sent, err := sendTrainingSession(context.Background(), b, user, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent {
		t.Fatalf("expected no session to send")
	}
	if len(client.requests) != 0 {
		t.Fatalf("expected no message to be sent")
	}
}

func TestSendTrainingSessionSendsPrompt(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	training.ResetDefaultManager(time.Now)

	if err := db.DB.Create(&db.WordPair{
		UserID:   11,
		Word1:    "Hola",
		Word2:    "Adios",
		SrsState: "new",
		SrsDueAt: time.Now().Add(-time.Hour),
	}).Error; err != nil {
		t.Fatalf("failed to seed word pair: %v", err)
	}

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	user := db.UserSettings{UserID: 11, PairsToSend: 1}

	sent, err := sendTrainingSession(context.Background(), b, user, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sent {
		t.Fatalf("expected session to be sent")
	}

	got := client.lastMessageText(t)
	if !strings.Contains(got, "||") {
		t.Fatalf("expected spoiler in prompt, got %q", got)
	}
}
