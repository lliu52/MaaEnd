package dailyrewardtelegram

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBotClientSendPhoto(t *testing.T) {
	t.Helper()
	photo := []byte("jpeg-data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm() error = %v", err)
		}
		if got := r.FormValue("chat_id"); got != "12345" {
			t.Errorf("chat_id = %q, want %q", got, "12345")
		}
		if got := r.FormValue("caption"); got != screenshotCaption {
			t.Errorf("caption = %q, want %q", got, screenshotCaption)
		}
		if got := r.FormValue("message_thread_id"); got != "42" {
			t.Errorf("message_thread_id = %q, want %q", got, "42")
		}
		if got := r.FormValue("disable_notification"); got != "true" {
			t.Errorf("disable_notification = %q, want true", got)
		}
		file, _, err := r.FormFile("photo")
		if err != nil {
			t.Fatalf("FormFile(photo) error = %v", err)
		}
		defer file.Close()
		gotPhoto, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("ReadAll(photo) error = %v", err)
		}
		if string(gotPhoto) != string(photo) {
			t.Errorf("photo = %q, want %q", gotPhoto, photo)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := newBotClient(Config{
		BotToken:            "token",
		ChatID:              "12345",
		MessageThreadID:     42,
		DisableNotification: true,
	}, server.Client())
	client.endpoint = server.URL

	if err := client.SendPhoto(context.Background(), photo, "daily.jpg", screenshotCaption); err != nil {
		t.Fatalf("SendPhoto() error = %v", err)
	}
}

func TestBotClientReportsTelegramError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"description":"bad chat"}`))
	}))
	defer server.Close()

	client := newBotClient(Config{BotToken: "token", ChatID: "12345"}, server.Client())
	client.endpoint = server.URL
	if err := client.SendPhoto(context.Background(), []byte("photo"), "daily.jpg", "caption"); err == nil {
		t.Fatal("SendPhoto() error = nil, want Telegram API error")
	}
}
