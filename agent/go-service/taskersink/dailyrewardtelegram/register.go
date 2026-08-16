package dailyrewardtelegram

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/rs/zerolog/log"
)

var (
	_ maa.TaskerEventSink  = &Sink{}
	_ maa.ContextEventSink = &Sink{}
)

// Register enables DailyRewards Telegram notifications when configuration exists.
func Register() {
	cfg, source, configured, err := loadConfig()
	if err != nil {
		log.Warn().Err(err).Str("config_source", source).Msg("DailyRewards Telegram notifications are disabled")
		return
	}
	if !configured {
		log.Debug().Msg("DailyRewards Telegram notifications are not configured")
		return
	}

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", url.PathEscape(cfg.BotToken))
	client := &botClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		endpoint:   endpoint,
		config:     cfg,
	}
	sink := newSink(client, &stateStore{path: defaultStatePath()}, dailyRewardEnabled())
	maa.AgentServerAddTaskerSink(sink)
	maa.AgentServerAddContextSink(sink)
	log.Info().Str("config_source", source).Msg("DailyRewards-only Telegram notifications enabled")
}
