// main.go
package main

import (
    "context"
    "encoding/json"
    "os"
	"os/signal"
    "strconv"
    "strings"
    "time"
	"log/slog"

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
    ID             uint `gorm:"primaryKey"`
    UserID         int64 `gorm:"index"`
    PairsToSend    int   `gorm:"default:1"` // Default to sending 1 pair
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

	b.RegisterHandler(bot.HandlerTypeMessageText, "/upload", bot.MatchTypeExact, handleUpload)
    b.RegisterHandler(bot.HandlerTypeMessageText, "/clear", bot.MatchTypeExact, handleClear)
    b.RegisterHandler(bot.HandlerTypeMessageText, "/setpairs", bot.MatchTypeExact, handleSetPairs)
    b.RegisterHandler(bot.HandlerTypeMessageText, "/setfrequency", bot.MatchTypeExact, handleSetFrequency)

    go startPeriodicMessages(ctx, b)

    b.Start(ctx)
}

func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Say /upload, /clear, /setpairs, /setfrequency",
	})
}

func handleUpload(ctx context.Context, b *bot.Bot, update *models.Update) {
    pairs := strings.Split(update.Message.Text, "\n")
    for _, pair := range pairs {
        words := strings.Split(pair, ",")
        if len(words) != 2 {
            b.SendMessage(ctx, &bot.SendMessageParams{
                ChatID: update.Message.Chat.ID,
                Text:   "Invalid format. Please use 'word1,word2' format.",
            })
            return
        }
        wordPair := WordPair{UserID: update.Message.From.ID, Word1: strings.TrimSpace(words[0]), Word2: strings.TrimSpace(words[1])}
        if err := db.Create(&wordPair).Error; err != nil {
            logger.Error("failed to create word pair", "user_id", update.Message.From.ID, "error", err)
            b.SendMessage(ctx, &bot.SendMessageParams{
                ChatID: update.Message.Chat.ID,
                Text:   "Failed to upload word pair.",
            })
            return
        }
    }
    b.SendMessage(ctx, &bot.SendMessageParams{
        ChatID: update.Message.Chat.ID,
        Text:   "Word pairs uploaded successfully.",
    })
}

func handleClear(ctx context.Context, b *bot.Bot, update *models.Update) {
    db.Where("user_id = ?", update.Message.From.ID).Delete(&WordPair{})
    b.SendMessage(ctx, &bot.SendMessageParams{
        ChatID: update.Message.Chat.ID,
        Text:   "Your word pair list has been cleared.",
    })
}

func handleSetPairs(ctx context.Context, b *bot.Bot, update *models.Update) {
    pairsCount, err := strconv.Atoi(update.Message.Text)
    if err != nil || pairsCount <= 0 {
        b.SendMessage(ctx, &bot.SendMessageParams{
            ChatID: update.Message.Chat.ID,
            Text:   "Please provide a valid number of pairs to send.",
        })
        return
    }
    settings := UserSettings{UserID: update.Message.From.ID, PairsToSend: pairsCount}
    db.Where("user_id = ?", update.Message.From.ID).Assign(settings).FirstOrCreate(&settings)
    b.SendMessage(ctx, &bot.SendMessageParams{
        ChatID: update.Message.Chat.ID,
        Text:   "Number of pairs to send has been set.",
    })
}

func handleSetFrequency(ctx context.Context, b *bot.Bot, update *models.Update) {
    frequency, err := strconv.Atoi(update.Message.Text)
    if err != nil || frequency <= 0 {
        b.SendMessage(ctx, &bot.SendMessageParams{
            ChatID: update.Message.Chat.ID,
            Text:   "Please provide a valid number of reminders per day.",
        })
        return
    }
    settings := UserSettings{UserID: update.Message.From.ID, RemindersPerDay: frequency}
    db.Where("user_id = ?", update.Message.From.ID).Assign(settings).FirstOrCreate(&settings)
    b.SendMessage(ctx, &bot.SendMessageParams{
        ChatID: update.Message.Chat.ID,
        Text:   "Frequency of reminders has been set.",
    })
}

func startPeriodicMessages(ctx context.Context, b *bot.Bot) {
    for {
        time.Sleep(24 * time.Hour) // Adjust the duration based on user settings
        users := []UserSettings{}
        db.Find(&users)

        for _, user := range users {
            wordPairs := []WordPair{}
            db.Where("user_id = ?", user.UserID).Limit(user.PairsToSend).Find(&wordPairs)

            if len(wordPairs) > 0 {
                message := ""
                for _, pair := range wordPairs {
                    message += pair.Word1 + " - ||" + pair.Word2 + "||\n" // Using Telegram spoiler formatting
                }
                b.SendMessage(ctx, &bot.SendMessageParams{
                    ChatID: user.UserID,
                    Text:   message,
                })
            }
        }
    }
}