# Onboarding Wizard Design

## Goal
Add a `/start` onboarding wizard that initializes a user vocabulary from a predefined multilingual base vocabulary, sets default training settings, and supports full account re-initialization with explicit destructive confirmation.

## Decisions
- `/start` always runs onboarding logic.
- Existing users must type exact phrase `RESET MY DATA` before destructive re-initialization.
- Existing users can cancel re-initialization with an inline `Keep my data` button.
- If `init_vocabularies` is empty/unavailable, onboarding wizard does not start.
- Destructive reset deletes: `word_pairs`, `user_settings`, `training_sessions`, `game_sessions`.
- Destructive reset does **not** delete `game_session_statistics`.
- Data wipe must run in one DB transaction; rollback on any error.
- Onboarding state is persisted in DB.
- Init vocabulary storage uses a wide table model: `db.InitVocabulary` (`en`, `ru`, `nl`, `es`, `de`, `fr`).
- Same language pair (e.g. `en -> en`) is not allowed.
- Language set is fixed to `en, ru, nl, es, de, fr`; labels include flag + language name.
- Display order is configurable in code via one ordered list.
- Init vocabulary refresh is strict for required columns, but startup must continue on refresh failure (error logged).

## Non-Goals
- Dynamic language discovery from CSV headers.
- Support for same-language decks.
- Deleting historical game statistics during re-initialization.

## Data Model

### `init_vocabularies` (`db.InitVocabulary`)
A wide table where each row represents one multilingual lexical item.

Columns:
- `id` (PK)
- `en`, `ru`, `nl`, `es`, `de`, `fr` (string)
- optional timestamps (`created_at`, `updated_at`) depending on model style

Semantics:
- Empty value in a selected source/target language means the row is not eligible for that requested pair.

### `onboarding_states` (`db.OnboardingState`)
Persists wizard progress and re-init confirmation flow.

Columns:
- `id` (PK)
- `user_id` (unique index)
- `step` (string enum-like)
- `learning_lang` (string, nullable/empty)
- `known_lang` (string, nullable/empty)
- `awaiting_reset_phrase` (bool)
- timestamps

## Configuration
Add onboarding config section:

```json
"onboarding": {
  "init_vocabulary_path": "/app/vocabularies/multilang.csv"
}
```

Behavior:
- If `init_vocabulary_path` is empty: skip refresh.
- If set: attempt refresh on startup.
- Required columns are hardcoded source-of-truth: `en`, `ru`, `nl`, `es`, `de`, `fr`.
- Missing required columns => refresh fails, error logged, bot continues startup.

## Startup Refresh Flow
1. Load CSV from configured path.
2. Parse header and validate required columns.
3. If validation fails: log missing columns and return error to caller.
4. In one DB transaction:
- delete existing `init_vocabularies` rows
- parse CSV rows and map fields by required header names
- insert rows in batches
5. Log summary (`loaded`, `skipped`, duration).

Runtime startup behavior:
- Main startup should call refresh and log failures.
- Refresh failure must not terminate process.

## User Experience

### New user (`/start`)
0. If built-in init vocabulary is unavailable:
- do not enter onboarding wizard
- inform user to upload their own CSV vocabulary file
- ensure default settings exist so `/settings` remains usable
1. Start wizard.
2. Choose learning language (maps to personal `word1`).
3. Choose known language (maps to personal `word2`; chosen learning language excluded).
4. Show confirmation summary: selected languages and eligible row count.
5. On confirm:
- copy eligible pairs from `init_vocabularies` to `word_pairs`
- initialize SRS fields same as current import path
- create default settings:
  - `PairsToSend = 5`
  - `ReminderMorning = true`
  - `ReminderAfternoon = true`
  - `ReminderEvening = true`
  - other fields default/zero
- clear onboarding state
- send completion message with settings summary and next commands

### Existing user (`/start`)
0. If built-in init vocabulary is unavailable:
- do not enter reset confirmation state
- keep existing data unchanged and inform user to try again later
1. Bot warns that re-initialization wipes training data.
2. Bot asks to type exact phrase `RESET MY DATA` and shows an inline `Keep my data` button.
3. If user taps `Keep my data`: clear reset-pending state, keep all existing data, and end the reset flow.
4. If phrase mismatches: keep awaiting phrase and send retry message.
5. If phrase matches:
- run destructive wipe in one transaction
- on success, start wizard from step 1
- on failure, rollback and inform user; do not start wizard

## Copy Rules: Init -> Personal Vocabulary
For selected `(learning_lang, known_lang)`:
- `word1 = init.<learning_lang>`
- `word2 = init.<known_lang>`
- skip rows where either side is empty after trim
- insert as new user rows with initialized SRS state (`new`, due now, rank, etc.)

## Transaction Boundaries
- Destructive re-init wipe: one transaction; all-or-nothing.
- Final provisioning (copy pairs + create settings + clear onboarding state): one transaction recommended for consistency.

## Callback and State Handling
- Use separate callback prefix for onboarding (e.g. `o:`), independent from settings (`s:`).
- Include a dedicated onboarding callback action for canceling reset flow (`cancel_reset`).
- Maintain language metadata in one ordered list:
- DB column key (`en`, `ru`, ...)
- display label (`🇬🇧 English`, etc.)
- this list controls both validation and button order

## Container Packaging
`vocabularies/multilang.csv` must exist in runtime image.

Docker update:
- copy `vocabularies/` into final runtime stage (not just builder stage)

## Error Handling
- Invalid callback/state mismatch: show safe recovery message and offer `/start` to restart flow.
- Reset cancellation callback should only work when reset confirmation is pending; otherwise return a no-op/notice.
- If init vocabulary becomes unavailable during onboarding callbacks, stop wizard flow, clear onboarding state, and show upload-CSV fallback guidance.
- No eligible pairs for selected language pair: show message and let user pick different known language.
- Config/path issues during refresh: log error and continue startup.
- DB failure in wipe/provisioning transactions: rollback and inform user.

## Testing Plan
- Model/migration tests for new tables.
- Refresh loader tests:
- strict required-column validation
- successful reload path
- skip empty rows behavior
- onboarding handler tests:
- new user wizard path
- existing user reset phrase required
- existing user reset flow can be canceled with `Keep my data`
- new user fallback path when init vocabulary is unavailable
- existing user reset flow blocked when init vocabulary is unavailable
- wrong phrase rejected
- successful phrase triggers full transactional wipe
- final provisioning creates expected pairs/settings
- ensure game statistics table remains untouched by reset.

## Acceptance Criteria
- `/start` on new users launches wizard and provisions vocabulary + defaults.
- `/start` on existing users requires exact `RESET MY DATA` before wipe.
- `/start` on existing users offers a `Keep my data` cancel action that exits reset flow without data changes.
- If init vocabulary is unavailable, `/start` provides a non-blocking fallback message instead of entering/resetting onboarding.
- Wipe is transactional and never partial.
- Game statistics are preserved.
- Init vocabulary comes from `db.InitVocabulary` table populated from configured CSV path.
- Startup continues when refresh fails, with clear error logs.
