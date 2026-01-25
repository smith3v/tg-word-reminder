# GORM Logging Implementation Plan

**Goal:** Route GORM logs through the app slog logger so they go to stdout + the configured log file with consistent formatting and level filtering.

**Architecture:** Add a small GORM logger adapter in `pkg/db` that emits slog records via `pkg/logger` and gates output based on the app's current log level. `InitDB` will pass this adapter via `gorm.Config{Logger: ...}` so GORM uses it for all DB work.

**Tech Stack:** Go 1.25, GORM, slog, existing `pkg/logger`.

---

### Task 1: Expose logger level gating helper

**Prompt:**
Update `pkg/logger/logger.go` to add an exported helper (e.g., `Enabled(level LogLevel) bool` or `CurrentLevel() LogLevel`) that reports whether the current app log level allows a message. Implement the minimal code for that helper, write a unit test in `pkg/logger/logger_test.go` that sets `SetLogLevel(...)` and asserts the helper returns the expected values, run `go test ./pkg/logger`, and commit with a sentence-case message describing the change.

---

### Task 2: Add a GORM slog adapter

**Prompt:**
Create `pkg/db/gorm_logger.go` implementing `gorm.io/gorm/logger.Interface` (methods `LogMode`, `Info`, `Warn`, `Error`, `Trace`). The adapter should format `Info/Warn/Error` via `fmt.Sprintf(msg, data...)`, and `Trace` should log SQL, rows, elapsed time, and errors, using slog levels (info for normal queries, warn for slow queries, error for failures). Gate output based on both the GORM log level and the app logger helper from Task 1 so it dynamically matches the configured app logging level. Use a sensible slow threshold (e.g., 200ms) and include structured fields like `sql`, `rows`, and `elapsed`. Add a unit test in `pkg/db/gorm_logger_test.go` that sets `logger.Logger` to a buffer, tweaks log levels, calls the adapter methods, and asserts expected output. Run `go test ./pkg/db`, and commit with a sentence-case message describing the new adapter.

---

### Task 3: Wire the adapter into DB initialization

**Prompt:**
Update `pkg/db/repository.go` to pass the new adapter in `gorm.Open` via `&gorm.Config{Logger: newGormLogger()}` (or the chosen constructor). If any tests or helpers need adjustment to use the same logger, update them with minimal changes. Run `go test ./...` to ensure the suite still passes, and commit with a sentence-case message describing the wiring change.
