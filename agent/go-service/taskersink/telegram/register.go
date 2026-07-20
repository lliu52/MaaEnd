package telegram

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/rs/zerolog/log"
)

var _ maa.TaskerEventSink = &Sink{}

// Register enables the Telegram task sink when a valid runtime configuration is present.
func Register() {
	cfg, source, enabled, err := loadConfig()
	if err != nil {
		log.Warn().Err(err).Str("config_source", source).Msg("Telegram task notifications are disabled")
		return
	}
	if !enabled {
		log.Debug().Msg("Telegram task notifications are not configured")
		return
	}

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", url.PathEscape(cfg.BotToken))
	httpClient := &http.Client{Timeout: 15 * time.Second}
	maa.AgentServerAddTaskerSink(newSink(cfg, httpClient, endpoint))
	log.Info().Str("config_source", source).Msg("Telegram task notifications enabled")
}
