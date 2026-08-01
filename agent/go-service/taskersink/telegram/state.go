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

type persistedPlannedTask struct {
	Name    string    `json:"name"`
	Entry   string    `json:"entry"`
	TaskID  uint64    `json:"task_id,omitempty"`
	State   taskState `json:"state"`
	KeyInfo string    `json:"key_info,omitempty"`
}

type persistedRunSummary struct {
	Active    bool                   `json:"active"`
	Total     int                    `json:"total"`
	Succeeded int                    `json:"succeeded"`
	Failed    []persistedFailedTask  `json:"failed,omitempty"`
	Tasks     []persistedPlannedTask `json:"tasks,omitempty"`
}

func defaultStatePath() string {
	filename := fmt.Sprintf("telegram-run-summary-%d.json", os.Getppid())
	return filepath.Join("debug", filename)
}

func (s *Sink) persistSummary(summary runSummary) {
	if s.statePath == "" || !summary.active {
		return
	}
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	state := persistedRunSummary{
		Active: summary.active, Total: summary.total, Succeeded: summary.succeeded,
		Failed: make([]persistedFailedTask, 0, len(summary.failed)),
		Tasks:  make([]persistedPlannedTask, 0, len(summary.tasks)),
	}
	for _, failed := range summary.failed {
		state.Failed = append(state.Failed, persistedFailedTask{Entry: failed.entry, Log: failed.log})
	}
	for _, task := range summary.tasks {
		state.Tasks = append(state.Tasks, persistedPlannedTask{
			Name: task.name, Entry: task.entry, TaskID: task.taskID, State: task.state, KeyInfo: task.keyInfo,
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
	temporary := s.statePath + ".tmp"
	if err := os.WriteFile(temporary, data, 0600); err != nil {
		log.Warn().Err(err).Msg("Failed to persist Telegram run summary state")
		return
	}
	if err := os.Rename(temporary, s.statePath); err != nil {
		_ = os.Remove(s.statePath)
		if retryErr := os.Rename(temporary, s.statePath); retryErr != nil {
			log.Warn().Err(retryErr).Msg("Failed to replace Telegram run summary state")
		}
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
		active: state.Active, total: state.Total, succeeded: state.Succeeded,
		failed: make([]failedTask, 0, len(state.Failed)),
		tasks:  make([]plannedTask, 0, len(state.Tasks)),
	}
	for _, failed := range state.Failed {
		summary.failed = append(summary.failed, failedTask{entry: failed.Entry, log: failed.Log})
	}
	for _, task := range state.Tasks {
		summary.tasks = append(summary.tasks, plannedTask{
			name: task.Name, entry: task.Entry, taskID: task.TaskID, state: task.State, keyInfo: task.KeyInfo,
		})
	}
	if summary.active {
		log.Info().Int("total", summary.total).Int("succeeded", summary.succeeded).
			Int("failed", len(summary.failed)).Msg("Restored Telegram run summary state")
	}
	return summary
}

func (s *Sink) clearSummary() {
	if s.statePath == "" {
		return
	}
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if err := os.Remove(s.statePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Warn().Err(err).Msg("Failed to clear Telegram run summary state")
	}
}
