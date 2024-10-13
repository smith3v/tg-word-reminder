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

	// Initialize tickers for existing users
	for _, user := range users {
		tickers = append(tickers, createUserTicker(user)) // Create ticker for each user
	}

	// Ticker for checking new users every 10 minutes
	newUserTicker := time.NewTicker(10 * time.Minute)
	defer newUserTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			for _, t := range tickers {
				t.ticker.Stop() // Stop all tickers when context is done
			}
			return
		case <-newUserTicker.C:
			checkAndStartRemindersForNewUsers(ctx, b, tickers) // Check for new users
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

// Helper function to create a ticker for a user
func createUserTicker(user db.UserSettings) struct {
	ticker *time.Ticker
	user   db.UserSettings
} {
	var ticker *time.Ticker
	if user.RemindersPerDay > 24 {
		interval := time.Duration(24*60/user.RemindersPerDay) * time.Minute
		ticker = time.NewTicker(interval)
	} else {
		ticker = time.NewTicker(time.Duration(24 * int(time.Hour) / user.RemindersPerDay))
	}
	return struct {
		ticker *time.Ticker
		user   db.UserSettings
	}{ticker: ticker, user: user}
}

// Function to check for new users and start reminders for them
func checkAndStartRemindersForNewUsers(ctx context.Context, b *bot.Bot, tickers []struct {
	ticker *time.Ticker
	user   db.UserSettings
}) {
	var existingUserIDs []int64
	for _, t := range tickers {
		existingUserIDs = append(existingUserIDs, t.user.UserID) // Collect existing user IDs
	}

	var newUsers []db.UserSettings
	if err := db.DB.Where("user_id NOT IN ?", existingUserIDs).Find(&newUsers).Error; err != nil {
		logger.Error("failed to fetch new users for reminders", "error", err)
		return
	}

	for _, newUser := range newUsers {
		tickers = append(tickers, createUserTicker(newUser)) // Create ticker for new user
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
