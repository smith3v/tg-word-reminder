# New Card Ordering Implementation Plan

**Goal:** Ensure due selection uses `srs_new_rank` for `new` cards so newly imported words are randomized.

**Architecture:** Keep the existing two-phase selection but adjust the “due” ordering in `SelectSessionPairs` so non-new cards remain ordered by due time while due `new` cards are ordered by `srs_new_rank`. Add a focused unit test in the training scheduler tests to lock in the ordering.

**Tech Stack:** Go 1.25, GORM, PostgreSQL (test DB via `pkg/internal/testutil`).

---

### Task 1: Update due ordering to honor `srs_new_rank` for new cards

**Prompt:**
Implement the minimal code to implement the task by updating the due query ordering in `pkg/bot/training/scheduler.go` inside `SelectSessionPairs` to:

```go
if err := db.DB.
    Where("user_id = ? AND srs_due_at <= ?", userID, now).
    Order("CASE WHEN srs_state = 'new' THEN 1 ELSE 0 END ASC").
    Order("CASE WHEN srs_state = 'new' THEN srs_new_rank ELSE 0 END ASC").
    Order("srs_due_at ASC, id ASC").
    Limit(size).
    Find(&due).Error; err != nil {
    return nil, err
}
```

Write the tests for the new code by adding a new unit test in `pkg/bot/training/scheduler_test.go` named `TestSelectSessionPairsOrdersDueNewByRank` with this exact body:

```go
func TestSelectSessionPairsOrdersDueNewByRank(t *testing.T) {
    testutil.SetupTestDB(t)

    now := time.Date(2025, 1, 2, 10, 0, 0, 0, time.UTC)
    due := db.WordPair{
        UserID:   701,
        Word1:    "review",
        Word2:    "pair",
        SrsState: string(StateReview),
        SrsDueAt: now.Add(-2 * time.Hour),
    }
    newLow := db.WordPair{
        UserID:     701,
        Word1:      "new-low",
        Word2:      "pair",
        SrsState:   string(StateNew),
        SrsDueAt:   now.Add(-1 * time.Hour),
        SrsNewRank: 10,
    }
    newHigh := db.WordPair{
        UserID:     701,
        Word1:      "new-high",
        Word2:      "pair",
        SrsState:   string(StateNew),
        SrsDueAt:   now.Add(-1 * time.Hour),
        SrsNewRank: 90,
    }

    if err := db.DB.Create(&due).Error; err != nil {
        t.Fatalf("failed to create due: %v", err)
    }
    if err := db.DB.Create(&newHigh).Error; err != nil {
        t.Fatalf("failed to create newHigh: %v", err)
    }
    if err := db.DB.Create(&newLow).Error; err != nil {
        t.Fatalf("failed to create newLow: %v", err)
    }

    got, err := SelectSessionPairs(701, 3, now)
    if err != nil {
        t.Fatalf("select failed: %v", err)
    }
    if len(got) != 3 {
        t.Fatalf("expected 3 pairs, got %+v", got)
    }
    if got[0].Word1 != "review" || got[1].Word1 != "new-low" || got[2].Word1 != "new-high" {
        t.Fatalf("expected due review then new by rank, got %+v", got)
    }
}
```

Run the tests and make sure they pass with:

```bash
go test ./pkg/bot/training -run TestSelectSessionPairsOrdersDueNewByRank
```

Expected output includes `ok   github.com/smith3v/tg-word-reminder/pkg/bot/training`.

Commit with the summary of the change as a commit message:

```
Order new due cards by srs_new_rank
```

---

### Task 2: Run the full test suite for regression coverage

**Prompt:**
Implement the minimal code to implement the task by leaving code unchanged unless tests fail; if fixes are needed, limit them to the smallest change in `pkg/bot/training/scheduler.go` or `pkg/bot/training/scheduler_test.go` to restore the ordering expectations introduced in Task 1.

Write the tests for the new code by reusing the existing training tests and adding no new tests unless a failure indicates a missing case; if needed, add a focused table-driven test in `pkg/bot/training/scheduler_test.go` that documents the failure and its fix.

Run the tests and make sure they pass with:

```bash
go test ./...
```

Expected output is a list of `ok` lines for all packages.

Commit with the summary of the change as a commit message (only if any fixes were made):

```
Fix training selection regression
```

