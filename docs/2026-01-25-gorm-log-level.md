# GORM Log Level Configuration Implementation Plan

**Goal:** Add a `logging.gorm_level` setting to independently control GORM log verbosity while keeping logs routed through slog.

**Architecture:** Extend `LoggingConfig` with a `gorm_level` field, parse it into a GORM log level with a warn default, and pass it into the GORM slog adapter in `InitDB`. Update docs and example config to describe the new option.

**Tech Stack:** Go 1.25, GORM, slog, existing `pkg/logger`.

---

### Task 1: Extend configuration and documentation

**Prompt:**
Update `pkg/config/config.go` to add `GormLevel string \`json:"gorm_level"\`` to `LoggingConfig`. Update `config.example.json` and the logging section in `README.md` to document `logging.gorm_level`, including valid values (`silent`, `error`, `warn`, `info`) and that it defaults to `warn` when unset. Implement the minimal code to support this config, write a small unit test if needed to ensure JSON decoding works, run `go test ./...`, and commit with a sentence-case summary.

---

### Task 2: Parse and apply GORM log level

**Prompt:**
Update `pkg/db/gorm_logger.go` to parse the string level, default to `warn` when empty/invalid, and return any parse error. Add a constructor that accepts the config value and returns the logger plus error. Update `pkg/db/repository.go` to use the constructor and log an error if the config value is invalid while continuing with the default. Update or add tests in `pkg/db/gorm_logger_test.go` to cover defaulting and level gating. Run `go test ./pkg/db`, and commit with a sentence-case summary.

---

### Task 3: Final verification

**Prompt:**
Run `go test ./...` to ensure all tests pass and commit any remaining changes with a sentence-case summary describing the completed feature.
