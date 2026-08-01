package hangtest

import (
	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/rs/zerolog/log"
)

func Register() {
	maa.AgentServerRegisterCustomAction(actionName, &Action{})
	log.Warn().
		Str("component", "telegram-hang-test").
		Str("custom_action", actionName).
		Msg("TEST ONLY: intentional no-log hang action registered")
}
