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

const notificationQueueSize = 128

type notification struct {
	taskID uint64
	entry  string
	text   string
}

// Sink reports MaaFramework task lifecycle events to Telegram without blocking task execution.
type Sink struct {
	client *botClient
	now    func() time.Time
	queue  chan notification

	mu      sync.Mutex
	started map[uint64]time.Time
}

func newSink(cfg Config, client *http.Client, endpoint string) *Sink {
	sink := &Sink{
		client:  newBotClient(client, endpoint, cfg),
		now:     time.Now,
		queue:   make(chan notification, notificationQueueSize),
		started: make(map[uint64]time.Time),
	}
	go sink.run()
	return sink
}

// OnTaskerTask handles top-level task lifecycle events emitted by MaaFramework.
func (s *Sink) OnTaskerTask(_ *maa.Tasker, event maa.EventStatus, detail maa.TaskerTaskDetail) {
	now := s.now()
	var status string
	var icon string
	var duration time.Duration

	s.mu.Lock()
	switch event {
	case maa.EventStatusStarting:
		s.started[detail.TaskID] = now
		status = "started"
		icon = "▶️"
	case maa.EventStatusSucceeded:
		status = "completed"
		icon = "✅"
		if started, ok := s.started[detail.TaskID]; ok {
			duration = now.Sub(started)
			delete(s.started, detail.TaskID)
		}
	case maa.EventStatusFailed:
		status = "failed"
		icon = "❌"
		if started, ok := s.started[detail.TaskID]; ok {
			duration = now.Sub(started)
			delete(s.started, detail.TaskID)
		}
	default:
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	text := formatMessage(icon, status, detail, s.client.config.Label, now, duration)
	item := notification{taskID: detail.TaskID, entry: detail.Entry, text: text}
	select {
	case s.queue <- item:
	default:
		log.Warn().
			Uint64("task_id", detail.TaskID).
			Str("entry", detail.Entry).
			Msg("Telegram notification queue is full, dropping task notification")
	}
}

func (s *Sink) run() {
	for item := range s.queue {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := s.client.send(ctx, item.text)
		cancel()
		if err != nil {
			log.Warn().
				Err(err).
				Uint64("task_id", item.taskID).
				Str("entry", item.entry).
				Msg("Failed to send Telegram task notification")
		}
	}
}

func formatMessage(
	icon string,
	status string,
	detail maa.TaskerTaskDetail,
	label string,
	now time.Time,
	duration time.Duration,
) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "%s MaaEnd task %s\n", icon, status)
	fmt.Fprintf(&builder, "Task: %s\n", detail.Entry)
	fmt.Fprintf(&builder, "Task ID: %d\n", detail.TaskID)
	if label = strings.TrimSpace(label); label != "" {
		fmt.Fprintf(&builder, "Device: %s\n", label)
	}
	fmt.Fprintf(&builder, "Time: %s", now.Format("2006-01-02 15:04:05 MST"))
	if duration > 0 {
		fmt.Fprintf(&builder, "\nDuration: %s", duration.Round(time.Second))
	}
	return builder.String()
}
