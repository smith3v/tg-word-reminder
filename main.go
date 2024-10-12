// main.go
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Config struct {
	Database struct {
		Host     string `json:"host"`
		User     string `json:"user"`
		Password string `json:"password"`
		DBName   string `json:"dbname"`
		Port     int    `json:"port"`
		SSLMode  string `json:"sslmode"`
	} `json:"database"`
	Telegram struct {
		Token string `json:"token"`
	} `json:"telegram"`
}

type WordPair struct {
	ID     uint   `gorm:"primaryKey"`
	UserID int64  `gorm:"index"` // To keep pairs separate for each user
	Word1  string `gorm:"not null"`
	Word2  string `gorm:"not null"`
}

type UserSettings struct {
	ID              uint  `gorm:"primaryKey"`
	UserID          int64 `gorm:"index"`
	PairsToSend     int   `gorm:"default:1"` // Default to sending 1 pair
	RemindersPerDay int   `gorm:"default:1"` // Default to 1 reminder per day
}

var db *gorm.DB
var logger = slog.Default()
var config Config

func loadConfig() {
	file, err := os.Open("config.json")
	if err != nil {
		logger.Error("failed to open config file", "error", err)
		os.Exit(1)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		logger.Error("failed to decode config file", "error", err)
		os.Exit(1)
	}
}

func initDB() {
	var err error
	dsn := "host=" + config.Database.Host +
		" user=" + config.Database.User +
		" password=" + config.Database.Password +
		" dbname=" + config.Database.DBName +
		" port=" + strconv.Itoa(config.Database.Port) +
		" sslmode=" + config.Database.SSLMode
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	if err := db.AutoMigrate(&WordPair{}, &UserSettings{}); err != nil {
		logger.Error("failed to auto-migrate database", "error", err)
	}
}

func main() {
	loadConfig()
	initDB()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithDefaultHandler(defaultHandler),
	}

	b, err := bot.New(config.Telegram.Token, opts...)
	if err != nil {
		logger.Error("failed to create bot", "error", err)
		os.Exit(1)
	}

	b.RegisterHandler(bot.HandlerTypeMessageText, "/clear", bot.MatchTypeExact, handleClear)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/setpairs", bot.MatchTypePrefix, handleSetPairs)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/setfrequency", bot.MatchTypePrefix, handleSetFrequency)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/getpair", bot.MatchTypeExact, handleGetPair)

	go startPeriodicMessages(ctx, b)

	logger.Info("Starting bot...")
	b.Start(ctx)
}

func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil {
		logger.Error("received invalid update in defaultHandler")
		return
	}

	// Check if Chat is zero value
	if update.Message.Chat.ID == 0 {
		logger.Error("chat ID is zero in defaultHandler")
		return
	}

	// Check if the message contains a document (file)
	if update.Message.Document == nil {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Say /getpair, /setpairs, /setfrequency, or /clear to use the bot. If you attach a CSV file, I'll upload the word pairs to your account.",
		})
		if err != nil {
			logger.Error("failed to send message in defaultHandler", "error", err)
		}
		return
	}

	logger.Info("Uploading file", "file_name", update.Message.Document.FileName, "for UserID", update.Message.From.ID)

	// Check if the file is a CSV
	if !strings.HasSuffix(update.Message.Document.FileName, ".csv") {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "The uploaded file is not a CSV. Please upload a valid CSV file.",
		})
		return
	}

	// Download the file
	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: update.Message.Document.FileID})
	if err != nil {
		logger.Error("failed to get file", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to download the file. Please try again.",
		})
		return
	}

	// Construct the file URL
	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", config.Telegram.Token, file.FilePath)

	// Open the file
	resp, err := http.Get(fileURL)
	if err != nil {
		logger.Error("failed to open file", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to open the file. Please try again.",
		})
		return
	}
	defer resp.Body.Close()

	// Read the CSV file
	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		logger.Error("failed to read CSV file", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to read the CSV file. Please ensure it is in the correct format.",
		})
		return
	}

	// Process each record
	for _, record := range records {
		if len(record) != 2 {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   fmt.Sprintf("Invalid format in record: %v. Please use 'word1,word2' format.", record),
			})
			continue
		}
		wordPair := WordPair{
			UserID: update.Message.From.ID,
			Word1:  strings.TrimSpace(record[0]),
			Word2:  strings.TrimSpace(record[1]),
		}
		if err := db.Create(&wordPair).Error; err != nil {
			logger.Error("failed to create word pair", "user_id", update.Message.From.ID, "error", err)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   fmt.Sprintf("Failed to upload word pair: %v", record),
			})
		}
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Word pairs uploaded successfully.",
	})
}

func handleClear(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in handleClear")
		return
	}

	db.Where("user_id = ?", update.Message.From.ID).Delete(&WordPair{})
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Your word pair list has been cleared.",
	})
}

func handleSetPairs(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in handleSetPairs")
		return
	}

	parts := strings.Fields(update.Message.Text)
	if len(parts) != 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Please use the format: /setpairs <number>",
		})
		return
	}

	pairsCount, err := strconv.Atoi(parts[1])
	if err != nil || pairsCount <= 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Please provide a valid number of pairs to send.",
		})
		return
	}

	settings := UserSettings{UserID: update.Message.From.ID, PairsToSend: pairsCount}
	if err := db.Where("user_id = ?", update.Message.From.ID).Assign(settings).FirstOrCreate(&settings).Error; err != nil {
		logger.Error("failed to update user settings", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to update settings. Please try again.",
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("Number of pairs to send has been set to %d.", pairsCount),
	})
}

func handleSetFrequency(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in handleSetFrequency")
		return
	}

	parts := strings.Fields(update.Message.Text)
	if len(parts) != 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Please use the format: /setfrequency <number>",
		})
		return
	}

	frequency, err := strconv.Atoi(parts[1])
	if err != nil || frequency <= 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Please provide a valid number of reminders per day.",
		})
		return
	}

	settings := UserSettings{UserID: update.Message.From.ID, RemindersPerDay: frequency}
	if err := db.Where("user_id = ?", update.Message.From.ID).Assign(settings).FirstOrCreate(&settings).Error; err != nil {
		logger.Error("failed to update user settings", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to update settings. Please try again.",
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("Frequency of reminders has been set to %d per day.", frequency),
	})
}

func startPeriodicMessages(ctx context.Context, b *bot.Bot) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sendReminders(ctx, b)
		}
	}
}

func handleGetPair(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in handleGetPair")
		return
	}

	var wordPair WordPair
	if err := db.Where("user_id = ?", update.Message.From.ID).Order("RANDOM()").Limit(1).Find(&wordPair).Error; err != nil {
		logger.Error("failed to fetch random word pair for user", "user_id", update.Message.From.ID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to retrieve a word pair. Please try again later.",
		})
		return
	}

	if (wordPair == WordPair{}) {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "You have no word pairs saved. Please upload some using the /upload command.",
		})
		return
	}

	message := fmt.Sprintf("%s - ||%s||", wordPair.Word1, wordPair.Word2) // Using Telegram spoiler formatting

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   message,
	})
	if err != nil {
		logger.Error("failed to send random word pair message", "user_id", update.Message.From.ID, "error", err)
	}
}

func sendReminders(ctx context.Context, b *bot.Bot) {
	var users []UserSettings
	if err := db.Find(&users).Error; err != nil {
		logger.Error("failed to fetch users for reminders", "error", err)
		return
	}

	for _, user := range users {
		var wordPairs []WordPair
		if err := db.Where("user_id = ?", user.UserID).Order("RANDOM()").Limit(user.PairsToSend).Find(&wordPairs).Error; err != nil {
			logger.Error("failed to fetch word pairs for user", "user_id", user.UserID, "error", err)
			continue
		}

		if len(wordPairs) > 0 {
			message := "Here are your word pairs for today:\n\n"
			for _, pair := range wordPairs {
				message += fmt.Sprintf("%s - ||%s||\n", pair.Word1, pair.Word2) // Using Telegram spoiler formatting
			}
			_, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: user.UserID,
				Text:   message,
			})
			if err != nil {
				logger.Error("failed to send reminder message", "user_id", user.UserID, "error", err)
			}
		}
	}
}
