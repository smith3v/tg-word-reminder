# Work Trainings: Spaced Repetition Design

## Goals
- Replace random reminders with SRS-driven training sessions.
- Add on-demand reviews via `/review` (private chat only).
- Use Anki-like scheduling with self-assessment buttons.
- Keep the UX low-noise by editing single messages and using inline keyboards.

## Non-goals (v1)
- Multiple decks or tags for training content.
- FSRS or other advanced schedulers.
- Cross-user leaderboards or long-term analytics UI.

## User Experience

### On-demand: `/review`
- Private chat only.
- Session size uses the "Cards per session" setting.
- If there are no due or new cards, reply with "Nothing to review right now."
- When the session finishes, send a short completion summary (e.g. "Well done reviewing N cards.").

### Auto reminders
- Replace existing random reminders with SRS sessions.
- Sessions run at fixed local times (08:00, 13:00, 20:00) based on user timezone.
- Users enable any subset of Morning/Afternoon/Evening.
- Session size uses the "Cards per session" setting (default 5).

### Prompt and grading UI
- Each prompt shows the term plus the answer hidden as a spoiler.
  - Example: `Shown -> ||Expected||`
- Inline keyboard buttons: `Again`, `Hard`, `Good`, `Easy`.
- Buttons are visible immediately; no separate "Show" step.
- On button press, the bot edits the same message:
  - Appends a short grade marker (for example: `Again`/`Hard`/`Good`/`Easy`).
  - Removes the keyboard to prevent double-taps.
  - Sends the next prompt if the session still has cards.

### Overdue handling
At the next scheduled reminder, if the overdue count is too large for the normal daily capacity,
show a choice instead of starting immediately:
- `Catch up now`
- `Snooze 1 day`
- `Snooze 1 week`

Condition to show the prompt:
- Overdue count exceeds either the session size or the daily capacity.

Snooze behavior:
- For each card with `srs_due_at <= now`, set `srs_due_at = now + delta`.
- Do not change `srs_state`, `srs_interval_days`, or `srs_ease`.

### Inactivity pause
If the user ignores 3 consecutive reminder sessions, pause future auto reminders until they re-engage.

Definition of "missed session":
- A reminder session is considered missed if **no grading action** occurs before the next scheduled reminder slot.
- A grading action is any `Again/Hard/Good/Easy` tap or a `/review` session start.

Behavior:
- Track `missed_training_sessions` per user (consecutive count).
- After 3 consecutive misses, set `training_paused = true` and stop auto reminders.
- On any engagement (grade tap or `/review`), clear `training_paused` and reset `missed_training_sessions` to 0.
- Optionally send a one-time notice when pausing ("Paused reminders due to inactivity"). Do not spam.

## Settings

### User-facing
- "Cards per session" (stored in existing `pairs_to_send`).
- Reminder slots: Morning, Afternoon, Evening (independent toggles).
- Timezone offset (hours from UTC, via settings UI buttons).

### Reminder schedule
- Morning: 08:00
- Afternoon: 13:00
- Evening: 20:00
- Local time is derived from `timezone_offset_hours`.
- If no slots are enabled, auto reminders are off.

### Daily capacity
Daily capacity is derived, not configurable:

```
capacity = cards_per_session * enabled_slots
```

This capacity is used for overdue decisions only. `/review` ignores the capacity.

## Session Assembly

### Session size
- `/review`: `cards_per_session` from user settings.
- Auto reminders: `cards_per_session` (default 5).

### Selection order
1. Due cards: `srs_due_at <= now`, ordered by oldest `srs_due_at` first.
2. New cards: `srs_state == new`, ordered by `srs_new_rank` (ascending), then `id`.

Due cards are always used first. If due cards are fewer than the session size,
fill with new cards up to the session size.

### Session state
Training sessions are in-memory only, keyed by `(chat_id, user_id)`. Each session
tracks the list of card IDs for this run, the current prompt token, and the message ID
for safe callback handling.

Sessions are swept after 24 hours of inactivity to avoid unbounded memory growth.

## Data Model

### word_pairs additions

Each field includes its meaning and how it is used by the algorithm.

- `srs_state` (string enum: `new`, `learning`, `review`)
  - Determines which scheduling rules apply.
  - New pairs start as `new` and move to `learning` on first review.

- `srs_due_at` (timestamp)
  - The next time the card is eligible to appear.
  - Card is due if `srs_due_at <= now`.

- `srs_last_reviewed_at` (timestamp)
  - Updated on every grade, used for diagnostics and analytics.

- `srs_interval_days` (int)
  - Current interval in days for `review` state.
  - Used with `srs_ease` to compute the next interval.
  - Set to 0 for `new` and `learning` states.

- `srs_ease` (float)
  - Ease factor, starts at 2.5, minimum 1.3.
  - Updated on `review` grades (Hard/Good/Easy, and Again on lapse).

- `srs_step` (int)
  - Index into learning steps (0-based).
  - Only meaningful in `learning` state.
  - Set to -1 when in `review`.

- `srs_reps` (int)
  - Count of successful review reps (Hard/Good/Easy in `review`).

- `srs_lapses` (int)
  - Count of lapses (Again in `review`).

- `srs_new_rank` (float or int)
  - Stable per-pair shuffle rank for ordering new cards.
  - Assigned randomly on import/creation.
  - Used only to order `new` cards; does not affect scheduling intervals.

### Defaults for existing pairs
On migration, all existing pairs become `new` with:
- `srs_state = new`
- `srs_due_at = now`
- `srs_interval_days = 0`
- `srs_ease = 2.5`
- `srs_step = 0`
- `srs_new_rank = random()`

### user_settings changes
- Reuse `pairs_to_send` as "Cards per session".
- Remove `reminders_per_day`.
- Add booleans: `reminder_morning`, `reminder_afternoon`, `reminder_evening`.
- Add `timezone_offset_hours` (int, range -12 to +14).
- Add `missed_training_sessions` (int, consecutive misses).
- Add `training_paused` (bool).
- Add `last_training_sent_at` (timestamp).
- Add `last_training_engaged_at` (timestamp).

Migration mapping for `reminders_per_day` before removal:
- `0`: disable all reminder slots.
- `1`: enable `reminder_evening` only.
- `2`: enable `reminder_morning` and `reminder_evening`.
- `>2`: enable all three slots.

### Indexing
- Add an index on `(user_id, srs_due_at)` for efficient due lookups.

## Scheduling Algorithm Details

### Learning steps
Default steps: 10 minutes, then 1 day.

State transitions and grading (learning):
- Again: `srs_step = 0`, `srs_due_at = now + 10m`.
- Hard: repeat current step, `srs_due_at = now + step_duration`.
- Good: advance to next step. If last step completed, graduate to `review`:
  - `srs_state = review`, `srs_step = -1`, `srs_interval_days = 1`,
    `srs_due_at = now + 1d`.
- Easy: graduate immediately to `review`:
  - `srs_state = review`, `srs_step = -1`, `srs_interval_days = 4`,
    `srs_due_at = now + 4d`.

Ease is not modified during learning.

### Review state (Anki-like defaults)
Let `I` be `srs_interval_days` and `E` be `srs_ease`.

- Again (lapse):
  - `srs_lapses++`
  - `srs_ease = max(1.3, E - 0.2)`
  - `srs_state = learning`, `srs_step = 0`
  - `srs_due_at = now + 10m`

- Hard:
  - `srs_ease = max(1.3, E - 0.15)`
  - `srs_interval_days = max(1, round(I * 1.2))`
  - `srs_due_at = now + srs_interval_days days`
  - `srs_reps++`

- Good:
  - `srs_interval_days = max(1, round(I * E))`
  - `srs_due_at = now + srs_interval_days days`
  - `srs_reps++`

- Easy:
  - `srs_ease = E + 0.15`
  - `srs_interval_days = max(1, round(I * E * 1.3))`
  - `srs_due_at = now + srs_interval_days days`
  - `srs_reps++`

Always update `srs_last_reviewed_at = now` after a grade.

### Direction
Each review chooses direction randomly (A->B or B->A). Direction is not persisted.

## Callback Data
Use a short, parseable callback payload with a training prefix.
Example format:
- `t:grade:<token>:again|hard|good|easy`

The token binds a button to the current prompt and prevents stale taps.

## Error Handling
- If callback data is invalid or stale, answer the callback with a short notice
  and do not change state.
- If the message is inaccessible, respond with "Message is not available".
- If DB updates fail, log the error and do not advance the session.

## Testing
- Scheduler unit tests: due-first selection, new fill, overdue prompt condition.
- Learning transitions: Again/Hard/Good/Easy with step handling.
- Review transitions: interval/ease math and lapse behavior.
- Settings callbacks: slot toggles, timezone offsets, cards per session.
- Reminder time calculation for each slot and timezone offset.
