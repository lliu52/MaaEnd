package telegram

import maa "github.com/MaaXYZ/maa-framework-go/v4"

// OnNodePipelineNode records progress from push events. It must not make a
// reverse AgentServer call: this callback may run while Tasker IPC is blocked.
func (s *Sink) OnNodePipelineNode(_ *maa.Context, _ maa.EventStatus, detail maa.NodePipelineNodeDetail) {
	if detail.TaskID == 0 || detail.Name == "" {
		return
	}
	s.mu.Lock()
	s.lastNodeByTask[detail.TaskID] = detail.Name
	for i := range s.summary.tasks {
		if s.summary.tasks[i].taskID == detail.TaskID && s.summary.tasks[i].state == taskRunning {
			s.summary.tasks[i].keyInfo = detail.Name
			break
		}
	}
	summary := cloneSummary(s.summary)
	s.mu.Unlock()
	if summary.active {
		s.persistSummary(summary)
	}
}

func (s *Sink) OnNodeNextList(_ *maa.Context, _ maa.EventStatus, _ maa.NodeNextListDetail)       {}
func (s *Sink) OnNodeRecognition(_ *maa.Context, _ maa.EventStatus, _ maa.NodeRecognitionDetail) {}
func (s *Sink) OnNodeAction(_ *maa.Context, _ maa.EventStatus, _ maa.NodeActionDetail)           {}
func (s *Sink) OnNodeRecognitionNode(_ *maa.Context, _ maa.EventStatus, _ maa.NodeRecognitionNodeDetail) {
}
func (s *Sink) OnNodeActionNode(_ *maa.Context, _ maa.EventStatus, _ maa.NodeActionNodeDetail) {}
