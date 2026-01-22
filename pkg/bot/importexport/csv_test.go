package importexport

import (
	"bytes"
	"strings"
	"testing"

	dbpkg "github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
)

func TestDetectCSVDelimiter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected rune
	}{
		{"comma", "word1,word2\nhello,world\n", ','},
		{"tab", "word1\tword2\nhello\tworld\n", '\t'},
		{"semicolon", "word1;word2\nhello;world\n", ';'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectCSVDelimiter([]byte(tt.input))
			if got != tt.expected {
				t.Fatalf("expected %q delimiter, got %q", tt.expected, got)
			}
		})
	}
}

func TestParseVocabularyCSV(t *testing.T) {
	data := strings.Join([]string{
		"word1;word2;extra",
		"hola;adios;note",
		"uno;;missing-word2",
		";missing-word1",
		"",
		"bonjour;hello",
	}, "\n")

	pairs, skipped, err := ParseVocabularyCSV([]byte(data))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
	if pairs[0].Word1 != "hola" || pairs[0].Word2 != "adios" {
		t.Fatalf("unexpected first pair: %+v", pairs[0])
	}
	if pairs[1].Word1 != "bonjour" || pairs[1].Word2 != "hello" {
		t.Fatalf("unexpected second pair: %+v", pairs[1])
	}
	if skipped != 2 {
		t.Fatalf("expected 2 skipped rows, got %d", skipped)
	}
}

func TestUpsertWordPairs(t *testing.T) {
	testutil.SetupTestDB(t)

	if err := dbpkg.DB.Create(&dbpkg.WordPair{
		UserID: 111,
		Word1:  "hola",
		Word2:  "adios",
	}).Error; err != nil {
		t.Fatalf("failed to seed word pair: %v", err)
	}

	inserted, updated, err := UpsertWordPairs(111, []wordPairInput{
		{Word1: "hola", Word2: "bonjour"},
		{Word1: "ciao", Word2: "hello"},
	})
	if err != nil {
		t.Fatalf("unexpected upsert error: %v", err)
	}
	if inserted != 1 || updated != 1 {
		t.Fatalf("expected 1 insert and 1 update, got %d inserts and %d updates", inserted, updated)
	}

	var pairs []dbpkg.WordPair
	if err := dbpkg.DB.Where("user_id = ?", 111).Order("word1 ASC").Find(&pairs).Error; err != nil {
		t.Fatalf("failed to load pairs: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
	if pairs[0].Word1 != "ciao" || pairs[0].Word2 != "hello" {
		t.Fatalf("unexpected pair[0]: %+v", pairs[0])
	}
	if pairs[1].Word1 != "hola" || pairs[1].Word2 != "bonjour" {
		t.Fatalf("unexpected pair[1]: %+v", pairs[1])
	}
	if pairs[0].SrsState != "new" {
		t.Fatalf("expected new pair to have SRS state, got %+v", pairs[0])
	}
	if pairs[0].SrsIntervalDays != 0 || pairs[0].SrsStep != 0 {
		t.Fatalf("expected default SRS interval/step, got %+v", pairs[0])
	}
	if pairs[0].SrsEase != 2.5 {
		t.Fatalf("expected default SRS ease 2.5, got %+v", pairs[0])
	}
	if pairs[0].SrsDueAt.IsZero() {
		t.Fatalf("expected default SRS due date set, got %+v", pairs[0])
	}
	if pairs[0].SrsNewRank == 0 {
		t.Fatalf("expected new pair to have SRS new rank set, got %+v", pairs[0])
	}
}

func TestBuildExportCSV(t *testing.T) {
	pairs := []dbpkg.WordPair{
		{Word1: "hello", Word2: "world"},
		{Word1: "comma,word", Word2: `quote"word`},
	}

	data, err := BuildExportCSV(pairs)
	if err != nil {
		t.Fatalf("unexpected export error: %v", err)
	}
	if !bytes.HasPrefix(data, utf8BOM) {
		t.Fatalf("expected UTF-8 BOM prefix")
	}

	output := string(data[len(utf8BOM):])
	if !strings.HasPrefix(output, "hello,world\r\n") {
		t.Fatalf("expected first row with CRLF, got %q", output)
	}
	if !strings.Contains(output, "\"comma,word\",\"quote\"\"word\"") {
		t.Fatalf("expected quoted fields, got %q", output)
	}
	if !strings.Contains(output, "\r\n") {
		t.Fatalf("expected CRLF line endings")
	}
}
