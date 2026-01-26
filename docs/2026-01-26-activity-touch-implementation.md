# Activity Touch Batching Implementation Plan

**Goal:** Reactivate paused users on any interaction without per-update DB writes by batching touches once per minute.

**Architecture:** Add an in-memory activity tracker, a middleware to record touches on every update, and a minute-based flusher to update `training_paused` and `missed_training_sessions`. Keep `last_training_engaged_at` unchanged.

**Tech Stack:** Go 1.25, go-telegram/bot v1.8.3, GORM, PostgreSQL.

---

### Task 1: Add activity tracker and middleware wiring

**Prompt:**
Implement the minimal code to add an activity tracker and middleware, then wire it into the bot. Do the following steps:
1) Create `pkg/bot/handlers/activity.go` with a concurrency-safe `ActivityTracker` (map + mutex) that supports `Touch(userID int64)` and `Flush(ctx context.Context)` for batched DB updates. `Flush` should snapshot IDs, update `training_paused=false` and `missed_training_sessions=0` via `db.DB.Model(&db.UserSettings{}).Where("user_id IN ?", ids).Updates(...)`, and delete IDs from the map only on successful update. Ignore userID 0 in `Touch`.
2) Add `ActivityMiddleware(tracker *ActivityTracker) bot.Middleware` in the same file. It should extract user IDs from `update.Message.From.ID` and `update.CallbackQuery.From.ID`, call `tracker.Touch(userID)`, then invoke the next handler.
3) In `cmd/tg-word-reminder/main.go`, create a tracker instance, add middleware with `bot.WithMiddlewares(handlers.ActivityMiddleware(tracker))`, and start a flusher goroutine `handlers.StartActivityFlusher(ctx, tracker)` that ticks every minute.
4) Run `go fmt ./...` and ensure it exits with code 0.
5) Commit with message `Add activity tracker middleware for reactivation batching`.

**Commands:**
- `go fmt ./...`
- `git status --short`
- `git commit -am "Add activity tracker middleware for reactivation batching"`

**Expected output:**
- `go fmt` produces no output on success.
- `git status --short` shows modified/new files under `pkg/bot/handlers` and `cmd/tg-word-reminder/main.go`.
- `git commit` reports 1+ files changed.

---

### Task 2: Add activity flusher tests

**Prompt:**
Write tests for the activity tracker flushing behavior.
1) Create `pkg/bot/handlers/activity_test.go` with table-driven tests for `Touch` and `Flush`.
2) Cover: duplicate touches collapse to one update; successful flush clears IDs; failed DB update leaves IDs; invalid user ID (0) does nothing.
3) Use test DB setup consistent with existing tests (see `pkg/internal/testutil`). If no helper exists, create minimal setup in the test to swap `db.DB` with a test connection and rollback after.
4) Run `go test ./pkg/bot/handlers -run Activity` and confirm success.
5) Commit with message `Add tests for activity tracker flush`.

**Commands:**
- `go test ./pkg/bot/handlers -run Activity`
- `git status --short`
- `git commit -am "Add tests for activity tracker flush"`

**Expected output:**
- `go test` prints `ok` for `./pkg/bot/handlers`.
- `git commit` reports updated test file(s).

---

### Task 3: Run full test pass (optional but recommended)

**Prompt:**
Run the full test suite to ensure no regressions.
1) Execute `go test ./...`.
2) If failures occur, fix them and re-run.
3) Commit any fixes with a concise message describing the fix.

**Commands:**
- `go test ./...`

**Expected output:**
- `go test` prints `ok` for all packages.

---
