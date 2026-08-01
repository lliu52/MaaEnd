package telegram

import (
	"encoding/json"
	"fmt"
	"strings"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

const maxFailureFocusRunes = 500

// OnNodePipelineNode records progress from push events. It must not make a
// reverse AgentServer call: this callback may run while Tasker IPC is blocked.
func (s *Sink) OnNodePipelineNode(_ *maa.Context, event maa.EventStatus, detail maa.NodePipelineNodeDetail) {
	if detail.TaskID == 0 || detail.Name == "" {
		return
	}
	keyInfo := detail.Name
	if event == maa.EventStatusFailed {
		keyInfo = formatFailureKeyInfo(detail.Name, detail.Focus)
	}
	s.mu.Lock()
	previous := s.lastNodeByTask[detail.TaskID]
	if event == maa.EventStatusFailed || !isFailureKeyInfo(previous) {
		s.lastNodeByTask[detail.TaskID] = keyInfo
	}
	for i := range s.summary.tasks {
		if s.summary.tasks[i].taskID == detail.TaskID && s.summary.tasks[i].state == taskRunning {
			if event == maa.EventStatusFailed || !isFailureKeyInfo(s.summary.tasks[i].keyInfo) {
				s.summary.tasks[i].keyInfo = keyInfo
			}
			break
		}
	}
	summary := cloneSummary(s.summary)
	s.mu.Unlock()
	if summary.active {
		s.persistSummary(summary)
	}
}

func formatFailureKeyInfo(node string, focus any) string {
	info := fmt.Sprintf("失败节点：%s", node)
	if focusText := compactFocus(focus); focusText != "" {
		info += "\n关键日志：" + focusText
	}
	return info
}

func compactFocus(focus any) string {
	if focus == nil {
		return ""
	}
	var text string
	switch value := focus.(type) {
	case string:
		text = value
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		text = string(data)
	}
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > maxFailureFocusRunes {
		text = string(runes[:maxFailureFocusRunes]) + "…"
	}
	return text
}

func isFailureKeyInfo(info string) bool {
	return strings.HasPrefix(info, "失败节点：")
}

func (s *Sink) OnNodeNextList(_ *maa.Context, _ maa.EventStatus, _ maa.NodeNextListDetail)       {}
func (s *Sink) OnNodeRecognition(_ *maa.Context, _ maa.EventStatus, _ maa.NodeRecognitionDetail) {}
func (s *Sink) OnNodeAction(_ *maa.Context, _ maa.EventStatus, _ maa.NodeActionDetail)           {}
func (s *Sink) OnNodeRecognitionNode(_ *maa.Context, _ maa.EventStatus, _ maa.NodeRecognitionNodeDetail) {
}
func (s *Sink) OnNodeActionNode(_ *maa.Context, _ maa.EventStatus, _ maa.NodeActionNodeDetail) {}
