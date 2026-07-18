package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type botClient struct {
	httpClient *http.Client
	endpoint   string
	config     Config
}

type botResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func newBotClient(httpClient *http.Client, endpoint string, cfg Config) *botClient {
	return &botClient{
		httpClient: httpClient,
		endpoint:   endpoint,
		config:     cfg,
	}
}

func (c *botClient) send(ctx context.Context, text string) error {
	form := url.Values{
		"chat_id": {c.config.ChatID},
		"text":    {text},
	}
	if c.config.MessageThreadID != 0 {
		form.Set("message_thread_id", strconv.FormatInt(c.config.MessageThreadID, 10))
	}
	if c.config.DisableNotification {
		form.Set("disable_notification", "true")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create Telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send Telegram request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read Telegram response: %w", err)
	}

	var result botResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse Telegram response (HTTP %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !result.OK {
		if result.Description == "" {
			result.Description = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("Telegram API rejected message (HTTP %d): %s", resp.StatusCode, result.Description)
	}

	return nil
}
