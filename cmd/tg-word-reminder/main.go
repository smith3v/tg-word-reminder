// cmd/yourbot/main.go
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/go-telegram/bot"
	"github.com/smith3v/tg-word-reminder/pkg/bot/game"
	"github.com/smith3v/tg-word-reminder/pkg/bot/handlers"
	"github.com/smith3v/tg-word-reminder/pkg/bot/reminders"
	"github.com/smith3v/tg-word-reminder/pkg/config"
	"github.com/smith3v/tg-word-reminder/pkg/db"
)

var logger = slog.Default()

type botSender struct {
	b *bot.Bot
}

func (s botSender) SendMessage(ctx context.Context, chatID int64, text string) error {
	_, err := s.b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	return err
}

func main() {
	config.LoadConfig("config.json")
	if err := db.InitDB(config.AppConfig.Database); err != nil {
		logger.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithDefaultHandler(handlers.DefaultHandler),
	}
	b, err := bot.New(config.AppConfig.Telegram.Token, opts...)
	if err != nil {
		logger.Error("failed to create bot", "error", err)
		os.Exit(1)
	}

	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, handlers.HandleStart)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/settings", bot.MatchTypeExact, handlers.HandleSettings)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/clear", bot.MatchTypeExact, handlers.HandleClear)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/getpair", bot.MatchTypeExact, handlers.HandleGetPair)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/export", bot.MatchTypeExact, handlers.HandleExport)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/game", bot.MatchTypeExact, handlers.HandleGameStart)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "s:", bot.MatchTypePrefix, handlers.HandleSettingsCallback)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "g:r:", bot.MatchTypePrefix, handlers.HandleGameCallback)

	go reminders.StartPeriodicMessages(ctx, b)
	go game.StartGameSweeper(ctx, botSender{b: b})

	logger.Info("Starting bot...")
	b.Start(ctx)
}
