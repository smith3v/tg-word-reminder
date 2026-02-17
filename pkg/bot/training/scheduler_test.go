package training

import (
	"testing"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
)

func TestApplyGradeLearningTransitions(t *testing.T) {
	now := time.Date(2025, 1, 2, 10, 0, 0, 0, time.UTC)
	pair := db.WordPair{SrsState: string(StateNew)}

	ApplyGrade(&pair, GradeAgain, now)
	if pair.SrsState != string(StateLearning) || pair.SrsStep != 0 {
		t.Fatalf("expected learning step 0, got %+v", pair)
	}
	if pair.SrsDueAt != now.Add(10*time.Minute) {
		t.Fatalf("expected due in 10m, got %v", pair.SrsDueAt)
	}

	ApplyGrade(&pair, GradeHard, now)
	if pair.SrsStep != 0 {
		t.Fatalf("expected hard to keep step 0, got %+v", pair)
	}
	if pair.SrsDueAt != now.Add(10*time.Minute) {
		t.Fatalf("expected hard to repeat step, got %v", pair.SrsDueAt)
	}

	ApplyGrade(&pair, GradeGood, now)
	if pair.SrsState != string(StateLearning) || pair.SrsStep != 1 {
		t.Fatalf("expected step 1, got %+v", pair)
	}
	if pair.SrsDueAt != now.Add(24*time.Hour) {
		t.Fatalf("expected due in 1d, got %v", pair.SrsDueAt)
	}

	ApplyGrade(&pair, GradeGood, now)
	if pair.SrsState != string(StateReview) || pair.SrsIntervalDays != 1 {
		t.Fatalf("expected graduation with 1d interval, got %+v", pair)
	}
	if pair.SrsDueAt != now.AddDate(0, 0, 1) {
		t.Fatalf("expected due in 1d, got %v", pair.SrsDueAt)
	}
}

func TestApplyGradeLearningEasyGraduates(t *testing.T) {
	now := time.Date(2025, 1, 2, 10, 0, 0, 0, time.UTC)
	pair := db.WordPair{SrsState: string(StateNew)}

	ApplyGrade(&pair, GradeEasy, now)
	if pair.SrsState != string(StateReview) || pair.SrsIntervalDays != 4 {
		t.Fatalf("expected easy to graduate to 4d, got %+v", pair)
	}
	if pair.SrsDueAt != now.AddDate(0, 0, 4) {
		t.Fatalf("expected due in 4d, got %v", pair.SrsDueAt)
	}
}

func TestApplyGradeReviewTransitions(t *testing.T) {
	now := time.Date(2025, 1, 2, 10, 0, 0, 0, time.UTC)
	pair := db.WordPair{
		SrsState:        string(StateReview),
		SrsIntervalDays: 10,
		SrsEase:         2.5,
		SrsStep:         -1,
	}

	ApplyGrade(&pair, GradeHard, now)
	if pair.SrsIntervalDays != 12 {
		t.Fatalf("expected hard interval 12, got %+v", pair)
	}
	if pair.SrsEase != 2.35 {
		t.Fatalf("expected ease 2.35, got %+v", pair)
	}

	ApplyGrade(&pair, GradeGood, now)
	if pair.SrsIntervalDays != 28 {
		t.Fatalf("expected good interval 28, got %+v", pair)
	}

	ApplyGrade(&pair, GradeEasy, now)
	if pair.SrsEase <= 2.35 {
		t.Fatalf("expected ease increase, got %+v", pair)
	}

	ApplyGrade(&pair, GradeAgain, now)
	if pair.SrsState != string(StateLearning) || pair.SrsStep != 0 {
		t.Fatalf("expected lapse to learning step 0, got %+v", pair)
	}
	if pair.SrsLapses != 1 {
		t.Fatalf("expected lapses increment, got %+v", pair)
	}
}

func TestApplyGradeReviewEaseFloor(t *testing.T) {
	now := time.Date(2025, 1, 2, 10, 0, 0, 0, time.UTC)
	pair := db.WordPair{
		SrsState:        string(StateReview),
		SrsIntervalDays: 5,
		SrsEase:         1.35,
		SrsStep:         -1,
	}

	ApplyGrade(&pair, GradeHard, now)
	if pair.SrsEase != EaseFloor {
		t.Fatalf("expected ease floor %.1f, got %+v", EaseFloor, pair)
	}
}

func TestSelectSessionPairs(t *testing.T) {
	testutil.SetupTestDB(t)

	now := time.Date(2025, 1, 2, 10, 0, 0, 0, time.UTC)
	due1 := db.WordPair{UserID: 700, Word1: "a", Word2: "b", SrsState: string(StateReview), SrsDueAt: now.Add(-2 * time.Hour)}
	due2 := db.WordPair{UserID: 700, Word1: "c", Word2: "d", SrsState: string(StateReview), SrsDueAt: now.Add(-1 * time.Hour)}
	new1 := db.WordPair{UserID: 700, Word1: "e", Word2: "f", SrsState: string(StateNew), SrsDueAt: now.Add(24 * time.Hour)}

	if err := db.DB.Create(&due1).Error; err != nil {
		t.Fatalf("failed to create due1: %v", err)
	}
	if err := db.DB.Create(&due2).Error; err != nil {
		t.Fatalf("failed to create due2: %v", err)
	}
	if err := db.DB.Create(&new1).Error; err != nil {
		t.Fatalf("failed to create new1: %v", err)
	}

	got, err := SelectSessionPairs(700, 2, now)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if len(got) != 2 || got[0].Word1 != "a" || got[1].Word1 != "c" {
		t.Fatalf("expected due pairs first, got %+v", got)
	}

	got, err = SelectSessionPairs(700, 3, now)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if len(got) != 3 || got[2].Word1 != "e" {
		t.Fatalf("expected new pair to fill, got %+v", got)
	}
}

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

func TestSelectSessionPairsAvoidsDuplicatesAcrossDueAndFresh(t *testing.T) {
	testutil.SetupTestDB(t)

	now := time.Date(2025, 1, 2, 10, 0, 0, 0, time.UTC)
	dueReview := db.WordPair{
		UserID:   702,
		Word1:    "review",
		Word2:    "pair",
		SrsState: string(StateReview),
		SrsDueAt: now.Add(-2 * time.Hour),
	}
	dueNew := db.WordPair{
		UserID:     702,
		Word1:      "new-due",
		Word2:      "pair",
		SrsState:   string(StateNew),
		SrsDueAt:   now.Add(-time.Hour),
		SrsNewRank: 10,
	}
	futureNew := db.WordPair{
		UserID:     702,
		Word1:      "new-future",
		Word2:      "pair",
		SrsState:   string(StateNew),
		SrsDueAt:   now.Add(24 * time.Hour),
		SrsNewRank: 20,
	}

	if err := db.DB.Create(&dueReview).Error; err != nil {
		t.Fatalf("failed to create dueReview: %v", err)
	}
	if err := db.DB.Create(&dueNew).Error; err != nil {
		t.Fatalf("failed to create dueNew: %v", err)
	}
	if err := db.DB.Create(&futureNew).Error; err != nil {
		t.Fatalf("failed to create futureNew: %v", err)
	}

	got, err := SelectSessionPairs(702, 3, now)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 pairs, got %+v", got)
	}
	if got[0].Word1 != "review" || got[1].Word1 != "new-due" || got[2].Word1 != "new-future" {
		t.Fatalf("expected no duplicates across due+fresh selection, got %+v", got)
	}
}
