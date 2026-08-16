package dailyrewardtelegram

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const dailyRewardTaskName = "DailyRewards"

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
	Enabled             bool            `json:"enabled"`
	EnabledByController map[string]bool `json:"enabledByController"`
}

func dailyRewardEnabled() bool {
	for _, path := range taskPlanCandidates() {
		enabled, found, err := dailyRewardEnabledInFile(path)
		if err == nil && found {
			return enabled
		}
	}
	return false
}

func dailyRewardEnabledInFile(path string) (bool, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false, err
	}

	var cfg mxuConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false, false, err
	}
	if len(cfg.Instances) == 0 {
		return false, false, nil
	}

	instance := &cfg.Instances[0]
	for i := range cfg.Instances {
		if cfg.Instances[i].ID == cfg.LastActiveInstanceID {
			instance = &cfg.Instances[i]
			break
		}
	}
	for _, task := range instance.Tasks {
		if task.TaskName != dailyRewardTaskName {
			continue
		}
		enabled := task.Enabled
		if controllerEnabled, exists := task.EnabledByController[instance.ControllerName]; exists {
			enabled = enabled && controllerEnabled
		}
		return enabled, true, nil
	}
	return false, false, nil
}

func taskPlanCandidates() []string {
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
