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

func TestUpdateUserTickersAddsNewUser(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	user1 := db.UserSettings{UserID: 1, PairsToSend: 1, RemindersPerDay: 1}
	user2 := db.UserSettings{UserID: 2, PairsToSend: 2, RemindersPerDay: 2}
	if err := db.DB.Create(&[]db.UserSettings{user1, user2}).Error; err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	tickers := []struct {
		ticker *time.Ticker
		user   db.UserSettings
	}{createUserTicker(user1)}
	defer func() {
		for _, t := range tickers {
			t.ticker.Stop()
		}
	}()

	updateUserTickers(&tickers)

	if len(tickers) != 2 {
		t.Fatalf("expected 2 tickers, got %d", len(tickers))
	}
	found := false
	for _, t := range tickers {
		if t.user.UserID == user2.UserID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected new user ticker to be added")
	}
}

func TestUpdateUserTickersUpdatesSettings(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	user := db.UserSettings{UserID: 3, PairsToSend: 1, RemindersPerDay: 1}
	if err := db.DB.Create(&user).Error; err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	tickers := []struct {
		ticker *time.Ticker
		user   db.UserSettings
	}{createUserTicker(user)}
	defer func() {
		for _, t := range tickers {
			t.ticker.Stop()
		}
	}()

	var stored db.UserSettings
	if err := db.DB.Where("user_id = ?", 3).First(&stored).Error; err != nil {
		t.Fatalf("failed to load settings: %v", err)
	}
	stored.PairsToSend = 2
	stored.RemindersPerDay = 4
	if err := db.DB.Save(&stored).Error; err != nil {
		t.Fatalf("failed to update settings: %v", err)
	}

	updateUserTickers(&tickers)

	if tickers[0].user.PairsToSend != 2 || tickers[0].user.RemindersPerDay != 4 {
		t.Fatalf("expected updated settings, got %+v", tickers[0].user)
	}
}

func TestSendRemindersNoPairsNoMessage(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	user := db.UserSettings{UserID: 10, PairsToSend: 1, RemindersPerDay: 1}

	sendReminders(context.Background(), b, user)

	if len(client.requests) != 0 {
		t.Fatalf("expected no message to be sent")
	}
}

func TestSendRemindersSendsMessage(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	if err := db.DB.Create(&db.WordPair{
		UserID: 11,
		Word1:  "Hola",
		Word2:  "Adios",
	}).Error; err != nil {
		t.Fatalf("failed to seed word pair: %v", err)
	}

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	user := db.UserSettings{UserID: 11, PairsToSend: 1, RemindersPerDay: 1}

	sendReminders(context.Background(), b, user)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Hola") || !strings.Contains(got, "Adios") {
		t.Fatalf("expected reminder to include both words, got %q", got)
	}
}

func TestStartPeriodicMessagesStopsOnCanceledContext(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		StartPeriodicMessages(ctx, b)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected StartPeriodicMessages to exit on canceled context")
	}
}
