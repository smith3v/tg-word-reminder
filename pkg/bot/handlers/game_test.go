package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/bot/game"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
	"github.com/smith3v/tg-word-reminder/pkg/ui"
	"gorm.io/datatypes"
)

func resetGameManager(now func() time.Time) {
	game.ResetDefaultManager(now)
}

func TestHandleGameStartRejectsNonPrivateChat(t *testing.T) {
	testutil.SetupTestDB(t)
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
	testutil.SetupTestDB(t)
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
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	resetGameManager(time.Now)

	if err := db.DB.Create(&db.WordPair{
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

func TestHandleGameStartResumesPersistedSession(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	resetGameManager(time.Now)

	now := time.Now().UTC()
	pair := db.WordPair{UserID: 919, Word1: "Hola", Word2: "Adios"}
	if err := db.DB.Create(&pair).Error; err != nil {
		t.Fatalf("failed to seed word pair: %v", err)
	}

	deck := []map[string]interface{}{
		{
			"pair_id":   pair.ID,
			"direction": game.DirectionAToB,
		},
	}
	raw, err := json.Marshal(deck)
	if err != nil {
		t.Fatalf("failed to marshal deck: %v", err)
	}
	if err := db.DB.Create(&db.GameSession{
		ChatID:           919,
		UserID:           919,
		PairIDs:          datatypes.JSON(raw),
		CurrentIndex:     0,
		CurrentToken:     "tok",
		CurrentMessageID: 11,
		LastActivityAt:   now,
		ExpiresAt:        now.Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("failed to seed game session state: %v", err)
	}

	client := newMockClient()
	client.response = `{"ok":true,"result":{"message_id":77}}`
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("/game", 919)
	update.Message.Chat.Type = models.ChatTypePrivate

	HandleGameStart(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Hola") {
		t.Fatalf("expected resumed prompt, got %q", got)
	}

	body := client.lastRequestBody(t)
	if !strings.Contains(body, "g:r:tok") {
		t.Fatalf("expected resumed token in callback data, got %q", body)
	}
}

func TestHandleGameTextAttemptNextPromptOmitsHint(t *testing.T) {
	logger.SetLogLevel(logger.ERROR)
	resetGameManager(time.Now)

	pairs := []db.WordPair{
		{UserID: 1001, Word1: "hola", Word2: "adios"},
	}
	session := game.DefaultManager.StartOrRestart(1001, 1001, pairs)
	if session == nil || session.CurrentCard() == nil {
		t.Fatalf("expected session with current card")
	}
	game.DefaultManager.SetCurrentMessageID(session, 777)

	client := newMockClient()
	client.response = `{"ok":true,"result":{"message_id":55}}`
	b := newTestTelegramBot(t, client)
	update := newTestUpdate(session.CurrentCard().Expected, 1001)
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

func TestHandleGameCallbackIgnoresStaleToken(t *testing.T) {
	logger.SetLogLevel(logger.ERROR)
	resetGameManager(time.Now)

	pairs := []db.WordPair{
		{UserID: 2001, Word1: "hola", Word2: "adios"},
	}
	session := game.DefaultManager.StartOrRestart(2001, 2001, pairs)
	if session == nil || session.CurrentCard() == nil {
		t.Fatalf("expected session with current card")
	}
	game.DefaultManager.SetCurrentMessageID(session, 44)
	badToken := session.CurrentToken() + "x"

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestCallbackUpdate("g:r:"+badToken, 2001, 2001, 44)

	HandleGameCallback(context.Background(), b, update)

	if len(client.requests) == 0 {
		t.Fatalf("expected callback answer request")
	}
	for _, req := range client.requests {
		if strings.Contains(req.path, "editMessageText") {
			t.Fatalf("did not expect editMessageText on stale callback")
		}
	}
}

func TestHandleGameCallbackRevealRequeuesAndEdits(t *testing.T) {
	logger.SetLogLevel(logger.ERROR)
	resetGameManager(time.Now)

	pairs := []db.WordPair{
		{UserID: 2002, Word1: "hola", Word2: "adios"},
		{UserID: 2002, Word1: "uno", Word2: "one"},
	}
	session := game.DefaultManager.StartOrRestart(2002, 2002, pairs)
	if session == nil || session.CurrentCard() == nil {
		t.Fatalf("expected session with current card")
	}
	game.DefaultManager.SetCurrentMessageID(session, 55)

	client := newMockClient()
	client.response = `{"ok":true,"result":{"message_id":99}}`
	b := newTestTelegramBot(t, client)
	update := newTestCallbackUpdate("g:r:"+session.CurrentToken(), 2002, 2002, 55)

	HandleGameCallback(context.Background(), b, update)

	updated := game.DefaultManager.GetSession(2002, 2002)
	if updated == nil {
		t.Fatalf("expected session to remain active")
	}

	body := client.lastRequestBody(t)
	if !strings.Contains(body, "👀") {
		t.Fatalf("expected reveal marker in edit text")
	}
}

func TestHandleGameCallbackResumesPersistedSession(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)
	resetGameManager(time.Now)

	pair := db.WordPair{UserID: 2101, Word1: "Hola", Word2: "Adios"}
	if err := db.DB.Create(&pair).Error; err != nil {
		t.Fatalf("failed to seed word pair: %v", err)
	}

	deck := []map[string]interface{}{
		{
			"pair_id":   pair.ID,
			"direction": game.DirectionAToB,
		},
	}
	raw, err := json.Marshal(deck)
	if err != nil {
		t.Fatalf("failed to marshal deck: %v", err)
	}
	if err := db.DB.Create(&db.GameSession{
		ChatID:           2101,
		UserID:           2101,
		PairIDs:          datatypes.JSON(raw),
		CurrentIndex:     0,
		CurrentToken:     "tok",
		CurrentMessageID: 11,
		LastActivityAt:   time.Now().UTC(),
		ExpiresAt:        time.Now().UTC().Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("failed to seed game session: %v", err)
	}

	resetGameManager(time.Now)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestCallbackUpdate("g:r:tok", 2101, 2101, 11)

	HandleGameCallback(context.Background(), b, update)

	edited := false
	for _, req := range client.requests {
		if strings.Contains(req.path, "editMessageText") {
			edited = true
			break
		}
	}
	if !edited {
		t.Fatalf("expected editMessageText after resuming session")
	}
}

func TestFormatGameRevealTextAddsSpoilerAndEscapes(t *testing.T) {
	card := game.Card{
		Shown:    "hello_world",
		Expected: "word[1]",
	}
	got := formatGameRevealText(card, "✅")
	expected := "hello\\_world — ||word\\[1\\]|| ✅"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
