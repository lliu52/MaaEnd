package dailyrewardtelegram

import (
	"net/http"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/rs/zerolog/log"
)

// Register always installs the pipeline action. Missing or invalid Telegram configuration
// disables delivery without breaking the DailyRewards task.
func Register() {
	action := &Action{}
	cfg, source, configured, err := loadConfig()
	if err != nil {
		log.Warn().Err(err).Str("config_source", source).Msg("Daily reward Telegram screenshot delivery is disabled")
	} else if !configured {
		log.Debug().Msg("Daily reward Telegram screenshot delivery is not configured")
	} else {
		action.sender = newBotClient(cfg, &http.Client{Timeout: 25 * time.Second})
		log.Info().Str("config_source", source).Msg("Daily reward Telegram screenshot delivery enabled")
	}
	maa.AgentServerRegisterCustomAction(actionName, action)
}
