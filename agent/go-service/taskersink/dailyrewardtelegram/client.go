package dailyrewardtelegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
)

type photoSender interface {
	SendPhoto(context.Context, []byte, string, string) error
}

type botClient struct {
	httpClient *http.Client
	endpoint   string
	config     Config
}

type botResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func newBotClient(cfg Config, httpClient *http.Client) *botClient {
	return &botClient{
		httpClient: httpClient,
		endpoint:   fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", url.PathEscape(cfg.BotToken)),
		config:     cfg,
	}
}

func (c *botClient) SendPhoto(ctx context.Context, photo []byte, filename, caption string) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("chat_id", c.config.ChatID); err != nil {
		return fmt.Errorf("write Telegram chat_id: %w", err)
	}
	if caption != "" {
		if err := writer.WriteField("caption", caption); err != nil {
			return fmt.Errorf("write Telegram caption: %w", err)
		}
	}
	if c.config.MessageThreadID != 0 {
		if err := writer.WriteField("message_thread_id", strconv.FormatInt(c.config.MessageThreadID, 10)); err != nil {
			return fmt.Errorf("write Telegram message_thread_id: %w", err)
		}
	}
	if c.config.DisableNotification {
		if err := writer.WriteField("disable_notification", "true"); err != nil {
			return fmt.Errorf("write Telegram disable_notification: %w", err)
		}
	}
	part, err := writer.CreateFormFile("photo", filename)
	if err != nil {
		return fmt.Errorf("create Telegram photo part: %w", err)
	}
	if _, err := part.Write(photo); err != nil {
		return fmt.Errorf("write Telegram photo: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish Telegram request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, &body)
	if err != nil {
		return fmt.Errorf("create Telegram request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send Telegram request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read Telegram response: %w", err)
	}
	var result botResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return fmt.Errorf("parse Telegram response (HTTP %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !result.OK {
		if result.Description == "" {
			result.Description = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("Telegram API rejected photo (HTTP %d): %s", resp.StatusCode, result.Description)
	}
	return nil
}
