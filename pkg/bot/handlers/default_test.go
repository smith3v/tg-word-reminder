package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

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
