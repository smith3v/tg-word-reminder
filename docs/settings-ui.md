# Settings UI (Inline keyboard + callback queries)

This document describes the UX and the technical protocol for the **Settings** screen of the **tg-word-reminder** Telegram bot.

Goal: make bot configuration convenient in Telegram so the user **does not need to type commands with arguments** (e.g., `/setnum 5`) and can configure everything by pressing buttons.

---

## 1. Context and constraints

- The bot runs via polling and **does not accept incoming webhook requests**.
- The UX must work in standard Telegram clients (mobile/desktop).
- We use Telegram Bot API features:
  - inline keyboards,
  - `callback_query`,
  - `editMessageText` / `editMessageReplyMarkup`,
  - `answerCallbackQuery` (to remove the loading spinner and show short hints).

Platform constraints:
- `callback_data` is limited to ~64 bytes → the protocol must be short.
- Tapping a command in Telegram’s menu often **sends it immediately** → commands without arguments should open the UI instead of showing a “wrong format” error.

---

## 2. UX goals

### Primary
- Settings are accessible via a single entry point: **`/settings`**.
- All actions are performed via **inline buttons**.
- Settings are edited in a single “screen” (a single message) that the bot **edits**, instead of spamming the chat.

### Secondary
- Keep backward compatibility:
  - `/setnum` and `/setfreq` without an argument open the corresponding UI screen.

---

## 3. Terms

- **Screen**: a UI state rendered in one message (Home / Pairs / Frequency).
- **Action**: a result of pressing a button (increment/decrement/set value/back/close).
- **Reducer**: a pure function that applies an action to the current settings and returns new settings + the next screen.

---

## 4. Entry point

### `/settings` command
- The bot sends a message with the Home screen text and an inline keyboard.
- Further navigation and changes happen only via buttons (callback queries).
- When buttons are pressed, the bot **edits the same message** the callback came from.

---

## 5. Screens and behavior

### 5.1 Home (main Settings screen)
Shows current values and links to sub-screens.

**Text (example):**
- `Settings`
- `• Pairs per reminder: N`
- `• Frequency per day: M`

**Buttons:**
- `Pairs` → opens the Pairs screen
- `Frequency` → opens the Frequency screen
- `Close` → closes the UI (removes the keyboard / shows “closed”)

---

### 5.2 Pairs (configure number of pairs per reminder)
**Text (example):**
- `Pairs per reminder`
- `Current value: N`

**Buttons:**
- `-1` → decrement N by 1 (not below min)
- `+1` → increment N by 1 (not above max)
- Presets: `1`, `2`, `3`, `5`, `10` → set N to the chosen value
- `Back` → return to Home

---

### 5.3 Frequency (configure reminders per day)
**Text (example):**
- `Reminder frequency`
- `Current value: M`

**Buttons:**
- `-1` → decrement M by 1 (not below min)
- `+1` → increment M by 1 (not above max)
- Presets: `1`, `2`, `3`, `5`, `10` → set M to the chosen value
- `Back` → return to Home

---

## 6. callback_data protocol

All callback data for the Settings UI use the `s:` prefix.

### 6.1 Action list
- `s:home` — go to Home
- `s:pairs` — open the Pairs screen
- `s:freq` — open the Frequency screen
- `s:close` — close the UI

Pairs:
- `s:pairs:+1`
- `s:pairs:-1`
- `s:pairs:set:<n>` — e.g., `s:pairs:set:5`

Frequency:
- `s:freq:+1`
- `s:freq:-1`
- `s:freq:set:<n>` — e.g., `s:freq:set:3`

Requirements:
- the string must be short and consistently parseable;
- `<n>` is an ASCII integer (no spaces).

---

## 7. Value validation rules

Recommended bounds:
- `pairsPerReminder`: min=0, max=10
- `remindersPerDay`: min=0, max=10

If the user tries to go out of bounds:
- settings **do not change**,
- the bot calls `answerCallbackQuery` with a short message:
  - e.g., `Minimum is 1` / `Maximum is 10`.

---

## 8. callback_query handler behavior

General rules:
1. Call `answerCallbackQuery` **immediately** (even empty) so Telegram removes the loading spinner.
2. Parse `callback_data` → get an Action.
3. Load user settings from storage.
4. Apply the Action via the reducer:
   - if settings changed → persist them.
   - determine the next screen.
5. Render text + keyboard for the next screen.
6. Call `editMessageText` (or `editMessageReplyMarkup`) for the message the callback came from.

Recommendation:
- Edit *the exact* message that contains the keyboard (use `chat_id` + `message_id` from `callbackQuery.Message`).

---

## 9. Closing the UI (`s:close`)

`editMessageText` → “Settings saved ✅” and remove the keyboard.

Main requirement:
- after `Close`, the buttons should no longer be active.

---

## 10. Backward compatibility with commands

### `/setnum`
- If there is no argument (`/setnum`) → open the Pairs screen (inline UI).

### `/setfreq`
- If there is no argument (`/setfreq`) → open the Frequency screen (inline UI).

---

## 11. Errors and edge cases

- Unknown `callback_data`:
  - `answerCallbackQuery("Unknown command")`, do not break the UI.
- The message the callback came from is missing (rare, but possible):
  - `answerCallbackQuery("Message is not available")`.
- Storage unavailable / save error:
  - `answerCallbackQuery("Failed to save settings")`
  - You may keep the UI unchanged (do not edit the message) to avoid misleading the user.

---

## 12. Non-functional requirements

- Minimize chat noise:
  - prefer editing a single message.
- Fast feedback:
  - call `answerCallbackQuery` as early as possible.
- Code should be testable:
  - callback parsing, reducer, and screen rendering should be pure functions with unit tests.
- Add the comprehensive tests for the new code.

---

## 13. Manual testing checklist

- `/settings` opens Home
- `Pairs`:
  - `+1/-1` work and respect bounds
  - presets set the value
  - `Back` returns to Home
- `Frequency` works the same way
- `Close` removes the keyboard
- `/setnum` without argument opens the Pairs UI
- `/setfreq` without argument opens the Frequency UI
- polling receives `callback_query` updates correctly
