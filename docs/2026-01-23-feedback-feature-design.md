# Feedback Command Design

## Goal
Add a Telegram-native way for users to send feedback that is delivered to a configured list of admin user IDs. The flow must be simple (single message), private (private chats only), and resilient (log the feedback text before delivery so admins can recover if delivery fails).

## Non-Goals
- Public feedback channels, group feedback collection, or threaded discussions.
- Multi-message feedback sessions.
- Persistent state across bot restarts.
- Moderation, spam filtering, or analytics.

## User Experience
- A user sends `/feedback` in a private chat with the bot.
- The bot replies: "Please send your feedback within 5 minutes (attachments are ok)."
- The next single message from that user (text and/or any Telegram-supported media) is captured as feedback.
- The user receives a confirmation: "Thanks for your feedback!"
- If no message arrives within 5 minutes, the pending state expires silently; the user can run `/feedback` again.

Notes:
- `/feedback` is accepted only in private chat.
- The next message counts as feedback even if it is a command. This avoids ambiguity.

## Configuration
Add a new block to `config.json` and `config.example.json`:

```json
"feedback": {
  "enabled": true,
  "admin_ids": [123456789, 987654321],
  "timeout_minutes": 5
}
```

- `admin_ids` are numeric Telegram user IDs only.
- If `admin_ids` is empty or missing, `/feedback` returns a friendly error and logs the misconfiguration.
- README update: mention using a "get my ID" bot to discover numeric Telegram IDs.

## Data Model (In-Memory)
Maintain an in-memory map keyed by user ID:

```
PendingFeedback {
  ChatID     int64
  ExpiresAt  time.Time
}
```

Behavior:
- Create entry on `/feedback`.
- Overwrite existing entry on `/feedback` (refreshes expiry).
- Clear entry when feedback is captured or expired.
- Entries are not persisted and are lost on restart.

Expiration:
- 5-minute timeout as configured.
- Implement a small sweeper (e.g., ticker every 1 minute) or check-and-expire opportunistically on message receipt.

## Delivery to Admins
For each admin in `admin_ids`, deliver feedback in two messages:

1) **Bot-crafted summary message** (visually distinct):
   - Include user display name, username (if present), user ID, and timestamp (UTC).
   - Include a quote-style block with the feedback text or caption.
   - If no text/caption, show "(no text; attachment only)".
   - Plain text is acceptable; a simple prefix like `>` for quoted lines is sufficient.

2) **Forwarded original message**:
   - Use Telegram's native forward so admins see the original context and can reply.
   - Forward exactly the user message that was captured.

Delivery order:
- Log feedback text/caption first (see Logging).
- Send summary message.
- Forward original message.

If forwarding is blocked by user privacy, Telegram will omit user details in the forward; the summary message still includes user identity.

## Logging (Backup)
Before sending to admins, log the feedback text/caption with metadata:
- `user_id`, `username`, `display_name`
- `chat_id`, `message_id`
- `timestamp_utc`
- `has_media` (bool) and media type (if available)

Do not log binary media content. Logging is the fallback trail if admin delivery fails or is misconfigured.

## Error Handling
- If admin delivery fails for one admin, continue sending to others and log the error.
- If all admin deliveries fail, still confirm to the user (the feedback was captured and logged).
- If `/feedback` is used outside private chat, respond with a short error message explaining it works only in private chat.
- If the feedback state has expired, treat the next message normally.

## Security and Abuse Notes
- No special rate limiting is introduced (out of scope). If needed later, add per-user cooldown.
- Feedback content is not shown to other users.

## Acceptance Criteria
- `/feedback` works only in private chats and captures exactly one message within 5 minutes.
- All message types (text + any media) are accepted and forwarded.
- Admins receive a bot-crafted summary and the forwarded original message.
- Feedback text/caption is logged before admin delivery.
- Misconfigured admin list results in a user-facing error and a log entry.

## Testing Plan
- Unit tests for pending state (create, refresh, expire, capture).
- Handler tests:
  - `/feedback` in private chat creates pending state and responds with instructions.
  - A follow-up message is captured, logged, and delivered; user gets confirmation.
  - `/feedback` in non-private chat returns the correct error.
- Configuration parsing tests for the new feedback settings.

