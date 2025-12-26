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
	"github.com/smith3v/tg-word-reminder/pkg/ui"

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

func (m *mockClient) lastRequestBody(t *testing.T) string {
	t.Helper()
	if len(m.requests) == 0 {
		t.Fatalf("expected at least one recorded request")
	}
	return string(m.requests[len(m.requests)-1].body)
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

func resetGameManager(now func() time.Time) {
	gameManager = NewGameManager(now)
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

func TestHandleGameStartRejectsNonPrivateChat(t *testing.T) {
	setupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	resetGameManager(time.Now)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/game", 707)
	update.Message.Chat.Type = models.ChatTypeGroup

	HandleGameStart(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "only in private chat") {
		t.Fatalf("expected private chat warning, got %q", got)
	}
}

func TestHandleGameStartWithEmptyVocabulary(t *testing.T) {
	setupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	resetGameManager(time.Now)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/game", 808)
	update.Message.Chat.Type = models.ChatTypePrivate

	HandleGameStart(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "You have no word pairs saved") {
		t.Fatalf("expected empty vocabulary message, got %q", got)
	}
}

func TestHandleGameStartSendsPromptWithCallback(t *testing.T) {
	setupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	resetGameManager(time.Now)

	if err := dbpkg.DB.Create(&dbpkg.WordPair{
		UserID: 909,
		Word1:  "Hola",
		Word2:  "Adios",
	}).Error; err != nil {
		t.Fatalf("failed to seed word pair: %v", err)
	}

	client := newMockClient()
	client.response = `{"ok":true,"result":{"message_id":42}}`
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/game", 909)
	update.Message.Chat.Type = models.ChatTypePrivate

	HandleGameStart(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "→ ?") {
		t.Fatalf("expected prompt format with arrow, got %q", got)
	}
	if !strings.Contains(got, "(reply with the missing word, or tap") {
		t.Fatalf("expected prompt hint, got %q", got)
	}

	body := client.lastRequestBody(t)
	idx := strings.Index(body, "g:r:")
	if idx == -1 {
		t.Fatalf("expected callback_data with g:r: prefix")
	}
	token := body[idx:]
	end := strings.Index(token, "\"")
	if end == -1 {
		t.Fatalf("expected closing quote for callback_data")
	}
	callback := token[:end]
	if len(callback) > ui.MaxCallbackDataLen {
		t.Fatalf("callback_data too long: %d", len(callback))
	}
}

func TestHandleGameTextAttemptNextPromptOmitsHint(t *testing.T) {
	logger.SetLogLevel(logger.ERROR)
	resetGameManager(time.Now)

	current := Card{PairID: 1, Direction: DirectionAToB, Shown: "hola", Expected: "adios"}
	next := Card{PairID: 2, Direction: DirectionBToA, Shown: "uno", Expected: "one"}
	session := &GameSession{
		chatID:           1001,
		userID:           1001,
		currentCard:      &current,
		currentMessageID: 777,
		currentResolved:  false,
		deck:             []Card{next},
	}
	key := getSessionKey(1001, 1001)
	gameManager.mu.Lock()
	gameManager.sessions[key] = session
	gameManager.mu.Unlock()

	client := newMockClient()
	client.response = `{"ok":true,"result":{"message_id":55}}`
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("adios", 1001)
	update.Message.Chat.Type = models.ChatTypePrivate

	handled := handleGameTextAttempt(context.Background(), b, update)
	if !handled {
		t.Fatalf("expected attempt to be handled")
	}

	got := client.lastMessageText(t)
	if strings.Contains(got, "reply with the missing word") {
		t.Fatalf("expected hint to be omitted, got %q", got)
	}
	if !strings.Contains(got, "→ ?") {
		t.Fatalf("expected prompt format with arrow, got %q", got)
	}
}

func newTestCallbackUpdate(data string, userID, chatID int64, messageID int) *models.Update {
	return &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "callback-1",
			From: models.User{ID: userID},
			Data: data,
			Message: models.MaybeInaccessibleMessage{
				Type: models.MaybeInaccessibleMessageTypeMessage,
				Message: &models.Message{
					ID: messageID,
					Chat: models.Chat{
						ID:   chatID,
						Type: models.ChatTypePrivate,
					},
				},
			},
		},
	}
}

func TestHandleGameCallbackIgnoresStaleToken(t *testing.T) {
	logger.SetLogLevel(logger.ERROR)
	resetGameManager(time.Now)

	current := Card{PairID: 1, Direction: DirectionAToB, Shown: "hola", Expected: "adios"}
	session := &GameSession{
		chatID:           2001,
		userID:           2001,
		currentCard:      &current,
		currentMessageID: 44,
		currentResolved:  false,
		currentToken:     "good",
	}
	key := getSessionKey(2001, 2001)
	gameManager.mu.Lock()
	gameManager.sessions[key] = session
	gameManager.mu.Unlock()

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestCallbackUpdate("g:r:bad", 2001, 2001, 44)

	HandleGameCallback(context.Background(), b, update)

	if len(client.requests) == 0 {
		t.Fatalf("expected callback answer request")
	}
	for _, req := range client.requests {
		if strings.Contains(req.path, "editMessageText") {
			t.Fatalf("did not expect editMessageText on stale callback")
		}
	}
	if session.attemptCount != 0 {
		t.Fatalf("expected no attempts recorded for stale callback")
	}
}

func TestHandleGameCallbackRevealRequeuesAndEdits(t *testing.T) {
	logger.SetLogLevel(logger.ERROR)
	resetGameManager(time.Now)

	current := Card{PairID: 1, Direction: DirectionAToB, Shown: "hola", Expected: "adios"}
	next := Card{PairID: 2, Direction: DirectionBToA, Shown: "uno", Expected: "one"}
	session := &GameSession{
		chatID:           2002,
		userID:           2002,
		currentCard:      &current,
		currentMessageID: 55,
		currentResolved:  false,
		currentToken:     "token",
		deck:             []Card{next},
	}
	key := getSessionKey(2002, 2002)
	gameManager.mu.Lock()
	gameManager.sessions[key] = session
	gameManager.mu.Unlock()

	client := newMockClient()
	client.response = `{"ok":true,"result":{"message_id":99}}`
	b := newTestTelegramBot(t, client)
	update := newTestCallbackUpdate("g:r:token", 2002, 2002, 55)

	HandleGameCallback(context.Background(), b, update)

	updated := gameManager.GetSession(2002, 2002)
	if updated == nil {
		t.Fatalf("expected session to remain active")
	}
	if updated.attemptCount != 1 || updated.correctCount != 0 {
		t.Fatalf("expected attempt count to increment, got attempts=%d correct=%d", updated.attemptCount, updated.correctCount)
	}
	if len(updated.deck) != 1 || updated.deck[0] != current {
		t.Fatalf("expected current card to be requeued, got %+v", updated.deck)
	}

	body := client.lastRequestBody(t)
	if !strings.Contains(body, "👀") {
		t.Fatalf("expected reveal marker in edit text")
	}
}
