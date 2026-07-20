package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBotClientSend(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		if got := r.Form.Get("chat_id"); got != "-100123" {
			t.Errorf("chat_id = %q, want %q", got, "-100123")
		}
		if got := r.Form.Get("message_thread_id"); got != "42" {
			t.Errorf("message_thread_id = %q, want %q", got, "42")
		}
		if got := r.Form.Get("disable_notification"); got != "true" {
			t.Errorf("disable_notification = %q, want true", got)
		}
		if got := r.Form.Get("text"); got != "hello" {
			t.Errorf("text = %q, want hello", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	cfg := Config{
		ChatID:              "-100123",
		MessageThreadID:     42,
		DisableNotification: true,
	}
	client := newBotClient(server.Client(), server.URL, cfg)
	if err := client.send(context.Background(), "hello"); err != nil {
		t.Fatalf("send() error = %v", err)
	}
}

func TestBotClientReportsAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"description":"chat not found"}`))
	}))
	defer server.Close()

	client := newBotClient(server.Client(), server.URL, Config{ChatID: "missing"})
	if err := client.send(context.Background(), "hello"); err == nil {
		t.Fatal("send() error = nil, want Telegram API error")
	}
}
