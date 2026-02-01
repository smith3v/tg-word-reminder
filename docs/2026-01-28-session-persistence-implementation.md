# Session Persistence Implementation Plan

**Goal:** Persist training and game sessions in the database so users can resume after bot restarts, and reminder ticks can safely expire active sessions.

**Architecture:** Add `training_sessions` and `game_sessions` tables keyed by `(chat_id, user_id)` with JSON arrays of pair IDs, index pointers, and activity timestamps. Update training/game session managers to read/write these rows on start/advance/end, and update reminder ticks to expire existing sessions and start fresh ones. Add a cleanup job to delete expired sessions.

**Tech Stack:** Go 1.25, GORM (PostgreSQL/SQLite for tests), standard library `time`.

---

### Task 1: Add session models + migrations

**Prompt:**
Implement the minimal code to add DB models for persisted sessions and wire them into migrations.
- Create new models in `pkg/db/models.go`:
  - `TrainingSession` with fields:
    - `ID uint` primary key
    - `ChatID int64` (indexed)
    - `UserID int64` (indexed)
    - `PairIDs` JSON array (use `gorm.io/datatypes.JSON` or `datatypes.JSONSlice[uint]` if available)
    - `CurrentIndex int`
    - `CurrentToken string`
    - `CurrentMessageID int`
    - `LastActivityAt time.Time`
    - `ExpiresAt time.Time`
    - `CreatedAt`, `UpdatedAt`
    - Add unique index on `(chat_id, user_id)`
  - `GameSession` with fields:
    - `ID uint` primary key
    - `ChatID int64`, `UserID int64` (indexed + unique together)
    - `PairIDs` JSON array (same JSON type as above)
    - `CurrentIndex int`
    - `CurrentToken string`
    - `CurrentMessageID int`
    - `ScoreCorrect int`, `ScoreAttempted int`
    - `LastActivityAt time.Time`
    - `ExpiresAt time.Time`
    - `CreatedAt`, `UpdatedAt`
- Update `pkg/db/repository.go` to include `TrainingSession` and `GameSession` in `AutoMigrate`.

Write the tests for the new code:
- Add a migration test in `pkg/db/repository_test.go` that `AutoMigrate` creates the new tables (use `Migrator().HasTable`).

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 2: Training session persistence helpers

**Prompt:**
Implement minimal DB helpers for training session persistence.
- Create `pkg/bot/training/session_store.go` with functions:
  - `LoadTrainingSession(chatID, userID int64, now time.Time) (*db.TrainingSession, error)`
  - `UpsertTrainingSession(session *db.TrainingSession) error`
  - `DeleteTrainingSession(chatID, userID int64) error`
  - `ExpireTrainingSession(chatID, userID int64, messageText string) error` (only deletes row; message edit happens in handler)
- Encode/decode `PairIDs` as JSON (e.g., `json.Marshal([]uint{...})` into `datatypes.JSON`).
- Ensure `ExpiresAt` is always `LastActivityAt.Add(24 * time.Hour)` on save.

Write the tests for the new code:
- Add `pkg/bot/training/session_store_test.go` covering:
  - Save + load roundtrip for `pair_ids` ordering.
  - Expired session returns nil when `expires_at <= now`.
  - Delete removes the row.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 3: Persist training sessions on start/advance/end

**Prompt:**
Implement minimal persistence wiring in the training session manager.
- Update `pkg/bot/training/session.go`:
  - On `StartOrRestart`, upsert a `TrainingSession` row with `pair_ids`, `current_index`, `current_token`, `current_message_id`, `last_activity_at`, `expires_at`.
  - Add a `Resume` path that can rehydrate a session into memory from the DB row (e.g., `StartFromPersisted` returning a `Session`).
  - On `Advance` and `MarkReviewed`, update the DB row with the new `current_index`, `current_token`, and `last_activity_at`.
  - On `End`, delete the DB row as well as in-memory state.
- Keep in-memory state as cache only; DB is source of truth.

Write the tests for the new code:
- Extend `pkg/bot/training/session_test.go` to assert:
  - Starting a session writes a DB row.
  - Advancing updates `current_index` and `current_token` in DB.
  - Ending deletes the DB row.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 4: Resume logic for `/review` and `/getpair`

**Prompt:**
Implement resume logic in handlers using persisted training sessions.
- Update `pkg/bot/handlers/review.go`:
  - Before selecting new pairs, attempt to load an active session from DB by `(chat_id, user_id)`.
  - If found, resume and send the current prompt (rebuild from `pair_ids[current_index]`).
  - Only start a new session when no active session exists.
- Update `pkg/bot/handlers/pairs.go` (`/getpair`) similarly:
  - Resume existing session if present; otherwise start a one-card session.

Write the tests for the new code:
- Extend `pkg/bot/handlers/review_test.go` to cover resume behavior (existing session does not get overwritten).
- Extend `pkg/bot/handlers/pairs_test.go` to cover resume behavior for `/getpair`.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 5: Reminder tick expiry + restart

**Prompt:**
Implement the reminder tick behavior to expire active sessions and start new ones.
- Update `pkg/bot/reminders/tickers.go`:
  - On reminder tick, check DB for an active training session.
  - If one exists, edit the last session message to “The session is expired.”, remove the inline keyboard, delete the session row, then start a fresh reminder session.
- Ensure this happens even if the session is not yet expired (per spec).

Write the tests for the new code:
- Extend `pkg/bot/reminders/tickers_test.go`:
  - Seed a persisted session with `current_message_id` and ensure reminder tick edits the message, removes buttons, deletes the row, and sends a new prompt.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 6: Game session persistence

**Prompt:**
Implement minimal persistence for game sessions.
- Update the game session logic in `pkg/bot/game` (and any handlers under `pkg/bot/handlers`) to:
  - Load persisted game session on `/game` start.
  - Persist progress after each answer (current index, score fields, message id, token, timestamps).
  - Delete the row on completion or expiry.

Write the tests for the new code:
- Extend existing game tests in `pkg/bot/game/*_test.go` or add `pkg/bot/handlers/game_test.go` cases to assert:
  - Resume on `/game` when a session exists.
  - Persisted score/index updates after an answer.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 7: Cleanup job for expired sessions

**Prompt:**
Implement a periodic cleanup job for expired sessions.
- Add a helper in `pkg/db` or `pkg/bot/training` to delete rows where `expires_at <= now` for both training and game sessions.
- Wire the cleanup job into an existing ticker (e.g., reminder ticker) or a new goroutine started in `cmd/tg-word-reminder/main.go`.

Write the tests for the new code:
- Add tests to verify expired rows are deleted while active ones remain.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 8: End-to-end regression and docs touch-up

**Prompt:**
Ensure behavior matches the spec and all tests pass.
- Run `go test ./...` and confirm all packages pass.
- Fix any failing tests or missing imports.
- Review `docs/2026-01-28-session-persistence.md` for alignment with the final behavior and update if needed.

Write the tests for the new code:
- (If any missing) add minimal tests to cover uncovered branches.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---
