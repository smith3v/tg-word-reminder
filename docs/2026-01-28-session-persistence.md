# Session Persistence Design

## Goal
Persist in-progress training and game sessions so users can resume after bot restarts.

## Scope
- Training sessions: `/review`, `/getpair`, and scheduled reminders.
- Game sessions: `/game`.
- One active session per `(chat_id, user_id)`.

## Architecture
- Add DB-backed session storage; in-memory managers become caches.
- Resume existing sessions on handler entry.
- Reminder ticks always expire any active training session for the user, then start a new reminder session.
- Sessions expire after 24 hours of inactivity.

## Data Model

### training_sessions
One row per active training session.

- `chat_id` (int64)
  - Telegram chat ID. Part of the session key.
- `user_id` (int64)
  - Telegram user ID. Part of the session key.
- `pair_ids` (json array of uint)
  - Ordered queue of word-pair IDs.
  - Used to resume the exact sequence after restart.
- `current_index` (int)
  - 0-based index into `pair_ids` for the active card.
- `current_token` (string)
  - Current callback token used to validate grading actions.
- `current_message_id` (int)
  - Message ID of the currently displayed prompt.
- `last_activity_at` (timestamp)
  - Updated on start and each grade; used for expiry.
- `expires_at` (timestamp)
  - `last_activity_at + 24h`.
- `created_at`, `updated_at` (timestamps)
  - Standard auditing fields.

Uniqueness:
- Unique index on `(chat_id, user_id)`.

### game_sessions
One row per active game session.

- `chat_id` (int64)
  - Telegram chat ID. Part of the session key.
- `user_id` (int64)
  - Telegram user ID. Part of the session key.
- `pair_ids` (json array of uint)
  - Ordered queue of word-pair IDs for the game.
- `current_index` (int)
  - 0-based index into `pair_ids` for the active question.
- `current_token` (string)
  - Current callback token used to validate answers.
- `current_message_id` (int)
  - Message ID of the currently displayed prompt.
- `score_correct` (int)
  - Running correct answer count.
- `score_attempted` (int)
  - Running attempt count.
- `last_activity_at` (timestamp)
  - Updated on start and each answer; used for expiry.
- `expires_at` (timestamp)
  - `last_activity_at + 24h`.
- `created_at`, `updated_at` (timestamps)
  - Standard auditing fields.

Uniqueness:
- Unique index on `(chat_id, user_id)`.

## Behavior

### Training session start/resume
- On `/review`, `/getpair`, or reminder start:
  - If an active session exists and `expires_at > now`, resume it.
  - Otherwise create a new session row and send the first prompt.
- Prompt text is recomputed from `pair_ids[current_index]` (not stored).

### Training callbacks
- On grade:
  - Validate token and message ID.
  - Apply SRS update and persist session progress:
    - increment `current_index`
    - rotate `current_token`
    - update `current_message_id`
    - update `last_activity_at` and `expires_at`
- On completion:
  - Delete session row.
  - If more than 1 card was reviewed, send:
    - `Well done reviewing N cards.`

### Reminder ticks
- If an active training session exists for the user:
  - Edit the last session message to: `The session is expired.`
  - Remove inline buttons.
  - Delete the session row.
  - Start a fresh reminder session immediately.

### Snooze
- Snooze ends the current training session and deletes the session row.

### Game sessions
- `/game` start/resume mirrors training:
  - Resume if session exists and not expired.
  - Persist after each answer.
  - Delete on completion or expiry.

## Cleanup
- Periodic cleanup job deletes any sessions where `expires_at <= now`.
- Cleanup also happens on explicit completion or snooze.

## Error Handling
- DB read/write failure: log, respond with a generic retry message, do not advance session state.
- If editing an expired message fails, still delete the session row to avoid blocking new sessions.

## Testing Plan

### Training sessions
- Resume on `/review` and `/getpair` with existing session row.
- Persist on grade (index/token/message ID updates).
- Completion deletes session and sends completion message when `reviewedCount > 1`.
- Snooze deletes the session row.

### Reminder ticks
- Expire an active session: edit message, remove buttons, delete row, start new session.
- Overdue first card includes snooze buttons (existing behavior).

### Game sessions
- Resume on `/game` if session row exists.
- Persist on answer (index/score updates).

### Cleanup
- Expired sessions are deleted (`expires_at <= now`).
