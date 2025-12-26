# Feature Proposal: `/game` — Vocabulary Quiz Mode (5 pairs, deck until empty)

## Background / context

`tg-word-reminder` already supports user-uploaded word pairs and commands like `/getpair`, `/settings`, `/clear`, plus periodic reminders using stored vocabulary.

This proposal adds a conversational mini-game to practice recall in the chat.

## Goals

- Provide an interactive “quiz session” started by `/game`.
- Each session uses a **fixed deck of 5 unique word pairs**.
- For each pair, create **two prompt cards**:
  - **A → ?** (expect B)
  - **B → ?** (expect A)
- Shuffle prompt cards at session start (initially **10 cards** when 5 pairs exist).
- Each prompt message includes an inline button **👀**:
  - pressing it **reveals the correct answer** by editing the prompt message,
  - **counts as a miss**, and
  - **returns the card back into the deck** (so the user will see it again later).
- Typed answers are evaluated:
  - if correct: reveal the full pair (edit message) and **remove the card from the deck**,
  - if incorrect: **count a miss** and **return the card back into the deck** (no reveal).
- The game can take **more than 10 tries** because missed/revealed cards re-enter the deck.
- Session ends when:
  - the **deck is empty** (all prompt cards were answered correctly), or
  - **15 minutes of inactivity** (no attempts).
- End-of-session stats:
  - greet the user with answering **N** times correctly during the session
  - show **percentage correct** during the session

## Non-goals (for v1)

- No spaced repetition weighting, streaks across days, leaderboards, or global stats.
- No fuzzy matching beyond basic normalization (optional minimal tolerance only).

---

## Session deck rules (5 pairs)

### Pair selection

- On `/game`, build a deck of **5 distinct pairs** sampled randomly from the user’s vocabulary.
- If the user has **fewer than 5 pairs**, pick **all available pairs** and build prompt cards for them (deck size becomes `2 * available_pairs`; normally 10 cards when 5 pairs exist).

### Prompt card construction

For each selected pair `(A, B)` generate two prompt cards:

1. Card type `A_to_B`: show `A`, expect `B`
2. Card type `B_to_A`: show `B`, expect `A`

Shuffle all cards once at session start.

### Deck dynamics

Treat the deck as a queue:

- To ask a question: **draw** (pop) the top card.
- On **correct typed answer**: card is **completed** (discarded, not returned).
- On **incorrect typed answer**: card is **returned** to the back of the deck.
- On **👀 reveal button**: card is **returned** to the back of the deck.

This guarantees the session ends exactly when every card is eventually answered correctly.

---

## UX specification

### Command: `/game`

**Trigger**

- User sends `/game` in a private chat (group chats are not supported for now).

**Bot response (session start)**

- Starts a new “Game Session” tied to `(chat_id, user_id)`.
- Builds and shuffles the deck (see “Session deck rules”).
- Immediately posts the first prompt message.

### Prompt message

**Prompt format (example)**

> Translate: **wordA** → ?\
> *(reply with the missing word, or tap 👀 to reveal — counts as a miss)*

**Inline keyboard**

- One button: **👀**
- Callback: “reveal this card” for the current prompt

### User answers

Each user action that resolves the current prompt counts as an **attempt**:

- sending a text answer (correct or incorrect), OR
- pressing 👀

**Correct typed answer**

- Bot edits the *prompt message* to reveal the full pair with the answer hidden under a spoiler and mark it correct:

> **wordA — ||wordB|| ✅**

- Bot removes the card from the deck.
- Bot sends the next prompt (if deck not empty).

**Incorrect typed answer**

- Bot edits the *prompt message* to reveal the full pair with the answer hidden under a spoiler and remove the inline keyboard:

> **wordA — ||wordB|| ❌**

- Bot counts a miss.
- Bot returns the card to the back of the deck.
- Bot sends the next prompt.

**👀 Reveal button**

- Bot counts a miss.
- Bot edits the *prompt message* to show the correct answer hidden under a spoiler):

> **wordA — ||wordB|| 👀**

- Bot returns the card to the back of the deck.
- Bot sends the next prompt.

**Important UX detail**

After a prompt is resolved (by correct answer, incorrect answer, or 👀), the bot should remove/disable the inline keyboard on that message to prevent double-counting via repeated clicks.

### Session end

A session ends when either condition is met:

1. **Deck is empty** (all cards answered correctly), OR
2. **15 minutes of inactivity** (no attempts)

**Stats message format**

> **Game over!**\
> You got **N** correct answers.\
> Accuracy: **P%** (N/M)

Where:

- `N = correctCount`
- `M = attemptCount`
- `P = round(100*N/M)` if `M>0`, else `0%`

---

## Answer matching rules (Normalization)

To avoid frustrating “almost correct” mismatches while staying deterministic:

- `strings.TrimSpace`
- `strings.ToLower` (or Unicode-aware case fold)
- Collapse multiple spaces to one
- Ignore trailing punctuation like `.`, `!`, `?`, `,`

**Explicitly out of scope (v1):**

- Levenshtein distance / fuzzy typo tolerance
- Synonym matching

---

## Data model

### In-memory session (recommended v1)

Because this is a “live chat game” and the bot is typically single-instance/self-hosted, v1 can store sessions in memory.

`GameSession`

- Identity
  - `chatID int64`
  - `userID int64`
- Lifecycle
  - `startedAt time.Time`
  - `lastActivityAt time.Time`
  - `active bool`
- Deck
  - `pairs []Pair` (size 5 or fewer)
  - `deck []Card` (size `2*len(pairs)` at start)
- Current prompt (one at a time)
  - `currentCard Card` (or pointer / index)
  - `currentMessageID int`
  - `currentResolved bool`
- Stats
  - `correctCount int`
  - `attemptCount int`

`Card`

- `pairID`
- `direction enum {A_to_B, B_to_A}`
- `shown string`
- `expected string`

**Why in-memory**

- Minimal schema changes.
- Resets naturally on bot restart (acceptable for a “session game”).

---

## State machine

States per `(chat_id, user_id)`:

1. **Idle**
2. **AwaitingAttempt**
   - Has a non-empty `deck` and one active `currentCard`
3. **Finished**
   - Terminal state; session removed from active map

Transitions:

- Idle → AwaitingAttempt on `/game`
- AwaitingAttempt → AwaitingAttempt on user attempt:
  - update `attemptCount++`, `lastActivityAt = now`
  - if correct: `correctCount++`, discard card
  - if incorrect or 👀: return card to deck back
  - if deck empty: finish
  - else: draw next card, send next prompt
- AwaitingAttempt → Finished on:
  - timeout (15 minutes inactivity)

---

## Timeout semantics

**Definition of inactivity**

- No user action that qualifies as an attempt (text answer or 👀 callback) within 15 minutes.

**Timer reset rule**

- On every attempt, update `lastActivityAt = now`.

**Implementation options**

- Per-session `time.Timer` reset on activity (simple, but watch for timer/goroutine leaks), OR
- Single sweeper goroutine runs every minute:
  - iterate active sessions
  - if `now - lastActivityAt > 15m`: finish session and send stats

---

## Telegram API interactions

### Prompt message

- `SendMessage` with:
  - prompt text
  - `InlineKeyboardMarkup` containing button **👀**
  - callback data encoding enough info to map to the active session/card

### On typed answer

- Evaluate answer against `currentCard.expected`.
- `EditMessageText` (and/or `EditMessageReplyMarkup`) to:
  - on correct: to reveal full pair and annotate with ✅ and remove keyboard
  - on incorrect: to reveal full pair and annotate with ❌ and remove keyboard

### On 👀 callback

- Validate callback belongs to the active session and the current message/card.
- `EditMessageText` to reveal full pair and annotate with 👀.
- Remove keyboard to prevent repeated clicks.
- Requeue card.

### Update handling

- Only accept attempts from the same `(chat_id, user_id)` as the session.
- For safety, ignore late/replayed callbacks by checking `currentMessageID` and `currentResolved`.

---

## Concurrency and edge cases

- **Starting `/game` while a session is active**
  - restart session (new deck + reset stats) and send “Starting a new game!”
- **Vocabulary too small**
  - If fewer than 5 pairs exist, start with fewer pairs (deck size becomes `2*N`).
- **Edits not allowed**
  - If Telegram refuses edit, fall back to sending a “wordA — wordB 👀” message and remove keyboard. Count as a miss.
- **Double-count prevention**
  - Remove keyboard after resolution.

---

## Metrics (session stats)

- `attemptCount`: increment once per resolved prompt (typed answer or 👀)
- `correctCount`: increment only on correct typed answers
- `accuracyPercent`:
  - if `attemptCount > 0`: `round(100 * correctCount / attemptCount)`
  - else `0`

---

## Acceptance criteria

- `/game` starts a session and builds a deck of **5 unique pairs** (or fewer if vocabulary is smaller).
- The session generates **two cards per pair**, one in each direction.
- A prompt message includes an inline **👀** button.
- Pressing 👀 reveals the correct answer (hidden under a spoiler) by editing the message, counts a miss, and requeues the card.
- Incorrect typed answers count a miss, reveals the answer (hidden under a spoiler) and requeue the card.
- Correct typed answers reveal the full pair (with the answer hidden under a spoiler) by editing the message and permanently remove the card.
- Session ends when the **deck becomes empty** or after **15 minutes inactivity**, then prints stats including:
  - correct count (`N`)
  - accuracy percentage (`P%`)

---

## Non-functional requirements

- Minimize chat noise:
  - prefer editing a single message.
- Code should be testable. Introduce pure functions when possible.
- Add the comprehensive tests for the new code.

---

## Testing notes (lightweight)

- Unit tests:
  - normalization function
  - deck creation (N pairs → 2N cards)
  - correctness path removes card
  - incorrect path requeues card
  - 👀 path reveals + requeues card
  - accuracy calculation
  - timeout decision (`now - lastActivityAt`)
- Manual tests:
  - answer correct until deck empty → game ends
  - use 👀 on some prompts → verify game takes more attempts and stats reflect misses
  - spam 👀 button → ensure no double-count (keyboard removed + `currentResolved` guard)

---

## Suggested documentation addition

Add a short section to README:

- How to start: `/game`
- How it works: 5 pairs per session, two directions, 👀 reveals (miss + requeue), ends when deck empty or after 15 minutes inactivity, shows stats.
