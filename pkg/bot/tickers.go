package bot

import (
	"context"
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

	// Ticker for checking user settings and new users every 5 minutes
	settingsUpdateTicker := time.NewTicker(5 * time.Minute)
	defer settingsUpdateTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			for _, t := range tickers {
				t.ticker.Stop() // Stop all tickers when context is done
			}
			return
		case <-settingsUpdateTicker.C:
			updateUserTickers(&tickers) // Check for user settings updates and new users
		default:
			time.Sleep(1000 * time.Millisecond) // Adjust the duration as needed
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

// Function to update user tickers based on settings changes and check for new users
func updateUserTickers(tickers *[]struct {
	ticker *time.Ticker
	user   db.UserSettings
}) {
	var users []db.UserSettings
	if err := db.DB.Find(&users).Error; err != nil {
		logger.Error("failed to fetch users for settings update", "error", err)
		return
	}

	existingUserIDs := make(map[int64]struct{})
	for _, t := range *tickers {
		existingUserIDs[t.user.UserID] = struct{}{} // Track existing user IDs
	}

	for _, user := range users {
		if _, exists := existingUserIDs[user.UserID]; !exists {
			logger.Debug("new user detected", "user_id", user.UserID)
			*tickers = append(*tickers, createUserTicker(user)) // Create ticker for new user
		} else {
			// Check if the settings have changed
			for i, t := range *tickers {
				if t.user.UserID == user.UserID {
					if t.user.RemindersPerDay != user.RemindersPerDay || t.user.PairsToSend != user.PairsToSend {
						logger.Debug("user settings updated", "user_id", user.UserID, "old_settings", t.user, "new_settings", user)
						t.ticker.Stop()                        // Stop the old ticker
						(*tickers)[i] = createUserTicker(user) // Recreate the ticker with updated settings
					}
					break
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
			message += PrepareWordPairMessage(pair.Word1, pair.Word2)
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
