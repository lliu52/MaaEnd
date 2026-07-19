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
	runSettleDelay        = 250 * time.Millisecond
)

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
	generation uint64
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
		s.onTaskStarting()
	case maa.EventStatusSucceeded:
		s.onTaskFinished(tasker, detail, true)
	case maa.EventStatusFailed:
		s.onTaskFinished(tasker, detail, false)
	}
}

func (s *Sink) onTaskStarting() {
	s.mu.Lock()
	isNewRun := !s.summary.active
	if isNewRun {
		s.summary = runSummary{active: true}
	}
	s.summary.total++
	s.generation++
	s.mu.Unlock()

	if isNewRun {
		s.enqueue(formatRunStarted())
	}
}

func (s *Sink) onTaskFinished(tasker *maa.Tasker, detail maa.TaskerTaskDetail, succeeded bool) {
	s.mu.Lock()
	if !s.summary.active {
		// Normally Starting is always emitted first. Keep the summary correct if
		// an older runtime only forwards the terminal event.
		s.summary = runSummary{active: true, total: 1}
	}
	if succeeded {
		s.summary.succeeded++
	} else {
		s.summary.failed = append(s.summary.failed, failedTask{
			entry: detail.Entry,
			log:   lastNodeLog(tasker, detail.TaskID),
		})
	}
	s.generation++
	generation := s.generation
	s.mu.Unlock()

	go s.finishAfterQuietPeriod(generation)
}

func (s *Sink) finishAfterQuietPeriod(generation uint64) {
	timer := time.NewTimer(runSettleDelay)
	defer timer.Stop()
	<-timer.C

	s.mu.Lock()
	if !s.summary.active || s.generation != generation {
		s.mu.Unlock()
		return
	}
	summary := s.summary
	s.summary = runSummary{}
	s.mu.Unlock()

	s.enqueue(formatRunSummary(summary))
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
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := s.client.send(ctx, item.text)
		cancel()
		if err != nil {
			log.Warn().Err(err).Msg("Failed to send Telegram notification")
		}
	}
}
