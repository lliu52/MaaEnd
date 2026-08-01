// Package hangtest contains an intentionally blocking action for validating
// external no-log watchdogs. It must only be shipped from the test branch.
package hangtest

import (
	"encoding/json"
	"os"
	"runtime"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/rs/zerolog/log"
)

const (
	actionName            = "TelegramNoLogHangTestAction"
	defaultSilenceSeconds = 300
)

type params struct {
	SilenceSeconds int `json:"silence_seconds"`
}

type Action struct{}

var _ maa.CustomActionRunner = &Action{}

// Run deliberately never returns. It emits all diagnostics before becoming
// silent so OneDragon's no-log watchdog can terminate MaaEnd and exercise the
// Telegram parent-exit notification path.
func (a *Action) Run(_ *maa.Context, arg *maa.CustomActionArg) bool {
	settings := params{SilenceSeconds: defaultSilenceSeconds}
	if arg != nil && arg.CustomActionParam != "" {
		if err := json.Unmarshal([]byte(arg.CustomActionParam), &settings); err != nil {
			log.Error().
				Err(err).
				Str("component", "telegram-hang-test").
				Str("custom_action_param", arg.CustomActionParam).
				Msg("failed to parse hang-test parameters")
			return false
		}
	}
	if settings.SilenceSeconds <= 0 {
		settings.SilenceSeconds = defaultSilenceSeconds
	}

	event := log.Warn().
		Str("component", "telegram-hang-test").
		Int("pid", os.Getpid()).
		Int("parent_pid", os.Getppid()).
		Str("go_version", runtime.Version()).
		Int("minimum_silence_seconds", settings.SilenceSeconds)
	if arg != nil {
		event = event.
			Int64("task_id", arg.TaskID).
			Str("current_task", arg.CurrentTaskName).
			Str("custom_action", arg.CustomActionName)
	}
	event.Msg("TEST ONLY: entering intentional no-log hang; action will not return")

	// Do not log from this point onward. After the requested minimum silence,
	// remain blocked so a watchdog threshold equal to 300 seconds is reliable.
	time.Sleep(time.Duration(settings.SilenceSeconds) * time.Second)
	select {}
}
