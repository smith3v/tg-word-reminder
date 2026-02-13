package onboarding

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/smith3v/tg-word-reminder/pkg/config"
	"github.com/smith3v/tg-word-reminder/pkg/db"
	"gorm.io/gorm"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

var errEmptyInitVocabulary = errors.New("init vocabulary CSV is empty")

func RefreshInitVocabularyFromConfig() error {
	path := strings.TrimSpace(config.AppConfig.Onboarding.InitVocabularyPath)
	if path == "" {
		return nil
	}
	return RefreshInitVocabularyFromFile(path)
}

func RefreshInitVocabularyFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	rows, err := parseInitVocabularyCSV(f)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return errEmptyInitVocabulary
	}

	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&db.InitVocabulary{}).Error; err != nil {
			return err
		}
		return tx.CreateInBatches(rows, 500).Error
	})
}

func parseInitVocabularyCSV(reader io.Reader) ([]db.InitVocabulary, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, utf8BOM)

	csvReader := csv.NewReader(bytes.NewReader(data))
	csvReader.FieldsPerRecord = -1

	header, err := csvReader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errEmptyInitVocabulary
		}
		return nil, err
	}

	indexByColumn := make(map[string]int, len(header))
	for idx, name := range header {
		indexByColumn[strings.ToLower(strings.TrimSpace(name))] = idx
	}

	requiredCodes := SupportedLanguageCodes()
	missing := make([]string, 0)
	for _, code := range requiredCodes {
		if _, ok := indexByColumn[code]; !ok {
			missing = append(missing, code)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, fmt.Errorf("missing required init vocabulary columns: %s", strings.Join(missing, ", "))
	}

	rows := make([]db.InitVocabulary, 0)
	for {
		record, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		row := db.InitVocabulary{
			EN: csvValue(record, indexByColumn["en"]),
			RU: csvValue(record, indexByColumn["ru"]),
			NL: csvValue(record, indexByColumn["nl"]),
			ES: csvValue(record, indexByColumn["es"]),
			DE: csvValue(record, indexByColumn["de"]),
			FR: csvValue(record, indexByColumn["fr"]),
		}
		if isEmptyInitVocabularyRow(row) {
			continue
		}
		rows = append(rows, row)
	}

	return rows, nil
}

func csvValue(record []string, idx int) string {
	if idx < 0 || idx >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[idx])
}

func isEmptyInitVocabularyRow(row db.InitVocabulary) bool {
	return row.EN == "" && row.RU == "" && row.NL == "" && row.ES == "" && row.DE == "" && row.FR == ""
}
