package importexport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/smith3v/tg-word-reminder/pkg/config"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

type recordedRequest struct {
	path        string
	method      string
	contentType string
	body        []byte
}

type mockClient struct {
	requests []recordedRequest
	response string
}

func newMockClient() *mockClient {
	return &mockClient{
		response: `{"ok":true,"result":{}}`,
	}
}

func (m *mockClient) Do(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	if err := req.Body.Close(); err != nil {
		return nil, fmt.Errorf("failed to close request body: %w", err)
	}
	m.requests = append(m.requests, recordedRequest{
		path:        req.URL.Path,
		method:      req.Method,
		contentType: req.Header.Get("Content-Type"),
		body:        body,
	})

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(m.response)),
		Header:     make(http.Header),
	}
	return resp, nil
}

func (m *mockClient) lastMessageText(t *testing.T) string {
	t.Helper()
	if len(m.requests) == 0 {
		t.Fatalf("expected at least one recorded request")
	}
	req := m.requests[len(m.requests)-1]

	mediaType, params, err := mime.ParseMediaType(req.contentType)
	if err != nil {
		t.Fatalf("failed to parse media type: %v", err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		t.Fatalf("unexpected media type: %s", mediaType)
	}

	reader := multipart.NewReader(bytes.NewReader(req.body), params["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read multipart part: %v", err)
		}
		if part.FormName() == "text" {
			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("failed to read text part: %v", err)
			}
			return string(data)
		}
	}
	t.Fatalf("text field not found in request")
	return ""
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestTelegramBot(t *testing.T, client *mockClient) *telegram.Bot {
	t.Helper()
	b, err := telegram.New("test-token",
		telegram.WithSkipGetMe(),
		telegram.WithHTTPClient(time.Second, client),
	)
	if err != nil {
		t.Fatalf("failed to create test bot: %v", err)
	}
	return b
}

func newTestDocumentUpdate(fileName, fileID string, userID int64) *models.Update {
	return &models.Update{
		Message: &models.Message{
			From: &models.User{
				ID: userID,
			},
			Chat: models.Chat{
				ID:   userID,
				Type: models.ChatTypePrivate,
			},
			Document: &models.Document{
				FileID:   fileID,
				FileName: fileName,
			},
		},
	}
}

func TestHandleDocumentImportRejectsNonCSVUpload(t *testing.T) {
	logger.SetLogLevel(logger.ERROR)

	client := newMockClient()
	b := newTestTelegramBot(t, client)
	update := newTestDocumentUpdate("words.txt", "file-1", 101)

	HandleDocumentImport(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "not a CSV") {
		t.Fatalf("expected non-CSV warning, got %q", got)
	}
}

func TestHandleDocumentImportImportsCSV(t *testing.T) {
	testutil.SetupTestDB(t)
	logger.SetLogLevel(logger.ERROR)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := "word1,word2\nhola,adios\n"
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	originalConfig := config.AppConfig
	t.Cleanup(func() {
		config.AppConfig = originalConfig
	})
	config.AppConfig.Telegram.Token = "test-token"

	client := newMockClient()
	client.response = `{"ok":true,"result":{"file_path":"files/test.csv"}}`
	b := newTestTelegramBot(t, client)
	update := newTestDocumentUpdate("words.csv", "file-2", 500)

	HandleDocumentImport(context.Background(), b, update)

	got := client.lastMessageText(t)
	if !strings.Contains(got, "Imported 1 new pairs") {
		t.Fatalf("expected import confirmation, got %q", got)
	}
}
