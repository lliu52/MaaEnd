package dailyrewardtelegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

const recoveryMaxAge = 72 * time.Hour

type taskStatus string

const (
	statusPending   taskStatus = "pending"
	statusRunning   taskStatus = "running"
	statusSucceeded taskStatus = "succeeded"
	statusSkipped   taskStatus = "skipped"
	statusFailed    taskStatus = "failed"
)

type persistedState struct {
	RunID     string     `json:"run_id"`
	OwnerPID  int32      `json:"owner_pid"`
	Status    taskStatus `json:"status"`
	TaskID    uint64     `json:"task_id,omitempty"`
	LastNode  string     `json:"last_node,omitempty"`
	KeyInfo   string     `json:"key_info,omitempty"`
	Reason    string     `json:"reason,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type stateStore struct {
	mu   sync.Mutex
	path string
}

func defaultStatePath() string {
	return filepath.Join("debug", "daily-reward-telegram-state.json")
}

func (s *stateStore) save(state persistedState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode daily reward Telegram state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("create daily reward Telegram state directory: %w", err)
	}
	temporary := s.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0600); err != nil {
		return fmt.Errorf("write daily reward Telegram state: %w", err)
	}
	if err := os.Rename(temporary, s.path); err != nil {
		_ = os.Remove(s.path)
		if retryErr := os.Rename(temporary, s.path); retryErr != nil {
			return fmt.Errorf("replace daily reward Telegram state: %w", retryErr)
		}
	}
	return nil
}

func (s *stateStore) clearIfRun(runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := readState(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if state.RunID != runID {
		return nil
	}
	return removeIfExists(s.path)
}

func (s *stateStore) claimRecovery() (persistedState, string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return persistedState{}, "", false, nil
	}
	if err != nil {
		return persistedState{}, "", false, err
	}
	if time.Since(info.ModTime()) > recoveryMaxAge {
		return persistedState{}, "", false, removeIfExists(s.path)
	}

	state, err := readState(s.path)
	if err != nil {
		removeErr := removeIfExists(s.path)
		if removeErr != nil {
			return persistedState{}, "", false, errors.Join(err, removeErr)
		}
		return persistedState{}, "", false, err
	}
	if state.OwnerPID != 0 && state.OwnerPID != int32(os.Getpid()) {
		alive, aliveErr := process.PidExists(state.OwnerPID)
		if aliveErr == nil && alive {
			return persistedState{}, "", false, nil
		}
	}

	claimPath := s.path + ".recovering"
	_ = os.Remove(claimPath)
	if err := os.Rename(s.path, claimPath); err != nil {
		return persistedState{}, "", false, err
	}
	return state, claimPath, true, nil
}

func (s *stateStore) finishRecovery(claimPath string, delivered bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if delivered {
		return removeIfExists(claimPath)
	}
	if _, err := os.Stat(s.path); err == nil {
		return nil
	}
	return os.Rename(claimPath, s.path)
}

func readState(path string) (persistedState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return persistedState{}, err
	}
	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return persistedState{}, fmt.Errorf("decode daily reward Telegram state: %w", err)
	}
	return state, nil
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
