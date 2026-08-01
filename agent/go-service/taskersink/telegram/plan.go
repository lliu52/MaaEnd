package telegram

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

type mxuConfig struct {
	Instances            []mxuInstance `json:"instances"`
	LastActiveInstanceID string        `json:"lastActiveInstanceId"`
}

type mxuInstance struct {
	ID             string    `json:"id"`
	ControllerName string    `json:"controllerName"`
	Tasks          []mxuTask `json:"tasks"`
}

type mxuTask struct {
	TaskName            string          `json:"taskName"`
	CustomName          string          `json:"customName"`
	Enabled             bool            `json:"enabled"`
	EnabledByController map[string]bool `json:"enabledByController"`
}

func loadTaskPlan() []plannedTask {
	for _, path := range taskPlanPaths() {
		plan, err := loadTaskPlanFile(path)
		if err == nil {
			if len(plan) > 0 {
				log.Info().Str("path", path).Int("tasks", len(plan)).Msg("Loaded Telegram task plan")
			}
			return plan
		}
		if !os.IsNotExist(err) {
			log.Warn().Err(err).Str("path", path).Msg("Failed to load Telegram task plan")
		}
	}
	return nil
}

func taskPlanPaths() []string {
	paths := []string{filepath.Join("config", "mxu-MaaEnd.json")}
	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		paths = append(paths,
			filepath.Join(dir, "config", "mxu-MaaEnd.json"),
			filepath.Join(filepath.Dir(dir), "config", "mxu-MaaEnd.json"),
		)
	}
	return paths
}

func loadTaskPlanFile(path string) ([]plannedTask, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg mxuConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.Instances) == 0 {
		return nil, nil
	}
	instance := &cfg.Instances[0]
	for i := range cfg.Instances {
		if cfg.Instances[i].ID == cfg.LastActiveInstanceID {
			instance = &cfg.Instances[i]
			break
		}
	}
	plan := make([]plannedTask, 0, len(instance.Tasks))
	for _, task := range instance.Tasks {
		if !task.Enabled || isControlTask(task.TaskName) {
			continue
		}
		if enabled, exists := task.EnabledByController[instance.ControllerName]; exists && !enabled {
			continue
		}
		name := strings.TrimSpace(task.CustomName)
		if name == "" {
			name = task.TaskName
		}
		plan = append(plan, plannedTask{name: name, entry: task.TaskName, state: taskPending})
	}
	return plan, nil
}

func isControlTask(entry string) bool {
	if strings.HasPrefix(entry, "__PRETASK__") {
		return true
	}
	_, found := shutdownTaskEntries[entry]
	return found
}
