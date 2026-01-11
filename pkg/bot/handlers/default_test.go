package handlers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/smith3v/tg-word-reminder/pkg/config"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDefaultHandlerSendsHelpForText(t *testing.T) {
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestUpdate("hello", 100)

	DefaultHandler(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Commands:") {
		t.Fatalf("expected commands message, got %q", got)
	}
}

func TestDefaultHandlerRejectsNonCSVUpload(t *testing.T) {
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestDocumentUpdate("words.txt", "file-1", 101)

	DefaultHandler(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "not a CSV") {
		t.Fatalf("expected non-CSV warning, got %q", got)
	}
}

func TestDefaultHandlerImportsCSV(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := "word1,word2\nhola,adios\n"
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	originalConfig := config.AppConfig
	t.Cleanup(func() {
		config.AppConfig = originalConfig
	})
	config.AppConfig.Telegram.Token = "test-token"

	client := newMockClient()
	client.response = `{"ok":true,"result":{"file_path":"files/test.csv"}}`
	b := newTestTelegramBot(t, client)
	update := newTestDocumentUpdate("words.csv", "file-2", 500)

	DefaultHandler(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Imported 1 new pairs") {
		t.Fatalf("expected import confirmation, got %q", got)
	}
}
