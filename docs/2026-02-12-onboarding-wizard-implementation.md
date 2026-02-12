# Onboarding Wizard Implementation Plan

**Goal:** Implement a `/start` onboarding wizard that provisions user vocabulary from `init_vocabularies`, applies new default settings, and supports transactional destructive re-initialization with explicit phrase confirmation.

**Architecture:** Add two DB-backed models (`InitVocabulary`, `OnboardingState`), a startup CSV refresh pipeline for the init table, and an onboarding coordinator in handlers with inline callbacks. Keep onboarding callback parsing isolated from existing settings callbacks. Enforce all destructive reset deletions in one transaction and preserve `game_session_statistics`.

**Tech Stack:** Go 1.25, GORM/PostgreSQL, go-telegram/bot, existing handlers/ui callback patterns, Docker multi-stage build.

---

Before starting tasks, create a dedicated branch/worktree and follow @brainstorming guidance: small, frequent commits, DRY, YAGNI.

### Task 1: Add onboarding config and model types

**Prompt:**
Implement minimal config + model scaffolding.
- Update `pkg/config/config.go`:
  - Add `OnboardingConfig` with `InitVocabularyPath string \`json:"init_vocabulary_path"\``.
  - Add `Onboarding OnboardingConfig \`json:"onboarding"\`` to `Config`.
- Update `config.example.json` with:
  - `"onboarding": {"init_vocabulary_path": "/app/vocabularies/multilang.csv"}`.
- Update `pkg/db/models.go`:
  - Add `InitVocabulary` struct with columns: `EN`, `RU`, `NL`, `ES`, `DE`, `FR`.
  - Add `OnboardingState` struct with `UserID`, `Step`, `LearningLang`, `KnownLang`, `AwaitingResetPhrase`.
  - Add proper indexes/uniqueness on `OnboardingState.UserID`.

Write the tests for the new code:
- Extend `pkg/config/config_test.go` to validate onboarding config parsing.
- Add/extend DB migration tests in `pkg/db/repository_test.go` to assert new tables are migratable.

Run the tests and make sure they pass:
- `go test ./pkg/config ./pkg/db/...`

Commit with the summary of the change as a commit message.

---

### Task 2: Register new tables in DB init

**Prompt:**
Wire tables into migration flow.
- Update `pkg/db/repository.go` `AutoMigrate(...)` call to include `InitVocabulary` and `OnboardingState`.
- Keep migration behavior backward compatible.

Write the tests for the new code:
- Extend `pkg/db/repository_test.go` to verify schema creation does not fail with new models.

Run the tests and make sure they pass:
- `go test ./pkg/db/...`

Commit with the summary of the change as a commit message.

---

### Task 3: Implement init vocabulary refresh service

**Prompt:**
Create startup CSV->DB refresh for `init_vocabularies` with strict header validation.
- Add `pkg/bot/onboarding/init_vocab.go` (new package `onboarding`):
  - Hardcoded ordered language definitions (single source of truth):
    - `en, ru, nl, es, de, fr`
    - include user labels with flags.
  - CSV loader that:
    - reads file from path
    - validates all required columns exist
    - maps rows into `db.InitVocabulary`
  - `RefreshInitVocabularyFromFile(path string) error`:
    - if path empty => no-op
    - if header invalid => return error listing missing columns
    - DB transaction: delete all `init_vocabularies`, insert parsed rows in batch.

Write the tests for the new code:
- Add `pkg/bot/onboarding/init_vocab_test.go`:
  - success path with required headers
  - missing required column returns error
  - rows with empty cells still load (eligibility filtering happens later)
  - transactional behavior (old data replaced only on success)

Run the tests and make sure they pass:
- `go test ./pkg/bot/onboarding ./pkg/db/...`

Commit with the summary of the change as a commit message.

---

### Task 4: Call refresh at startup without blocking bot launch

**Prompt:**
Wire refresh into app startup.
- Update `cmd/tg-word-reminder/main.go`:
  - after `db.InitDB(...)`, call onboarding refresh using config path.
  - on error: log structured error and continue startup.
  - do not `os.Exit` on refresh failure.

Write the tests for the new code:
- Add unit-level tests for refresh caller behavior if applicable in onboarding package.
- If `main.go` is hard to unit test, test helper function extracted into onboarding package.

Run the tests and make sure they pass:
- `go test ./pkg/bot/onboarding ./cmd/tg-word-reminder`

Commit with the summary of the change as a commit message.

---

### Task 5: Build onboarding callback schema + rendering helpers

**Prompt:**
Add onboarding-specific callback encoding/parsing and keyboards.
- Create `pkg/ui/onboarding_callback_data.go`:
  - prefix `o:`
  - actions for selecting learning language, selecting known language, confirm provisioning, cancel/back.
  - validation with max callback length and strict parsing.
- Create `pkg/ui/onboarding_view.go`:
  - render language selection keyboards from ordered language list.
  - exclude selected learning language from known-language options.
  - render confirmation text with selected labels and eligible pair count.

Write the tests for the new code:
- `pkg/ui/onboarding_callback_data_test.go` parse/build coverage.
- `pkg/ui/onboarding_view_test.go` button presence/exclusion coverage.

Run the tests and make sure they pass:
- `go test ./pkg/ui/...`

Commit with the summary of the change as a commit message.

---

### Task 6: Implement onboarding state machine service

**Prompt:**
Create DB-backed onboarding service with explicit steps.
- Add `pkg/bot/onboarding/service.go`:
  - constants for steps (`choose_learning`, `choose_known`, `confirm_import`).
  - CRUD helpers for `OnboardingState` by `user_id`.
  - `HasExistingUserData(userID)` helper checking `word_pairs`, `user_settings`, `training_sessions`, `game_sessions`.
  - `Begin(userID)` to create/reset state for new flow.
  - `SetLearningLanguage`, `SetKnownLanguage` (reject same language).
  - `CountEligiblePairs(learning, known)` from `init_vocabularies` where both cells are non-empty.

Write the tests for the new code:
- `pkg/bot/onboarding/service_test.go`:
  - state transitions
  - same-language rejection
  - eligibility counting with empty-cell skipping

Run the tests and make sure they pass:
- `go test ./pkg/bot/onboarding`

Commit with the summary of the change as a commit message.

---

### Task 7: Implement transactional reset + provisioning

**Prompt:**
Add core write operations with strict transactions.
- In `pkg/bot/onboarding/service.go` (or split files):
  - `ResetUserDataTx(userID int64) error`:
    - single DB transaction deleting from:
      - `word_pairs`
      - `user_settings`
      - `training_sessions`
      - `game_sessions`
      - `onboarding_states` for cleanup
    - do not touch `game_session_statistics`.
  - `ProvisionUserVocabularyAndDefaults(userID, learning, known) (inserted int, err error)`:
    - one transaction:
      - copy eligible `init_vocabularies` rows into `word_pairs`
      - set SRS defaults consistent with existing import path
      - create default `user_settings` with:
        - `PairsToSend=5`
        - `ReminderMorning=true`
        - `ReminderAfternoon=true`
        - `ReminderEvening=true`
      - clear onboarding state

Write the tests for the new code:
- `pkg/bot/onboarding/service_test.go`:
  - reset deletes requested tables only
  - reset rollback on induced error (no half-wiped state)
  - provisioning inserts only eligible pairs
  - provisioning creates settings defaults exactly as required

Run the tests and make sure they pass:
- `go test ./pkg/bot/onboarding ./pkg/db/...`

Commit with the summary of the change as a commit message.

---

### Task 8: Integrate `/start` onboarding and reset phrase flow

**Prompt:**
Refactor `pkg/bot/handlers/start.go` to use onboarding coordinator.
- `/start` behavior:
  - if existing user data => set onboarding state to awaiting phrase and ask for exact `RESET MY DATA`.
  - else begin wizard and send learning-language keyboard.
- Add handler support for phrase processing on plain text messages:
  - if user has awaiting reset phrase state and message text matches exactly `RESET MY DATA`:
    - run transactional reset
    - on success start wizard step 1
  - if mismatch:
    - keep awaiting state, send retry warning.
- Remove old `/start` direct settings-creation flow.

Write the tests for the new code:
- Update `pkg/bot/handlers/start_test.go`:
  - new user starts wizard (no immediate settings creation)
  - existing user gets phrase prompt
  - exact phrase triggers reset + wizard start
  - wrong phrase does not wipe data

Run the tests and make sure they pass:
- `go test ./pkg/bot/handlers -run Start`

Commit with the summary of the change as a commit message.

---

### Task 9: Add onboarding callback handler and registration

**Prompt:**
Implement callback handling for wizard steps and finalize provisioning.
- Create `pkg/bot/handlers/onboarding.go`:
  - `HandleOnboardingCallback` parsing `o:` callbacks.
  - step-aware transitions:
    - learning selected -> edit message to known-language keyboard
    - known selected -> edit message to confirmation view
    - confirm -> provision user vocabulary + default settings; send completion message.
  - robust recovery for stale/invalid state.
- Register callback route in `cmd/tg-word-reminder/main.go`:
  - `b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "o:", bot.MatchTypePrefix, handlers.HandleOnboardingCallback)`

Write the tests for the new code:
- `pkg/bot/handlers/onboarding_test.go`:
  - callback happy path through all steps
  - stale callback rejection
  - confirm with zero eligible pairs prompts reselection path

Run the tests and make sure they pass:
- `go test ./pkg/bot/handlers -run Onboarding`

Commit with the summary of the change as a commit message.

---

### Task 10: Route phrase capture in default message flow

**Prompt:**
Ensure phrase confirmation works regardless of command handler path.
- Update `pkg/bot/handlers/default.go`:
  - before generic command help, check onboarding awaiting phrase state.
  - consume exact phrase and trigger reset/start wizard.
- Ensure no conflicts with existing game/review text handling order.

Write the tests for the new code:
- Extend `pkg/bot/handlers/default_test.go` for phrase capture precedence and no regression for normal help responses.

Run the tests and make sure they pass:
- `go test ./pkg/bot/handlers -run Default`

Commit with the summary of the change as a commit message.

---

### Task 11: Update Docker runtime image and docs

**Prompt:**
Make init CSV available in container and document new behavior.
- Update `Dockerfile` final stage:
  - `COPY --from=builder /app/vocabularies /app/vocabularies`
- Update `README.md`:
  - onboarding wizard overview
  - re-initialization phrase requirement
  - onboarding config (`init_vocabulary_path`)
  - note that startup continues if refresh fails (error logged)

Write the tests for the new code:
- Add/adjust any config/readme-related tests if present.
- Manual sanity: ensure Dockerfile still builds.

Run the tests and make sure they pass:
- `go test ./...`
- `docker build -t tg-word-reminder .`

Commit with the summary of the change as a commit message.

---

### Task 12: Full regression and cleanup

**Prompt:**
Run full suite and clean up naming/docs.
- Run full tests and ensure no flaky failures.
- Verify callback payload size constraints remain within limits.
- Verify no command regressions (`/settings`, `/review`, `/game`) for onboarded users.

Write the tests for the new code:
- Add any missing regression tests discovered while running full suite.

Run the tests and make sure they pass:
- `go test ./...`

Commit with the summary of the change as a commit message.

---
