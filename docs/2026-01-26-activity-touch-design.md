# Activity touch batching for reminder reactivation

Date: 2026-01-26

## Problem
Users who miss three reminder slots are paused (`training_paused = true`). If they return and interact via `/game` (or other non-review paths), reminders stay paused because only review/overdue flows call `markTrainingEngaged`. This can leave users unaware they are paused.

## Goal
Unpause reminders and reset missed sessions on **any user interaction** (messages + callbacks), without updating `last_training_engaged_at`. Avoid a DB write on every interaction by batching at most once per minute.

## Non-goals
- Changing the definition of training engagement or updating `last_training_engaged_at`.
- Multi-instance coordination (single bot instance assumed).
- Behavioral changes to reminder scheduling logic beyond reactivation.

## Proposed approach
Introduce an in-memory activity tracker that records unique user IDs from inbound updates. A background flusher runs every minute and updates user settings in the database for touched users:

- `training_paused = false`
- `missed_training_sessions = 0`

The tracker deletes user IDs from its map after a **successful** database update. If the update fails, entries remain to retry on the next tick.

## Components
- `ActivityTracker`
  - `Touch(userID int64)` — record activity in a concurrency-safe map.
  - `Flush(ctx)` — snapshot user IDs, perform batched DB updates, and clean up on success.
- `ActivityMiddleware` (bot middleware)
  - Extract user ID from `update.Message.From.ID` or `update.CallbackQuery.From.ID`.
  - Call `tracker.Touch(userID)` and then continue to the next handler.
- `StartActivityFlusher(ctx, tracker)`
  - Ticker loop (1 minute) that calls `tracker.Flush(ctx)` until context cancellation.

## Data flow
1. Bot receives update (message or callback).
2. Middleware extracts user ID and records `Touch`.
3. Every minute, flusher snapshots touched IDs and executes a batched DB update.
4. On success, IDs are removed from the map; on error, they remain for retry.

## Error handling
- Middleware is best-effort and must not fail requests.
- Flush errors are logged; entries are retained for retry.
- Missing/zero user IDs are ignored.

## Edge cases
- Multiple interactions in a minute collapse to one update.
- Process restart loses pending touches; next interaction re-touches.
- If a user settings row is missing, the update is effectively a no-op; IDs are still cleared to avoid infinite retry.

## Testing plan
- Unit test `ActivityTracker`:
  - Touch collapses duplicates.
  - Successful flush removes entries.
  - Failed flush keeps entries.
  - No-op for invalid user IDs.
- Optional handler/middleware test to verify `Touch` on message and callback updates.

## Rollout
- Implement middleware and flusher.
- Enable middleware via `bot.WithMiddlewares`.
- Start flusher alongside existing sweepers in `main.go`.
- Monitor logs for DB update errors.
