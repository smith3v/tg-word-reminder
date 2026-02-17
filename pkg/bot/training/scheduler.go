package training

import (
	"math"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
)

type SrsState string

const (
	StateNew      SrsState = "new"
	StateLearning SrsState = "learning"
	StateReview   SrsState = "review"
)

type Grade string

const (
	GradeAgain Grade = "again"
	GradeHard  Grade = "hard"
	GradeGood  Grade = "good"
	GradeEasy  Grade = "easy"
)

const (
	LearningStepMinutes = 10
	EaseFloor           = 1.3
)

var LearningSteps = []time.Duration{
	LearningStepMinutes * time.Minute,
	24 * time.Hour,
}

func SelectSessionPairs(userID int64, size int, now time.Time) ([]db.WordPair, error) {
	if size <= 0 {
		return []db.WordPair{}, nil
	}

	var due []db.WordPair
	if err := db.DB.
		Where("user_id = ? AND srs_due_at <= ?", userID, now).
		Order("CASE WHEN srs_state = 'new' THEN 1 ELSE 0 END ASC").
		Order("CASE WHEN srs_state = 'new' THEN srs_new_rank ELSE 0 END ASC").
		Order("srs_due_at ASC, id ASC").
		Limit(size).
		Find(&due).Error; err != nil {
		return nil, err
	}

	if len(due) >= size {
		return due, nil
	}

	remaining := size - len(due)
	selectedIDs := make([]uint, 0, len(due))
	for _, pair := range due {
		if pair.ID != 0 {
			selectedIDs = append(selectedIDs, pair.ID)
		}
	}

	var fresh []db.WordPair
	query := db.DB.
		Where("user_id = ? AND srs_state = ?", userID, StateNew).
		Order("srs_new_rank ASC, id ASC")
	if len(selectedIDs) > 0 {
		query = query.Where("id NOT IN ?", selectedIDs)
	}
	if err := query.Limit(remaining).Find(&fresh).Error; err != nil {
		return nil, err
	}

	return append(due, fresh...), nil
}

func ApplyGrade(pair *db.WordPair, grade Grade, now time.Time) {
	if pair == nil {
		return
	}

	switch SrsState(pair.SrsState) {
	case StateNew:
		applyLearning(pair, grade, now, true)
	case StateLearning:
		applyLearning(pair, grade, now, false)
	case StateReview:
		applyReview(pair, grade, now)
	default:
		pair.SrsState = string(StateNew)
		applyLearning(pair, grade, now, true)
	}
}

func applyLearning(pair *db.WordPair, grade Grade, now time.Time, fromNew bool) {
	pair.SrsState = string(StateLearning)
	if fromNew {
		pair.SrsStep = 0
	}

	switch grade {
	case GradeAgain:
		pair.SrsStep = 0
		pair.SrsDueAt = now.Add(LearningSteps[0])
	case GradeHard:
		step := clampStep(pair.SrsStep)
		pair.SrsDueAt = now.Add(LearningSteps[step])
	case GradeGood:
		step := clampStep(pair.SrsStep)
		if step+1 >= len(LearningSteps) {
			graduateToReview(pair, now, 1)
			return
		}
		pair.SrsStep = step + 1
		pair.SrsDueAt = now.Add(LearningSteps[pair.SrsStep])
	case GradeEasy:
		graduateToReview(pair, now, 4)
		return
	default:
		step := clampStep(pair.SrsStep)
		pair.SrsDueAt = now.Add(LearningSteps[step])
	}

	pair.SrsLastReviewedAt = &now
}

func applyReview(pair *db.WordPair, grade Grade, now time.Time) {
	switch grade {
	case GradeAgain:
		pair.SrsLapses++
		pair.SrsEase = maxEase(pair.SrsEase - 0.2)
		pair.SrsState = string(StateLearning)
		pair.SrsStep = 0
		pair.SrsDueAt = now.Add(LearningSteps[0])
		pair.SrsLastReviewedAt = &now
		return
	case GradeHard:
		pair.SrsEase = maxEase(pair.SrsEase - 0.15)
		pair.SrsIntervalDays = maxInt(1, int(math.Round(float64(pair.SrsIntervalDays)*1.2)))
	case GradeGood:
		pair.SrsIntervalDays = maxInt(1, int(math.Round(float64(pair.SrsIntervalDays)*pair.SrsEase)))
	case GradeEasy:
		pair.SrsEase = pair.SrsEase + 0.15
		pair.SrsIntervalDays = maxInt(1, int(math.Round(float64(pair.SrsIntervalDays)*pair.SrsEase*1.3)))
	default:
		pair.SrsIntervalDays = maxInt(1, int(math.Round(float64(pair.SrsIntervalDays)*pair.SrsEase)))
	}

	pair.SrsState = string(StateReview)
	pair.SrsStep = -1
	pair.SrsDueAt = now.AddDate(0, 0, pair.SrsIntervalDays)
	pair.SrsReps++
	pair.SrsLastReviewedAt = &now
}

func graduateToReview(pair *db.WordPair, now time.Time, intervalDays int) {
	if intervalDays < 1 {
		intervalDays = 1
	}
	pair.SrsState = string(StateReview)
	pair.SrsStep = -1
	pair.SrsIntervalDays = intervalDays
	pair.SrsDueAt = now.AddDate(0, 0, intervalDays)
	pair.SrsLastReviewedAt = &now
}

func clampStep(step int) int {
	if step < 0 {
		return 0
	}
	if step >= len(LearningSteps) {
		return len(LearningSteps) - 1
	}
	return step
}

func maxEase(ease float64) float64 {
	if ease < EaseFloor {
		return EaseFloor
	}
	return ease
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
