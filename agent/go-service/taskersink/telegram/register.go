package telegram

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/MaaXYZ/MaaEnd/agent/go-service/pkg/parentwatch"
	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/rs/zerolog/log"
)

var (
	_ maa.TaskerEventSink  = &Sink{}
	_ maa.ContextEventSink = &Sink{}
)

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
	sink := newSink(cfg, httpClient, endpoint)
	maa.AgentServerAddTaskerSink(sink)
	maa.AgentServerAddContextSink(sink)
	parentwatch.RegisterExitHandler(sink.onParentExit)
	log.Info().Str("config_source", source).Msg("Telegram task notifications enabled")
}
