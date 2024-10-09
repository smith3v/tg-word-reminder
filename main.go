// main.go
package main

import (
    "encoding/json"
    "os"
    "strconv"
    "strings"
    "time"
	"log/slog"

    "github.com/go-telegram/bot"
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
    logger = slog.New(slog.NewTextHandler(os.Stdout))
    loadConfig()
    initDB()
    bot, err := bot.New(config.Telegram.Token)
    if err != nil {
        logger.Fatalf("failed to create bot: %v", err)
    }

    bot.Handle("/upload", handleUpload)
    bot.Handle("/clear", handleClear)
    bot.Handle("/setpairs", handleSetPairs)
    bot.Handle("/setfrequency", handleSetFrequency)

    go startPeriodicMessages(bot)

    bot.Start()
}

func handleUpload(c *bot.Context) {
    pairs := strings.Split(c.Message.Text, "\n")
    for _, pair := range pairs {
        words := strings.Split(pair, ",")
        if len(words) != 2 {
            c.Reply("Invalid format. Please use 'word1,word2' format.")
            return
        }
        wordPair := WordPair{UserID: c.Message.From.ID, Word1: strings.TrimSpace(words[0]), Word2: strings.TrimSpace(words[1])}
        if err := db.Create(&wordPair).Error; err != nil {
            logger.Error("failed to create word pair", "user_id", c.Message.From.ID, "error", err)
            c.Reply("Failed to upload word pair.")
            return
        }
    }
    c.Reply("Word pairs uploaded successfully.")
}

func handleClear(c *bot.Context) {
    db.Where("user_id = ?", c.Message.From.ID).Delete(&WordPair{})
    c.Reply("Your word pair list has been cleared.")
}

func handleSetPairs(c *bot.Context) {
    pairsCount, err := strconv.Atoi(c.Message.Text)
    if err != nil || pairsCount <= 0 {
        c.Reply("Please provide a valid number of pairs to send.")
        return
    }
    settings := UserSettings{UserID: c.Message.From.ID, PairsToSend: pairsCount}
    db.Where("user_id = ?", c.Message.From.ID).Assign(settings).FirstOrCreate(&settings)
    c.Reply("Number of pairs to send has been set.")
}

func handleSetFrequency(c *bot.Context) {
    frequency, err := strconv.Atoi(c.Message.Text)
    if err != nil || frequency <= 0 {
        c.Reply("Please provide a valid number of reminders per day.")
        return
    }
    settings := UserSettings{UserID: c.Message.From.ID, RemindersPerDay: frequency}
    db.Where("user_id = ?", c.Message.From.ID).Assign(settings).FirstOrCreate(&settings)
    c.Reply("Frequency of reminders has been set.")
}

func startPeriodicMessages(bot *bot.Bot) {
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
                bot.Send(user.UserID, message)
            }
        }
    }
}