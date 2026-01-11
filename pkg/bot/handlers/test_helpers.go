package handlers

import (
	"bytes"
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

func (m *mockClient) lastMultipartField(t *testing.T, fieldName string) (string, string) {
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
		if part.FormName() == fieldName {
			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("failed to read multipart field: %v", err)
			}
			return string(data), part.FileName()
		}
	}
	t.Fatalf("field %q not found in request", fieldName)
	return "", ""
}

func (m *mockClient) lastRequestBody(t *testing.T) string {
	t.Helper()
	if len(m.requests) == 0 {
		t.Fatalf("expected at least one recorded request")
	}
	return string(m.requests[len(m.requests)-1].body)
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

func newTestUpdate(text string, userID int64) *models.Update {
	return &models.Update{
		Message: &models.Message{
			From: &models.User{
				ID: userID,
			},
			Chat: models.Chat{
				ID: userID,
			},
			Text: text,
		},
	}
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

func newTestCallbackUpdate(data string, userID, chatID int64, messageID int) *models.Update {
	return &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "callback-1",
			From: models.User{ID: userID},
			Data: data,
			Message: models.MaybeInaccessibleMessage{
				Type: models.MaybeInaccessibleMessageTypeMessage,
				Message: &models.Message{
					ID: messageID,
					Chat: models.Chat{
						ID:   chatID,
						Type: models.ChatTypePrivate,
					},
				},
			},
		},
	}
}
