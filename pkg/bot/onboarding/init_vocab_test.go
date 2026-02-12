package onboarding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
)

func TestRefreshInitVocabularyFromFile(t *testing.T) {
	testutil.SetupTestDB(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "multilang.csv")
	content := "en,ru,nl,es,de,fr\nhello,привет,hallo,hola,hallo,salut\nbye,пока,dag,adios,tschuss,au revoir\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write csv: %v", err)
	}

	if err := RefreshInitVocabularyFromFile(path); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	var count int64
	if err := db.DB.Model(&db.InitVocabulary{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count init rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 rows, got %d", count)
	}
}

func TestRefreshInitVocabularyFromFileMissingRequiredColumn(t *testing.T) {
	testutil.SetupTestDB(t)

	seed := db.InitVocabulary{EN: "a", RU: "b", NL: "c", ES: "d", DE: "e", FR: "f"}
	if err := db.DB.Create(&seed).Error; err != nil {
		t.Fatalf("failed to seed row: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.csv")
	content := "en,ru,nl,es,de\nhello,привет,hallo,hola,hallo\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write csv: %v", err)
	}

	err := RefreshInitVocabularyFromFile(path)
	if err == nil {
		t.Fatalf("expected refresh to fail")
	}
	if !strings.Contains(err.Error(), "fr") {
		t.Fatalf("expected missing column message, got %v", err)
	}

	var count int64
	if err := db.DB.Model(&db.InitVocabulary{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected existing rows to remain, got %d", count)
	}
}
