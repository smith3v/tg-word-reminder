# Spaced Repetition Training Implementation Plan

**Goal:** Implement the SRS-based training sessions, settings, and reminder scheduling described in `docs/plans/2026-01-15-spaced-repetition.md`.

**Architecture:** Add SRS fields to `word_pairs` and new reminder state to `user_settings`, introduce a training scheduler (card selection + grading transitions), and replace the current reminder ticker with fixed daily slots plus inactivity handling. Training sessions are in-memory and are started via `/review` or scheduled slots.

**Tech Stack:** Go 1.25, GORM (PostgreSQL), go-telegram/bot, standard library `time`.

---

Before starting tasks, create a dedicated worktree/branch (per @brainstorming guidance) and work there. Use frequent, small commits.

### Task 1: Add SRS and reminder state fields + migration for reminders_per_day (no drop)

**Prompt:**
Implement the minimal code to add new SRS fields and reminder state, plus the `reminders_per_day` mapping migration. Keep the legacy `reminders_per_day` column during development (do not drop it in code).
- Update `pkg/db/models.go`:
  - In `WordPair`, add fields: `SrsState string`, `SrsDueAt time.Time`, `SrsLastReviewedAt *time.Time`, `SrsIntervalDays int`, `SrsEase float64`, `SrsStep int`, `SrsReps int`, `SrsLapses int`.
  - Add GORM index tag for `UserID` + `SrsDueAt` (e.g., `gorm:"index:idx_user_due"` on both fields).
  - In `UserSettings`, remove `RemindersPerDay` and add: `ReminderMorning bool`, `ReminderAfternoon bool`, `ReminderEvening bool`, `TimezoneOffsetHours int`, `MissedTrainingSessions int`, `TrainingPaused bool`, `LastTrainingSentAt *time.Time`, `LastTrainingEngagedAt *time.Time`.
- Update `pkg/db/repository.go`:
  - After `AutoMigrate`, add a migration helper that:
    - If column `reminders_per_day` exists, read it and map to the new booleans:
      - 0 -> all false
      - 1 -> evening only
      - 2 -> morning + evening
      - >2 -> morning + afternoon + evening
    - Use `DB.Migrator().HasColumn` and `DB.Exec` with a SQL `UPDATE user_settings SET ... CASE ...`.
  - Do not drop `reminders_per_day` in code; the column will be removed manually after deployment.
- Set sensible defaults via struct tags (e.g., `SrsState` default "new", `SrsEase` default 2.5, `SrsStep` default 0).

Write the tests for the new code:
- Add `pkg/db/repository_test.go` (or extend existing tests) to validate the migration mapping using a temporary test DB from `pkg/internal/testutil`.
- Assert the mapping for 0,1,2,3 values and that the new boolean fields are set as expected.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 2: Initialize SRS defaults on import and new users

**Prompt:**
Implement minimal code to set SRS defaults for imported pairs and new user settings.
- Update `pkg/bot/importexport/csv.go` in `UpsertWordPairs`:
  - On create, set SRS defaults:
    - `SrsState = "new"`, `SrsDueAt = time.Now().UTC()`, `SrsIntervalDays = 0`, `SrsEase = 2.5`, `SrsStep = 0`.
- Update `pkg/bot/handlers/start.go`:
  - Set defaults for new users:
    - `PairsToSend = 5`
    - `ReminderEvening = true` (others false)
    - `TimezoneOffsetHours = 0`
    - `MissedTrainingSessions = 0`, `TrainingPaused = false`

Write the tests for the new code:
- Extend `pkg/bot/importexport/import_test.go` to assert default SRS fields for new inserts.
- Extend `pkg/bot/handlers/start_test.go` to assert the new defaults are set on first `/start`.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 3: Implement SRS scheduling core (selection + grading)

**Prompt:**
Implement the minimal SRS scheduling core in a new package.
- Create `pkg/bot/training/scheduler.go` with:
  - Types `SrsState`, `Grade`, and constants for steps (10m, 1d) and ease floor.
  - `SelectSessionPairs(userID, size int, now time.Time)` that:
    - Loads due cards ordered by `srs_due_at`, then fills with new cards by `id`.
  - `ApplyGrade(pair *db.WordPair, grade Grade, now time.Time)` that:
    - Implements the learning/review rules from the spec (Again/Hard/Good/Easy).
    - Updates `SrsDueAt`, `SrsState`, `SrsStep`, `SrsIntervalDays`, `SrsEase`, `SrsReps`, `SrsLapses`, `SrsLastReviewedAt`.

Write the tests for the new code:
- New tests in `pkg/bot/training/scheduler_test.go`:
  - Learning transitions for all grades.
  - Review transitions for all grades, including ease floor.
  - Selection order (due first, then new).

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 4: Training session manager and `/review` handler

**Prompt:**
Implement minimal in-memory sessions and the `/review` command.
- Add `pkg/bot/training/session.go` with:
  - Session struct keyed by `(chatID, userID)` storing queue of pair IDs, current token, current message ID.
  - Functions to start a session and resolve a grade callback.
- Add `pkg/bot/handlers/review.go` with:
  - `/review` handler: loads session pairs via scheduler, sends first prompt.
  - Callback handler for `t:grade:<token>:<grade>`: applies grade, updates DB, sends next prompt or end message.
- Update `cmd/tg-word-reminder/main.go` to register `/review` and the callback prefix.
- Prompt format: `Shown -> ||Expected||` with inline buttons `Again/Hard/Good/Easy`.

Write the tests for the new code:
- Add `pkg/bot/handlers/review_test.go` to validate:
  - `/review` when no cards -> "Nothing to review right now".
  - Grade callback updates DB fields and sends next prompt.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 5: Update settings UI for reminder slots + timezone

**Prompt:**
Implement minimal settings UI changes for Morning/Afternoon/Evening toggles and timezone offset selection.
- Update `pkg/ui/callback_data.go`:
  - Add callback data for toggling slots and for timezone selection (UTC offsets -12 to +14).
- Update `pkg/ui/settings_view.go`:
  - Render new settings screens (Home with slot states, Reminder Slots screen, Timezone screen).
- Update `pkg/bot/handlers/settings.go`:
  - Apply new actions, persist booleans and timezone.

Write the tests for the new code:
- Extend `pkg/ui/*_test.go` for callback parsing and rendering.
- Extend `pkg/bot/handlers/settings_test.go` to assert slot toggles and timezone changes.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 6: Replace reminders ticker with fixed slots + inactivity pause

**Prompt:**
Implement fixed-time reminder scheduling and inactivity pause logic.
- Replace logic in `pkg/bot/reminders/tickers.go` with a slot-based scheduler:
  - Compute next reminder time for each enabled slot based on `timezone_offset_hours`.
  - Only schedule when `training_paused == false` and slots enabled.
  - Store `last_training_sent_at` when a session is sent.
  - If no engagement before next slot, increment `missed_training_sessions`.
  - When `missed_training_sessions >= 3`, set `training_paused = true` and optionally send a one-time pause notice.
- On any engagement (grade tap or `/review`), reset `missed_training_sessions` and clear `training_paused`.

Write the tests for the new code:
- Add `pkg/bot/reminders/tickers_test.go` cases for:
  - Next-slot calculation for all 3 slots.
  - Missed session increment and pause after 3.
  - Reset on engagement.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 7: Overdue choice flow (catch-up / snooze)

**Prompt:**
Implement the overdue choice prompt and snooze behavior.
- Add a callback prefix for overdue actions (e.g., `t:overdue:<token>:catch|snooze1d|snooze1w`).
- In reminders:
  - When overdue count exceeds session size or daily capacity, send a choice message instead of a session.
  - On snooze, shift all due cards by +1d or +1w using a DB `UPDATE`.
  - On catch-up, immediately start a session.

Write the tests for the new code:
- Add unit tests verifying the overdue condition and snooze updates.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 8: Integrate engagement tracking into training callbacks

**Prompt:**
Implement minimal engagement tracking for grade taps and `/review`.
- In grade callback handler, set `last_training_engaged_at = now`, reset `missed_training_sessions` to 0, and clear `training_paused`.
- In `/review` handler, do the same when a session starts.

Write the tests for the new code:
- Extend `pkg/bot/handlers/review_test.go` to assert engagement fields are updated.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---

### Task 9: End-to-end regression tests and cleanup

**Prompt:**
Ensure all tests pass and behavior matches the spec.
- Run `go test ./...` and confirm all packages pass.
- Fix any failing tests or missing imports.
- Verify reminder slots and timezone calculations manually if needed.

Write the tests for the new code:
- (If any missing) add minimal tests to cover uncovered branches.

Run the tests and make sure they pass:
- `go test ./...` (expect all `ok`).

Commit with the summary of the change as a commit message.

---
