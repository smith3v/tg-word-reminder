# Feedback Command Implementation Plan

**Goal:** Implement a `/feedback` command that captures a single message (text or any media) in private chat and delivers it to configured admin user IDs, with logging as a fallback.

**Architecture:** Add feedback configuration (admin IDs + timeout), an in-memory pending-feedback manager with expiration, a `/feedback` handler, and a shared capture helper that intercepts the next user message before other handlers. Deliver feedback to admins as a bot-crafted summary plus a native forward, and log the text/caption first.

**Tech Stack:** Go 1.25, go-telegram/bot, in-memory state with mutex, existing logger/config patterns.

---

Before starting tasks, create a dedicated worktree/branch (per @brainstorming guidance) and work there. Use small, frequent commits.

### Task 1: Add feedback configuration + docs

**Prompt:**
Implement the minimal configuration and documentation updates.
- Update `pkg/config/config.go`:
  - Add `FeedbackConfig` struct with fields `Enabled bool`, `AdminIDs []int64`, `TimeoutMinutes int`.
  - Add `Feedback FeedbackConfig \`json:"feedback"\`` to `Config`.
- Update `config.example.json` to include:
  - `"feedback": {"enabled": true, "admin_ids": [123456789], "timeout_minutes": 5}`
- Update `README.md` to document `/feedback` usage (private chat only), admin ID configuration, and add a short note: “Use a ‘get my ID’ bot to find your numeric Telegram ID.”

Write the tests for the new code:
- Extend `pkg/config/config_test.go` to parse a config JSON containing `feedback` and assert the fields load correctly.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 2: Implement in-memory feedback manager

**Prompt:**
Add a small, thread-safe in-memory manager to track pending feedback.
- Create `pkg/bot/feedback/manager.go` with:
  - `PendingFeedback` struct: `ChatID int64`, `ExpiresAt time.Time`.
  - `Manager` struct with mutex + `map[int64]PendingFeedback` keyed by user ID.
  - Methods:
    - `Start(userID, chatID int64, now time.Time, timeout time.Duration)` to insert/overwrite pending state.
    - `Consume(userID, chatID int64, now time.Time) bool` to return true and delete if pending + not expired, otherwise false (and delete if expired).
    - `SweepExpired(now time.Time)` to remove stale entries.
  - `DefaultManager` and `ResetDefaultManager(now func() time.Time)` consistent with existing patterns.
  - Optional `StartSweeper(ctx)` with a small ticker (e.g., 1 minute) to call `SweepExpired`.

Write the tests for the new code:
- Add `pkg/bot/feedback/manager_test.go` covering:
  - Start/overwrite resets expiration.
  - Consume succeeds before expiration and clears entry.
  - Consume fails after expiration and clears entry.
  - SweepExpired removes only stale entries.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 3: Add `/feedback` command handler

**Prompt:**
Implement the `/feedback` entrypoint that creates pending state.
- Create `pkg/bot/handlers/feedback.go` with `HandleFeedback(ctx, b, update)`:
  - Validate `update.Message`, `From`, and `Chat.ID`.
  - Require `update.Message.Chat.Type == models.ChatTypePrivate` or return a short error message.
  - Check `config.AppConfig.Feedback.Enabled` and that `AdminIDs` is non-empty; if not, log an error and reply with a friendly configuration error.
  - Call `feedback.DefaultManager.Start(userID, chatID, time.Now().UTC(), timeout)` using `TimeoutMinutes` (fallback to 5 if zero).
  - Reply: “Please send your feedback within 5 minutes (attachments are ok).”

Write the tests for the new code:
- Add `pkg/bot/handlers/feedback_test.go` with cases:
  - Private chat `/feedback` creates pending state and sends the instruction message.
  - Non-private chat returns the “private chat only” response.
  - Missing admin IDs returns a friendly error and does not create pending state.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 4: Capture next message, log, and deliver to admins

**Prompt:**
Implement capture + delivery logic and wire it into message handlers.
- In `pkg/bot/handlers/feedback.go` (or a new `pkg/bot/handlers/feedback_capture.go`), add `tryHandleFeedbackCapture(ctx, b, update) bool`:
  - Validate `update.Message` and `From`.
  - If there is no pending feedback for this user+chat, return false.
  - If pending and not expired:
    - Extract `text := update.Message.Text`, or `Caption` if text is empty.
    - Determine `hasMedia` by checking any non-nil media fields (Document/Photo/Audio/Video/Voice/Sticker/etc.).
    - **Log** the feedback text/caption and metadata via `logger.Info` before sending to admins.
    - For each admin ID in config:
      - Send a **summary** message built by a helper `formatFeedbackSummary(user, text, hasMedia, timestampUTC)`; use plain text (no Markdown) with a visible header and quoted lines prefixed by `> `.
      - Forward the original message via `b.ForwardMessage`.
      - Log errors but continue to the next admin.
    - Send confirmation to the user: “Thanks for your feedback!”
    - Return true to stop other handlers.

- Wire capture into **all message handlers** at the top:
  - `pkg/bot/handlers/start.go`, `settings.go`, `pairs.go`, `review.go`, `game.go`, `export.go`, and `default.go`.
  - Call `if tryHandleFeedbackCapture(...){ return }` before any other logic.
  - Do **not** call capture inside `HandleFeedback` to avoid consuming the `/feedback` message itself.

Write the tests for the new code:
- Extend `pkg/bot/handlers/feedback_test.go` to assert:
  - A follow-up message after `/feedback` is captured and the user gets confirmation.
  - Admin receives **two** deliveries (summary + forward) per admin ID.
  - The pending state is cleared after capture.
- Use the existing `mockClient` in `pkg/bot/handlers/test_helpers.go` to inspect outgoing bot calls (path/body checks).

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 5: Wire the sweeper + register the new handler

**Prompt:**
Finish wiring for runtime behavior.
- In `cmd/tg-word-reminder/main.go`:
  - Register the `/feedback` handler: `b.RegisterHandler(bot.HandlerTypeMessageText, "/feedback", bot.MatchTypeExact, handlers.HandleFeedback)`.
  - Start the feedback sweeper if you implemented one: `go feedback.StartSweeper(ctx)`.

Write the tests for the new code:
- If you added `feedback.StartSweeper`, add a small unit test in `pkg/bot/feedback/manager_test.go` to validate it expires entries (or skip if covered by `SweepExpired`).

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---
