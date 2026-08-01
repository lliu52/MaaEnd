package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/rs/zerolog/log"
)

const interruptedRecoveryWindow = 30 * time.Minute

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
	summary, err := loadSummaryFile(s.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return runSummary{}
	}
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read Telegram run summary state")
		return runSummary{}
	}
	if summary.active {
		log.Info().Int("total", summary.total).Int("succeeded", summary.succeeded).
			Int("failed", len(summary.failed)).Msg("Restored Telegram run summary state")
	}
	return summary
}

func loadSummaryFile(path string) (runSummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return runSummary{}, err
	}
	var state persistedRunSummary
	if err := json.Unmarshal(data, &state); err != nil {
		return runSummary{}, err
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
	return summary, nil
}

type staleSummaryCandidate struct {
	path    string
	modTime time.Time
	summary runSummary
}

// recoverInterruptedSummary handles watchdogs that kill the complete MaaEnd
// process tree. The old Agent cannot run an exit hook in that case, but its
// atomic state file survives and the replacement Agent can send the snapshot.
func (s *Sink) recoverInterruptedSummary() {
	if s.statePath == "" {
		return
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(s.statePath), "telegram-run-summary-*.json"))
	if err != nil {
		log.Warn().Err(err).Msg("Failed to scan interrupted Telegram run states")
		return
	}
	now := time.Now()
	candidates := make([]staleSummaryCandidate, 0, len(paths))
	for _, path := range paths {
		if filepath.Clean(path) == filepath.Clean(s.statePath) {
			continue
		}
		info, statErr := os.Stat(path)
		if statErr != nil || now.Sub(info.ModTime()) > interruptedRecoveryWindow {
			continue
		}
		summary, loadErr := loadSummaryFile(path)
		if loadErr != nil {
			log.Warn().Err(loadErr).Str("state_path", path).Msg("Failed to decode stale Telegram run state")
			continue
		}
		if !summary.active || !hasRunningTask(summary) {
			continue
		}
		candidates = append(candidates, staleSummaryCandidate{path: path, modTime: info.ModTime(), summary: summary})
	}
	if len(candidates) == 0 {
		return
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].modTime.After(candidates[j].modTime) })
	candidate := candidates[0]
	log.Warn().
		Str("component", "telegram").
		Str("stale_state_path", candidate.path).
		Time("state_updated_at", candidate.modTime).
		Int("configured_tasks", len(candidate.summary.tasks)).
		Int("started_tasks", candidate.summary.total).
		Int("succeeded_tasks", candidate.summary.succeeded).
		Int("failed_tasks", len(candidate.summary.failed)).
		Msg("recovered interrupted run after complete process-tree termination; sending Telegram snapshot")
	s.send(formatRecoveredSummary(candidate.summary))
	if removeErr := os.Remove(candidate.path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		log.Warn().Err(removeErr).Str("state_path", candidate.path).Msg("Failed to clear recovered Telegram run state")
		return
	}
	log.Info().Str("component", "telegram").Str("state_path", candidate.path).
		Msg("recovered interrupted Telegram snapshot send attempt finished; stale state cleared")
}

func hasRunningTask(summary runSummary) bool {
	for _, task := range summary.tasks {
		if task.state == taskRunning {
			return true
		}
	}
	return false
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
