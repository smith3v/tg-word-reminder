package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

func StartPeriodicMessages(ctx context.Context, b *bot.Bot) {
	var users []db.UserSettings
	if err := db.DB.Find(&users).Error; err != nil {
		logger.Error("failed to fetch users for reminders", "error", err)
		return
	}

	var tickers []struct {
		ticker *time.Ticker
		user   db.UserSettings
	}

	for _, user := range users {
		var ticker *time.Ticker
		if user.RemindersPerDay > 24 {
			// Calculate interval in minutes for reminders greater than 24
			interval := time.Duration(24*60/user.RemindersPerDay) * time.Minute
			ticker = time.NewTicker(interval) // Set ticker based on calculated interval
		} else {
			ticker = time.NewTicker(time.Duration(24 * int(time.Hour) / user.RemindersPerDay)) // Calculate based on RemindersPerDay
		}
		tickers = append(tickers, struct {
			ticker *time.Ticker
			user   db.UserSettings
		}{ticker: ticker, user: user}) // Store the ticker and user settings
	}

	for {
		select {
		case <-ctx.Done():
			for _, t := range tickers {
				t.ticker.Stop() // Stop all tickers when context is done
			}
			return
		default:
			for _, t := range tickers {
				select {
				case <-t.ticker.C:
					sendReminders(ctx, b, t.user) // Send reminders for the corresponding user
				default:
					continue
				}
			}
		}
	}
}

func sendReminders(ctx context.Context, b *bot.Bot, user db.UserSettings) {
	var wordPairs []db.WordPair
	if err := db.DB.Where("user_id = ?", user.UserID).Order("RANDOM()").Limit(user.PairsToSend).Find(&wordPairs).Error; err != nil {
		logger.Error("failed to fetch word pairs for user", "user_id", user.UserID, "error", err)
		return
	}

	if len(wordPairs) > 0 {
		message := ""
		for _, pair := range wordPairs {
			message += fmt.Sprintf("%s  ||%s||\n", bot.EscapeMarkdown(pair.Word1), bot.EscapeMarkdown(pair.Word2)) // Using Telegram spoiler formatting
		}
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    user.UserID,
			Text:      message,
			ParseMode: models.ParseModeMarkdown,
		})
		if err != nil {
			logger.Error("failed to send reminder message", "user_id", user.UserID, "error", err)
		}
	}
}
