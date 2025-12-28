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
- Migration note: deploying this change on existing databases requires creating the unique index; ensure any duplicates per `(user_id, word1)` are resolved before applying the migration.

### Pre-migration duplicate cleanup checklist (PostgreSQL)

- Identify duplicates:
  ```sql
  SELECT user_id, word1, COUNT(*) AS duplicate_count
  FROM word_pairs
  GROUP BY user_id, word1
  HAVING COUNT(*) > 1
  ORDER BY duplicate_count DESC, user_id;
  ```
- Show only duplicate rows (full records):
  ```sql
  SELECT wp.*
  FROM word_pairs wp
  JOIN (
    SELECT user_id, word1
    FROM word_pairs
    GROUP BY user_id, word1
    HAVING COUNT(*) > 1
  ) dups
    ON wp.user_id = dups.user_id
   AND wp.word1 = dups.word1
  ORDER BY wp.user_id, wp.word1, wp.id;
  ```
- Inspect the specific rows for a given user/word:
  ```sql
  SELECT id, user_id, word1, word2
  FROM word_pairs
  WHERE user_id = $USER_ID AND word1 = $WORD1
  ORDER BY id;
  ```
- Keep one row (lowest id) and remove the rest:
  ```sql
  DELETE FROM word_pairs a
  USING word_pairs b
  WHERE a.user_id = b.user_id
    AND a.word1 = b.word1
    AND a.id > b.id;
  ```
- If duplicates have different `word2` values, merge them into a single comma-separated value before deletion. Example (keeps lowest id and merges others):
  ```sql
  WITH merged AS (
    SELECT
      MIN(id) AS keep_id,
      user_id,
      word1,
      STRING_AGG(DISTINCT word2, ',' ORDER BY word2) AS merged_word2
    FROM word_pairs
    GROUP BY user_id, word1
    HAVING COUNT(*) > 1
  )
  UPDATE word_pairs wp
  SET word2 = merged.merged_word2
  FROM merged
  WHERE wp.id = merged.keep_id;
  ```
- Verify cleanup:
  ```sql
  SELECT COUNT(*) AS remaining_duplicates
  FROM (
    SELECT 1
    FROM word_pairs
    GROUP BY user_id, word1
    HAVING COUNT(*) > 1
  ) t;
  ```

---

## Non-functional requirements

- Code should be testable. Introduce pure functions when possible.
- Add the comprehensive tests for the new code.
