package dailyrewardtelegram

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/rs/zerolog/log"
)

const (
	dailyRewardEntry = "DailyRewardStart"
	sendTimeout      = 10 * time.Second
	queueSize        = 4
	maxKeyInfoRunes  = 500
)

var shutdownEntries = map[string]struct{}{
	"CloseGame":        {},
	"MXU_KILLPROC":     {},
	"__MXU_KILLPROC__": {},
}

type queuedMessage struct {
	text  string
	runID string
}

// Sink reports only the DailyRewards top-level task.
type Sink struct {
	sender messageSender
	store  *stateStore
	queue  chan queuedMessage

	mu                 sync.Mutex
	state              persistedState
	dailyRewardPlanned bool
}

func newSink(sender messageSender, store *stateStore, dailyRewardPlanned bool) *Sink {
	sink := &Sink{
		sender:             sender,
		store:              store,
		queue:              make(chan queuedMessage, queueSize),
		dailyRewardPlanned: dailyRewardPlanned,
	}
	go sink.runSender()
	sink.recoverPreviousRun()
	return sink
}

func (s *Sink) OnTaskerTask(_ *maa.Tasker, event maa.EventStatus, detail maa.TaskerTaskDetail) {
	if event == maa.EventStatusStarting {
		if _, shutdown := shutdownEntries[detail.Entry]; shutdown {
			s.onShutdownStarting()
			return
		}
		if strings.HasPrefix(detail.Entry, "__PRETASK__") || detail.Entry == "MaaTaskerPostStop" {
			return
		}
		s.onTaskStarting(detail)
		return
	}
	if detail.Entry != dailyRewardEntry {
		return
	}
	switch event {
	case maa.EventStatusSucceeded:
		s.finish(statusSucceeded, "")
	case maa.EventStatusFailed:
		s.finish(statusFailed, "任务执行失败")
	}
}

func (s *Sink) onTaskStarting(detail maa.TaskerTaskDetail) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if detail.Entry == dailyRewardEntry {
		s.state = newState(statusRunning)
		s.state.TaskID = detail.TaskID
		s.persistLocked()
		return
	}
	if s.dailyRewardPlanned && s.state.Status == "" {
		s.state = newState(statusPending)
		s.persistLocked()
	}
}

func (s *Sink) onShutdownStarting() {
	s.mu.Lock()
	if s.state.Status != statusPending {
		s.mu.Unlock()
		return
	}
	s.state.Status = statusSkipped
	s.state.Reason = "本次 MAA 运行未执行到该任务"
	s.persistLocked()
	state := s.state
	s.mu.Unlock()
	s.enqueue(state)
}

func (s *Sink) finish(status taskStatus, reason string) {
	s.mu.Lock()
	if s.state.Status != statusRunning {
		s.mu.Unlock()
		return
	}
	s.state.Status = status
	s.state.Reason = reason
	s.persistLocked()
	state := s.state
	s.mu.Unlock()
	s.enqueue(state)
}

func (s *Sink) OnNodePipelineNode(_ *maa.Context, event maa.EventStatus, detail maa.NodePipelineNodeDetail) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Status != statusRunning || detail.TaskID != s.state.TaskID || detail.Name == "" {
		return
	}
	if s.state.LastNode == detail.Name && event != maa.EventStatusFailed {
		return
	}
	s.state.LastNode = detail.Name
	if event == maa.EventStatusFailed {
		s.state.KeyInfo = compactFocus(detail.Focus)
	}
	s.persistLocked()
}

func (s *Sink) OnNodeNextList(_ *maa.Context, _ maa.EventStatus, _ maa.NodeNextListDetail) {}

func (s *Sink) OnNodeRecognition(_ *maa.Context, _ maa.EventStatus, _ maa.NodeRecognitionDetail) {}

func (s *Sink) OnNodeAction(_ *maa.Context, _ maa.EventStatus, _ maa.NodeActionDetail) {}

func (s *Sink) OnNodeRecognitionNode(_ *maa.Context, _ maa.EventStatus, _ maa.NodeRecognitionNodeDetail) {
}

func (s *Sink) OnNodeActionNode(_ *maa.Context, _ maa.EventStatus, _ maa.NodeActionNodeDetail) {}

func newState(status taskStatus) persistedState {
	return persistedState{
		RunID:    fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano()),
		OwnerPID: int32(os.Getpid()),
		Status:   status,
	}
}

func (s *Sink) persistLocked() {
	if err := s.store.save(s.state); err != nil {
		log.Warn().Err(err).Str("component", "daily-reward-telegram").Msg("Failed to persist notification state")
	}
}

func (s *Sink) enqueue(state persistedState) {
	message := queuedMessage{text: formatResult(state, false), runID: state.RunID}
	select {
	case s.queue <- message:
	default:
		log.Warn().Str("component", "daily-reward-telegram").Msg("Notification queue is full; result will be retried on next launch")
	}
}

func (s *Sink) runSender() {
	for message := range s.queue {
		ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
		err := s.sender.Send(ctx, message.text)
		cancel()
		if err != nil {
			log.Warn().Err(err).Str("component", "daily-reward-telegram").Msg("Failed to send Telegram notification; state retained for retry")
			continue
		}
		if err := s.store.clearIfRun(message.runID); err != nil {
			log.Warn().Err(err).Str("component", "daily-reward-telegram").Msg("Failed to clear delivered notification state")
		}
	}
}

func (s *Sink) recoverPreviousRun() {
	state, claimPath, claimed, err := s.store.claimRecovery()
	if err != nil {
		log.Warn().Err(err).Str("component", "daily-reward-telegram").Msg("Failed to inspect previous notification state")
		return
	}
	if !claimed {
		return
	}

	if state.Status == statusPending {
		state.Status = statusSkipped
		state.Reason = "上次 MAA 运行在执行到该任务前被外部终止"
	} else if state.Status == statusRunning {
		state.Status = statusFailed
		state.Reason = "上次执行期间 MaaEnd 无响应或被 OneDragon 外部终止"
	}

	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	err = s.sender.Send(ctx, formatResult(state, true))
	cancel()
	if finishErr := s.store.finishRecovery(claimPath, err == nil); finishErr != nil {
		log.Warn().Err(finishErr).Str("component", "daily-reward-telegram").Msg("Failed to finalize previous notification state")
	}
	if err != nil {
		log.Warn().Err(err).Str("component", "daily-reward-telegram").Msg("Failed to recover previous Telegram result")
		return
	}
	log.Info().Str("component", "daily-reward-telegram").Str("status", string(state.Status)).Msg("Recovered previous DailyRewards result")
}

func formatResult(state persistedState, recovered bool) string {
	prefix := ""
	if recovered {
		prefix = "检测到上次运行中断，补发结果：\n"
	}
	switch state.Status {
	case statusSucceeded:
		return prefix + "✅ 日常奖励领取：成功"
	case statusSkipped:
		return prefix + "⏭️ 日常奖励领取：跳过\n原因：" + fallback(state.Reason, "任务未执行")
	default:
		var builder strings.Builder
		builder.WriteString(prefix)
		builder.WriteString("❌ 日常奖励领取：失败")
		if state.Reason != "" {
			builder.WriteString("\n原因：")
			builder.WriteString(state.Reason)
		}
		if state.LastNode != "" {
			builder.WriteString("\n最后节点：")
			builder.WriteString(state.LastNode)
		}
		if state.KeyInfo != "" {
			builder.WriteString("\n关键日志：")
			builder.WriteString(state.KeyInfo)
		}
		return builder.String()
	}
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
	if len(runes) > maxKeyInfoRunes {
		return string(runes[:maxKeyInfoRunes]) + "…"
	}
	return text
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
