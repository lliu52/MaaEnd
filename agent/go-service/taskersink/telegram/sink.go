package telegram

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/rs/zerolog/log"
)

const (
	notificationQueueSize = 128
	sendTimeout           = 10 * time.Second
)

var shutdownTaskEntries = map[string]struct{}{
	"CloseGame":    {},
	"MXU_KILLPROC": {},
}

type notification struct {
	text string
}

type failedTask struct {
	entry string
	log   string
}

type runSummary struct {
	active    bool
	total     int
	succeeded int
	failed    []failedTask
}

// Sink reports one start message and one summary for each MaaEnd run.
type Sink struct {
	client *botClient
	queue  chan notification

	mu         sync.Mutex
	summary    runSummary
	sender     func(string)
}

func newSink(cfg Config, client *http.Client, endpoint string) *Sink {
	sink := &Sink{
		client: newBotClient(client, endpoint, cfg),
		queue:  make(chan notification, notificationQueueSize),
	}
	go sink.run()
	return sink
}

// OnTaskerTask collects top-level task results into a single run summary.
func (s *Sink) OnTaskerTask(tasker *maa.Tasker, event maa.EventStatus, detail maa.TaskerTaskDetail) {
	switch event {
	case maa.EventStatusStarting:
		s.onTaskStarting(detail)
	case maa.EventStatusSucceeded:
		s.onTaskFinished(tasker, detail, true)
	case maa.EventStatusFailed:
		s.onTaskFinished(tasker, detail, false)
	}
}

func (s *Sink) onTaskStarting(detail maa.TaskerTaskDetail) {
	if _, isShutdownTask := shutdownTaskEntries[detail.Entry]; isShutdownTask {
		s.mu.Lock()
		if !s.summary.active {
			s.mu.Unlock()
			return
		}
		summary := s.summary
		s.summary = runSummary{}
		s.mu.Unlock()

		// Closing the game and closing MXU are destructive control actions. Send
		// the result before either action starts, and do not return until Telegram
		// has responded. In particular, MXU_KILLPROC may terminate MXU from inside
		// the action, so its terminal callback is not a safe notification point.
		s.send(formatRunSummary(summary))
		return
	}

	s.mu.Lock()
	isNewRun := !s.summary.active
	if isNewRun {
		s.summary = runSummary{active: true}
	}
	s.summary.total++
	s.mu.Unlock()

	if isNewRun {
		s.enqueue(formatRunStarted())
	}
}

func (s *Sink) onTaskFinished(tasker *maa.Tasker, detail maa.TaskerTaskDetail, succeeded bool) {
	s.mu.Lock()
	if !s.summary.active {
		// Shutdown actions finish after the summary has already been sent. They
		// are control actions and must not start a second run summary.
		s.mu.Unlock()
		return
	}
	if succeeded {
		s.summary.succeeded++
	} else {
		s.summary.failed = append(s.summary.failed, failedTask{
			entry: detail.Entry,
			log:   lastNodeLog(tasker, detail.TaskID),
		})
	}
	s.mu.Unlock()
}

func lastNodeLog(tasker *maa.Tasker, taskID uint64) string {
	if tasker == nil {
		return "最后节点未知"
	}
	detail, err := tasker.GetTaskDetail(int64(taskID))
	if err != nil || detail == nil || len(detail.Nodes) == 0 {
		return "最后节点未知"
	}
	last, err := detail.Nodes[len(detail.Nodes)-1].GetDetail()
	if err != nil || last == nil || strings.TrimSpace(last.Name) == "" {
		return "最后节点未知"
	}
	return fmt.Sprintf("最后节点 %s", last.Name)
}

func formatRunStarted() string {
	return "✅ MAA 开始运行"
}

func formatRunSummary(summary runSummary) string {
	icon := "✅"
	if len(summary.failed) > 0 {
		icon = "❌"
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "%s MAA 运行完成\n", icon)
	fmt.Fprintf(&builder, "任务成功 %d/%d", summary.succeeded, summary.total)
	if len(summary.failed) == 0 {
		return builder.String()
	}

	builder.WriteString("\n失败任务：")
	for _, failed := range summary.failed {
		fmt.Fprintf(&builder, "\n❌ %s\n日志：%s", failed.entry, failed.log)
	}
	return builder.String()
}

func (s *Sink) enqueue(text string) {
	select {
	case s.queue <- notification{text: text}:
	default:
		log.Warn().Msg("Telegram notification queue is full, dropping notification")
	}
}

func (s *Sink) run() {
	for item := range s.queue {
		s.send(item.text)
	}
}

func (s *Sink) send(text string) {
	if s.sender != nil {
		s.sender(text)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	if err := s.client.send(ctx, text); err != nil {
		log.Warn().Err(err).Msg("Failed to send Telegram notification")
	}
}
