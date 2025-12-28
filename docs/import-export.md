# Feature Proposal: Import/Export - Vocabulary CSV

## Background / context

`tg-word-reminder` currently accepts user vocabulary by uploading a CSV file, but the parser is limited to tab-separated input and there is no export path. This proposal broadens import support to general CSV inputs and adds an explicit `/export` command for users to download their vocabulary.

## Goals

- Support general CSV input (commas, tabs, semicolons, quotes) when importing vocabulary.
- Upsert word pairs on import using `word1` as the key per user.
- Preserve any existing per-pair stats when updating word pairs.
- Add a `/export` command that returns a comma-separated CSV file to the user.

## Non-goals (v1)

- No schema for arbitrary extra columns beyond the two word fields.
- No automatic language detection, translation, or dedupe beyond `word1`.
- No deletion of word pairs that are missing from the import file.
- No export formats other than comma-separated CSV.

---

## Import specification

### Entry points

- **File upload**: user sends a `.csv` document in a private chat.
- The bot treats the uploaded document as a vocabulary import request.

### CSV parsing rules

- Use Go’s `encoding/csv` directly and implement delimiter detection in the import path.
- Accept common delimiters: comma, tab, semicolon.
- Accept quoted fields and embedded delimiters inside quotes.
- Ignore empty lines.
- Optional header row is allowed. If the first row matches case-insensitive names like `word1`, `word2`, `source`, `target`, then skip it.
- The first two columns are `word1` and `word2`. Extra columns are ignored.

### Validation and normalization

- Trim leading/trailing whitespace for both fields.
- Skip rows with missing/empty `word1` or `word2`.
- Keep the trimmed values as-is (case sensitive match for `word1`).
- Validate the entire file first and collect all valid pairs before any database writes.
- Only proceed with the import if at least one valid pair is collected.

### Upsert behavior (per user)

- After validating the full dataset, perform the upsert for the collected pairs:
  - For each row, look up an existing pair by `(user_id, word1)`.
  - If a match exists:
    - Update only `word2` (and any relevant normalization fields, if added later).
    - Preserve any pair-related stats (for example: correct/incorrect counts, streaks, last_seen timestamps).
  - If no match exists:
    - Create a new `WordPair` row.
  - Do not delete pairs that are absent from the file.

### User feedback

- Report a summary after import:
  - `Imported N new pairs, updated M pairs, skipped K rows.`
- For parse errors, respond with a short error message and stop the import.
- For row-level errors (missing fields), count them as skipped and continue.

### Delimiter detection algorithm

Use a small in-memory sample of the CSV file to pick the delimiter before full parsing:

- Read the first N non-empty lines (for example: N=20) as raw bytes.
- For each candidate delimiter `{',', '\t', ';'}`:
  - Parse the sample using `encoding/csv` with that delimiter and RFC 4180 defaults.
  - Count the number of fields per line.
  - Score the delimiter by the number of lines that have **at least 2 fields** and **consistent field counts**.
- Choose the delimiter with the highest score; if there’s a tie, prefer comma, then tab, then semicolon.
- If no delimiter yields a usable score, default to comma.

---

## Export specification

### Command: `/export`

**Trigger**
- User sends `/export` in a private chat.

**Behavior**
- Fetch all word pairs for the user.
- If there are no pairs, return a short message: `You have no vocabulary to export.`
- Otherwise, generate a **comma-separated** CSV file with:
  - No header.
  - One row per pair: `word1,word2`.
  - Quote fields as required by RFC 4180 (commas, quotes, or newlines in data).
  - Deterministic ordering (for example: `word1` ASC, then `id` ASC).

**Delivery**
- Send the file as a Telegram document attachment.
- Suggested file name: `vocabulary-YYYYMMDD.csv`.
- Include a short caption, e.g., `Your vocabulary export (N pairs).`
- Follow RFC 4180 as closely as possible, including CRLF line endings.
- For compatibility with common spreadsheet tools, generate UTF-8 with BOM.

---

## Data model considerations

- Add a unique index on `(user_id, word1)` to enforce a single row per word per user.
- Ensure update statements only modify `word2` to preserve any existing stats columns.

---

## Non-functional requirements

- Code should be testable. Introduce pure functions when possible.
- Add the comprehensive tests for the new code.
