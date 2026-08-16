package dailyrewardtelegram

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestBotClientSend(t *testing.T) {
	var received url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		received, err = url.ParseQuery(string(body))
		if err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := &botClient{
		httpClient: server.Client(),
		endpoint:   server.URL,
		config: Config{
			ChatID:              "1234",
			MessageThreadID:     42,
			DisableNotification: true,
		},
	}
	if err := client.Send(context.Background(), "test message"); err != nil {
		t.Fatal(err)
	}

	if received.Get("chat_id") != "1234" || received.Get("text") != "test message" {
		t.Fatalf("unexpected form: %#v", received)
	}
	if received.Get("message_thread_id") != "42" || received.Get("disable_notification") != "true" {
		t.Fatalf("unexpected optional form values: %#v", received)
	}
}
