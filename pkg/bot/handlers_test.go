package bot

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
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/logger"

	dbpkg "github.com/smith3v/tg-word-reminder/pkg/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

func setupTestDB(t *testing.T) {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	if err := gdb.AutoMigrate(&dbpkg.WordPair{}, &dbpkg.UserSettings{}); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	dbpkg.DB = gdb

	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("failed to access underlying DB: %v", err)
	}

	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("failed to close database: %v", err)
		}
		dbpkg.DB = nil
	})
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

func newTestUpdate(text string, userID int64) *models.Update {
	return &models.Update{
		Message: &models.Message{
			From: &models.User{
				ID: userID,
			},
			Chat: models.Chat{
				ID: userID,
			},
			Text: text,
		},
	}
}

func TestHandleSetNumOfPairsRejectsInvalidInput(t *testing.T) {
	setupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)

	update := newTestUpdate("/setnum 0", 42)

	HandleSetNumOfPairs(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Please provide a valid number") {
		t.Fatalf("expected validation message, got %q", got)
	}

	var count int64
	if err := dbpkg.DB.Model(&dbpkg.UserSettings{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count user settings: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no user settings to be created, got %d", count)
	}
}

func TestHandleSetNumOfPairsNoArgsOpensSettings(t *testing.T) {
	setupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)

	settings := dbpkg.UserSettings{
		UserID:          42,
		PairsToSend:     2,
		RemindersPerDay: 3,
	}
	if err := dbpkg.DB.Create(&settings).Error; err != nil {
		t.Fatalf("failed to create settings: %v", err)
	}

	update := newTestUpdate("/setnum", 42)

	HandleSetNumOfPairs(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Pairs per reminder") {
		t.Fatalf("expected pairs settings message, got %q", got)
	}
	if !strings.Contains(got, "Current value: 2") {
		t.Fatalf("expected current value in message, got %q", got)
	}

	var count int64
	if err := dbpkg.DB.Model(&dbpkg.UserSettings{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count user settings: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one user setting, got %d", count)
	}
}

func TestHandleSetNumOfPairsUpdatesSettings(t *testing.T) {
	setupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/setnum 3", 101)

	HandleSetNumOfPairs(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "has been set to 3") {
		t.Fatalf("expected confirmation message, got %q", got)
	}

	var settings dbpkg.UserSettings
	if err := dbpkg.DB.Where("user_id = ?", int64(101)).First(&settings).Error; err != nil {
		t.Fatalf("expected user settings to be stored: %v", err)
	}
	if settings.PairsToSend != 3 {
		t.Fatalf("expected PairsToSend to be 3, got %d", settings.PairsToSend)
	}
}

func TestHandleSetFrequencyRejectsInvalidInput(t *testing.T) {
	setupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/setfreq zero", 202)

	HandleSetFrequency(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Please provide a valid number") {
		t.Fatalf("expected validation message, got %q", got)
	}
}

func TestHandleSetFrequencyNoArgsOpensSettings(t *testing.T) {
	setupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)

	settings := dbpkg.UserSettings{
		UserID:          202,
		PairsToSend:     2,
		RemindersPerDay: 4,
	}
	if err := dbpkg.DB.Create(&settings).Error; err != nil {
		t.Fatalf("failed to create settings: %v", err)
	}

	update := newTestUpdate("/setfreq", 202)

	HandleSetFrequency(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Reminder frequency") {
		t.Fatalf("expected frequency settings message, got %q", got)
	}
	if !strings.Contains(got, "Current value: 4") {
		t.Fatalf("expected current value in message, got %q", got)
	}

	var count int64
	if err := dbpkg.DB.Model(&dbpkg.UserSettings{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count user settings: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one user setting, got %d", count)
	}
}

func TestHandleSetFrequencyUpdatesSettings(t *testing.T) {
	setupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/setfreq 4", 303)

	HandleSetFrequency(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "has been set to 4 per day") {
		t.Fatalf("expected confirmation message, got %q", got)
	}

	var settings dbpkg.UserSettings
	if err := dbpkg.DB.Where("user_id = ?", int64(303)).First(&settings).Error; err != nil {
		t.Fatalf("expected user settings to be stored: %v", err)
	}
	if settings.RemindersPerDay != 4 {
		t.Fatalf("expected RemindersPerDay to be 4, got %d", settings.RemindersPerDay)
	}
}

func TestHandleGetPairWithoutWords(t *testing.T) {
	setupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/getpair", 404)

	HandleGetPair(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "You have no word pairs saved") {
		t.Fatalf("expected no data message, got %q", got)
	}
}

func TestHandleGetPairSendsRandomPair(t *testing.T) {
	setupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	if err := dbpkg.DB.Create(&dbpkg.WordPair{
		UserID: 505,
		Word1:  "Hola",
		Word2:  "Adios",
	}).Error; err != nil {
		t.Fatalf("failed to seed word pair: %v", err)
	}

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/getpair", 505)

	HandleGetPair(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Hola") || !strings.Contains(got, "Adios") {
		t.Fatalf("expected message to contain both words, got %q", got)
	}
}

func TestHandleClearRemovesWordPairs(t *testing.T) {
	setupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	userID := int64(606)
	pairs := []dbpkg.WordPair{
		{UserID: userID, Word1: "one", Word2: "uno"},
		{UserID: userID, Word1: "two", Word2: "dos"},
	}
	if err := dbpkg.DB.Create(&pairs).Error; err != nil {
		t.Fatalf("failed to seed word pairs: %v", err)
	}

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/clear", userID)

	HandleClear(context.Background(), b, update)

	var count int64
	if err := dbpkg.DB.Model(&dbpkg.WordPair{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count word pairs: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected word pairs to be deleted, found %d", count)
	}

	got := client.lastMessageText(t)
	if !strings.Contains(got, "has been cleared") {
		t.Fatalf("expected confirmation message, got %q", got)
	}
}
