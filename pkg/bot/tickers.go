package bot

import (
	"context"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

const (
	reminderWindowStartHour = 8
	reminderWindowEndHour   = 22
	reminderWindowHours     = reminderWindowEndHour - reminderWindowStartHour
	reminderWindowMinutes   = reminderWindowHours * 60
)

var (
	reminderWindowDuration = time.Duration(reminderWindowHours) * time.Hour
	amsterdamLocation      = loadAmsterdamLocation()
)

type tickerHandle struct {
	C    <-chan time.Time
	stop func()
}

var tickerFactory = func(d time.Duration) tickerHandle {
	t := time.NewTicker(d)
	return tickerHandle{
		C: t.C,
		stop: func() {
			t.Stop()
		},
	}
}

type userTicker struct {
	stopFunc func()
	channel  <-chan time.Time
	stop     chan struct{}
	user     db.UserSettings
}

func (t *userTicker) stopTicker() {
	if t == nil {
		return
	}
	if t.stopFunc != nil {
		t.stopFunc()
	}
	if t.stop == nil {
		return
	}
	select {
	case <-t.stop:
	default:
		close(t.stop)
	}
}

func isWithinReminderWindow(ts time.Time) bool {
	localTime := ts.In(amsterdamLocation)
	start := time.Date(localTime.Year(), localTime.Month(), localTime.Day(), reminderWindowStartHour, 0, 0, 0, amsterdamLocation)
	end := time.Date(localTime.Year(), localTime.Month(), localTime.Day(), reminderWindowEndHour, 0, 0, 0, amsterdamLocation)
	return !localTime.Before(start) && localTime.Before(end)
}

func loadAmsterdamLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		logger.Error("failed to load Amsterdam timezone, falling back to UTC", "error", err)
		return time.UTC
	}
	return loc
}

func StartPeriodicMessages(ctx context.Context, b *bot.Bot) {
	var users []db.UserSettings
	if err := db.DB.Find(&users).Error; err != nil {
		logger.Error("failed to fetch users for reminders", "error", err)
		return
	}

	var tickers []userTicker

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
			for i := range tickers {
				tickers[i].stopTicker() // Stop all tickers when context is done
			}
			return
		case <-settingsUpdateTicker.C:
			updateUserTickers(&tickers) // Check for user settings updates and new users
		default:
			time.Sleep(1000 * time.Millisecond) // Adjust the duration as needed
			for i := range tickers {
				select {
				case <-tickers[i].channel:
					sendReminders(ctx, b, tickers[i].user) // Send reminders for the corresponding user
				default:
					continue
				}
			}
		}
	}
}

// Helper function to create a ticker for a user
func createUserTicker(user db.UserSettings) userTicker {
	remindersPerDay := user.RemindersPerDay
	if remindersPerDay <= 0 {
		remindersPerDay = 1
	}

	var interval time.Duration
	switch {
	case remindersPerDay >= reminderWindowMinutes:
		interval = time.Minute
	case remindersPerDay > reminderWindowHours:
		interval = time.Duration(reminderWindowMinutes/remindersPerDay) * time.Minute
	default:
		interval = reminderWindowDuration / time.Duration(remindersPerDay)
	}
	if interval <= 0 {
		interval = time.Minute
	}

	source := tickerFactory(interval)

	stop := make(chan struct{})
	filtered := make(chan time.Time, 1)
	go func() {
		for {
			select {
			case <-stop:
				return
			case tick := <-source.C:
				if !isWithinReminderWindow(tick) {
					continue
				}
				select {
				case <-stop:
					return
				case filtered <- tick:
				}
			}
		}
	}()

	return userTicker{
		stopFunc: func() {
			if source.stop != nil {
				source.stop()
			}
		},
		channel: filtered,
		stop:    stop,
		user:    user,
	}
}

// Function to update user tickers based on settings changes and check for new users
func updateUserTickers(tickers *[]userTicker) {
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
						(*tickers)[i].stopTicker()             // Stop the old ticker
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
