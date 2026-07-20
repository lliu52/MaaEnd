package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

type persistedFailedTask struct {
	Entry string `json:"entry"`
	Log   string `json:"log"`
}

type persistedRunSummary struct {
	Active    bool                  `json:"active"`
	Total     int                   `json:"total"`
	Succeeded int                   `json:"succeeded"`
	Failed    []persistedFailedTask `json:"failed,omitempty"`
}

func defaultStatePath() string {
	// MXU starts a new Agent process for its trailing special-task batch. Both
	// Agent processes have the same MXU parent PID, which keeps concurrent MXU
	// instances isolated while allowing the second Agent to recover the run.
	filename := fmt.Sprintf("telegram-run-summary-%d.json", os.Getppid())
	return filepath.Join("debug", filename)
}

func (s *Sink) persistSummary(summary runSummary) {
	if s.statePath == "" || !summary.active {
		return
	}

	state := persistedRunSummary{
		Active:    summary.active,
		Total:     summary.total,
		Succeeded: summary.succeeded,
		Failed:    make([]persistedFailedTask, 0, len(summary.failed)),
	}
	for _, failed := range summary.failed {
		state.Failed = append(state.Failed, persistedFailedTask{
			Entry: failed.entry,
			Log:   failed.log,
		})
	}

	data, err := json.Marshal(state)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to encode Telegram run summary state")
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.statePath), 0755); err != nil {
		log.Warn().Err(err).Msg("Failed to create Telegram run summary state directory")
		return
	}
	if err := os.WriteFile(s.statePath, data, 0600); err != nil {
		log.Warn().Err(err).Msg("Failed to persist Telegram run summary state")
	}
}

func (s *Sink) loadSummary() runSummary {
	if s.statePath == "" {
		return runSummary{}
	}

	data, err := os.ReadFile(s.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return runSummary{}
	}
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read Telegram run summary state")
		return runSummary{}
	}

	var state persistedRunSummary
	if err := json.Unmarshal(data, &state); err != nil {
		log.Warn().Err(err).Msg("Failed to decode Telegram run summary state")
		return runSummary{}
	}

	summary := runSummary{
		active:    state.Active,
		total:     state.Total,
		succeeded: state.Succeeded,
		failed:    make([]failedTask, 0, len(state.Failed)),
	}
	for _, failed := range state.Failed {
		summary.failed = append(summary.failed, failedTask{
			entry: failed.Entry,
			log:   failed.Log,
		})
	}
	if summary.active {
		log.Info().
			Int("total", summary.total).
			Int("succeeded", summary.succeeded).
			Int("failed", len(summary.failed)).
			Msg("Restored Telegram run summary state")
	}
	return summary
}

func (s *Sink) clearSummary() {
	if s.statePath == "" {
		return
	}
	if err := os.Remove(s.statePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Warn().Err(err).Msg("Failed to clear Telegram run summary state")
	}
}
