package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/bot/feedback"
	"github.com/smith3v/tg-word-reminder/pkg/config"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

const defaultFeedbackTimeoutMinutes = 5

func HandleFeedback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		logger.Error("invalid update in HandleFeedback")
		return
	}

	if update.Message.Chat.Type != models.ChatTypePrivate {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "The /feedback command works only in private chat.",
		})
		return
	}

	cfg := config.AppConfig.Feedback
	if !cfg.Enabled || len(cfg.AdminIDs) == 0 {
		logger.Error("feedback is not configured", "user_id", update.Message.From.ID)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Feedback is not configured yet. Please try again later.",
		})
		return
	}

	timeoutMinutes := cfg.TimeoutMinutes
	if timeoutMinutes <= 0 {
		timeoutMinutes = defaultFeedbackTimeoutMinutes
	}
	timeout := time.Duration(timeoutMinutes) * time.Minute

	feedback.DefaultManager.Start(update.Message.From.ID, update.Message.Chat.ID, time.Now().UTC(), timeout)

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("Please send your feedback within %d minutes (attachments are ok).", timeoutMinutes),
	})
	if err != nil {
		logger.Error("failed to send feedback prompt", "user_id", update.Message.From.ID, "error", err)
	}
}

func tryHandleFeedbackCapture(ctx context.Context, b *bot.Bot, update *models.Update) bool {
	if update == nil || update.Message == nil || update.Message.From == nil || update.Message.Chat.ID == 0 {
		return false
	}
	msg := update.Message
	user := msg.From
	now := time.Now().UTC()
	if !feedback.DefaultManager.Consume(user.ID, msg.Chat.ID, now) {
		return false
	}

	text := msg.Text
	if text == "" {
		text = msg.Caption
	}

	hasMedia, mediaType := detectMedia(msg)
	displayName := formatDisplayName(user)
	logger.Info(
		"feedback received",
		"user_id", user.ID,
		"username", user.Username,
		"display_name", displayName,
		"chat_id", msg.Chat.ID,
		"message_id", msg.ID,
		"timestamp_utc", now.Format(time.RFC3339),
		"has_media", hasMedia,
		"media_type", mediaType,
		"text", text,
	)

	cfg := config.AppConfig.Feedback
	if !cfg.Enabled || len(cfg.AdminIDs) == 0 {
		logger.Error("feedback is not configured", "user_id", user.ID)
	} else {
		summary := formatFeedbackSummary(user, hasMedia, mediaType, now)
		for _, adminID := range cfg.AdminIDs {
			if adminID == 0 {
				continue
			}
			if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: adminID,
				Text:   summary,
			}); err != nil {
				logger.Error("failed to send feedback summary", "admin_id", adminID, "error", err)
			}
			if _, err := b.ForwardMessage(ctx, &bot.ForwardMessageParams{
				ChatID:     adminID,
				FromChatID: msg.Chat.ID,
				MessageID:  msg.ID,
			}); err != nil {
				logger.Error("failed to forward feedback", "admin_id", adminID, "error", err)
			}
		}
	}

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   "Thanks for your feedback!",
	}); err != nil {
		logger.Error("failed to send feedback confirmation", "user_id", user.ID, "error", err)
	}

	return true
}

func formatFeedbackSummary(user *models.User, hasMedia bool, mediaType string, now time.Time) string {
	name := formatDisplayName(user)
	meta := fmt.Sprintf("From: %s (ID %d)", name, user.ID)
	mediaLine := "Media: none"
	if hasMedia {
		if strings.TrimSpace(mediaType) == "" {
			mediaLine = "Media: attached"
		} else {
			mediaLine = fmt.Sprintf("Media: %s", mediaType)
		}
	}
	return fmt.Sprintf(
		"Feedback received\n%s\nAt: %s UTC\n%s",
		meta,
		now.Format("2006-01-02 15:04"),
		mediaLine,
	)
}

func formatDisplayName(user *models.User) string {
	if user == nil {
		return "Unknown user"
	}
	name := strings.TrimSpace(strings.TrimSpace(user.FirstName + " " + user.LastName))
	if name != "" && user.Username != "" {
		return fmt.Sprintf("%s (@%s)", name, user.Username)
	}
	if name != "" {
		return name
	}
	if user.Username != "" {
		return user.Username
	}
	return fmt.Sprintf("User %d", user.ID)
}

func detectMedia(msg *models.Message) (bool, string) {
	if msg == nil {
		return false, ""
	}
	switch {
	case msg.Document != nil:
		return true, "document"
	case len(msg.Photo) > 0:
		return true, "photo"
	case msg.Video != nil:
		return true, "video"
	case msg.Audio != nil:
		return true, "audio"
	case msg.Voice != nil:
		return true, "voice"
	case msg.VideoNote != nil:
		return true, "video_note"
	case msg.Sticker != nil:
		return true, "sticker"
	case msg.Animation != nil:
		return true, "animation"
	case msg.Contact != nil:
		return true, "contact"
	case msg.Location != nil:
		return true, "location"
	case msg.Venue != nil:
		return true, "venue"
	case msg.Poll != nil:
		return true, "poll"
	case msg.Dice != nil:
		return true, "dice"
	case msg.Game != nil:
		return true, "game"
	case msg.PaidMedia != nil:
		return true, "paid_media"
	case msg.Invoice != nil:
		return true, "invoice"
	case msg.SuccessfulPayment != nil:
		return true, "payment"
	case msg.RefundedPayment != nil:
		return true, "refunded_payment"
	case msg.Story != nil:
		return true, "story"
	}
	if msg.Caption != "" && msg.Text == "" {
		return true, "caption"
	}
	return false, ""
}
