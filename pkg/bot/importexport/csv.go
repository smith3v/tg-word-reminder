package importexport

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"gorm.io/gorm"
)

type wordPairInput struct {
	Word1 string
	Word2 string
}

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

const maxDelimiterSampleRecords = 20

var (
	srsNewRankRand = rand.New(rand.NewSource(time.Now().UnixNano()))
	srsNewRankMu   sync.Mutex
)

func randomSrsNewRank() int {
	srsNewRankMu.Lock()
	defer srsNewRankMu.Unlock()
	return srsNewRankRand.Intn(db.SrsNewRankMax) + 1
}

func ParseVocabularyCSV(data []byte) ([]wordPairInput, int, error) {
	data = bytes.TrimPrefix(data, utf8BOM)
	delimiter := detectCSVDelimiter(data)

	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1

	var pairs []wordPairInput
	skipped := 0
	checkedHeader := false

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, skipped, err
		}
		if isEmptyCSVRecord(record) {
			skipped++
			continue
		}
		if !checkedHeader {
			checkedHeader = true
			if isHeaderRecord(record) {
				continue
			}
		}
		if len(record) < 2 {
			skipped++
			continue
		}
		word1 := strings.TrimSpace(record[0])
		word2 := strings.TrimSpace(record[1])
		if word1 == "" || word2 == "" {
			skipped++
			continue
		}
		pairs = append(pairs, wordPairInput{
			Word1: word1,
			Word2: word2,
		})
	}

	return pairs, skipped, nil
}

func detectCSVDelimiter(data []byte) rune {
	candidates := []rune{',', '\t', ';'}
	bestDelimiter := candidates[0]
	bestScore := -1

	for _, delimiter := range candidates {
		score, err := scoreDelimiter(data, delimiter, maxDelimiterSampleRecords)
		if err != nil {
			continue
		}
		if score > bestScore {
			bestScore = score
			bestDelimiter = delimiter
		}
	}

	if bestScore <= 0 {
		return ','
	}
	return bestDelimiter
}

func scoreDelimiter(data []byte, delimiter rune, maxRecords int) (int, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1

	counts := make(map[int]int)
	recordsSeen := 0

	for recordsSeen < maxRecords {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, err
		}
		if isEmptyCSVRecord(record) {
			continue
		}
		recordsSeen++

		if len(record) < 2 {
			continue
		}
		counts[len(record)]++
	}

	best := 0
	for _, score := range counts {
		if score > best {
			best = score
		}
	}
	return best, nil
}

func isEmptyCSVRecord(record []string) bool {
	for _, field := range record {
		if strings.TrimSpace(field) != "" {
			return false
		}
	}
	return true
}

func isHeaderRecord(record []string) bool {
	if len(record) < 2 {
		return false
	}
	left := strings.ToLower(strings.TrimSpace(record[0]))
	right := strings.ToLower(strings.TrimSpace(record[1]))
	headers := map[string]struct{}{
		"word1":  {},
		"word2":  {},
		"source": {},
		"target": {},
	}
	_, leftOK := headers[left]
	_, rightOK := headers[right]
	return leftOK && rightOK
}

func UpsertWordPairs(userID int64, pairs []wordPairInput) (int, int, error) {
	inserted := 0
	updated := 0

	if len(pairs) == 0 {
		return inserted, updated, nil
	}

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		for _, pair := range pairs {
			result := tx.Model(&db.WordPair{}).
				Where("user_id = ? AND word1 = ?", userID, pair.Word1).
				Update("word2", pair.Word2)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				updated++
				continue
			}

			newPair := db.WordPair{
				UserID:          userID,
				Word1:           pair.Word1,
				Word2:           pair.Word2,
				SrsState:        "new",
				SrsDueAt:        now,
				SrsIntervalDays: 0,
				SrsEase:         2.5,
				SrsStep:         0,
				SrsNewRank:      randomSrsNewRank(),
			}
			if err := tx.Create(&newPair).Error; err != nil {
				return err
			}
			inserted++
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	return inserted, updated, nil
}

func BuildExportCSV(pairs []db.WordPair) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := buf.Write(utf8BOM); err != nil {
		return nil, err
	}

	writer := csv.NewWriter(&buf)
	writer.UseCRLF = true

	for _, pair := range pairs {
		if err := writer.Write([]string{pair.Word1, pair.Word2}); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ExportFilename(now time.Time) string {
	return fmt.Sprintf("vocabulary-%s.csv", now.Format("20060102"))
}

func SortPairsForExport(pairs []db.WordPair) {
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Word1 == pairs[j].Word1 {
			return pairs[i].ID < pairs[j].ID
		}
		return pairs[i].Word1 < pairs[j].Word1
	})
}
