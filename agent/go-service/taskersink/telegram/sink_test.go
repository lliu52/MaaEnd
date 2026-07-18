package telegram

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

func TestFormatMessage(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 18, 15, 4, 5, 0, time.FixedZone("CDT", -5*60*60))
	message := formatMessage(
		"✅",
		"completed",
		maa.TaskerTaskDetail{TaskID: 7, Entry: "DailyRewardStart"},
		"Gaming PC",
		now,
		90*time.Second,
	)

	for _, expected := range []string{
		"✅ MaaEnd task completed",
		"Task: DailyRewardStart",
		"Task ID: 7",
		"Device: Gaming PC",
		"Time: 2026-07-18 15:04:05 CDT",
		"Duration: 1m30s",
	} {
		if !strings.Contains(message, expected) {
			t.Errorf("message does not contain %q:\n%s", expected, message)
		}
	}
}

func TestSinkReportsLifecycleEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		event    maa.EventStatus
		expected string
	}{
		{name: "completed", event: maa.EventStatusSucceeded, expected: "✅ MaaEnd task completed"},
		{name: "failed", event: maa.EventStatusFailed, expected: "❌ MaaEnd task failed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			start := time.Date(2026, time.July, 18, 15, 0, 0, 0, time.UTC)
			var nowMu sync.Mutex
			now := start
			sink := &Sink{
				client: newBotClient(http.DefaultClient, "", Config{}),
				now: func() time.Time {
					nowMu.Lock()
					defer nowMu.Unlock()
					return now
				},
				queue:   make(chan notification, 2),
				started: make(map[uint64]time.Time),
			}
			detail := maa.TaskerTaskDetail{TaskID: 8, Entry: "DailyRewardStart"}

			sink.OnTaskerTask(nil, maa.EventStatusStarting, detail)
			startedMessage := <-sink.queue
			if !strings.Contains(startedMessage.text, "▶️ MaaEnd task started") {
				t.Fatalf("starting message = %q", startedMessage.text)
			}

			nowMu.Lock()
			now = start.Add(2 * time.Minute)
			nowMu.Unlock()
			sink.OnTaskerTask(nil, test.event, detail)
			finishedMessage := <-sink.queue
			if !strings.Contains(finishedMessage.text, test.expected) {
				t.Errorf("finished message does not contain %q: %s", test.expected, finishedMessage.text)
			}
			if !strings.Contains(finishedMessage.text, "Duration: 2m0s") {
				t.Errorf("finished message has no duration: %s", finishedMessage.text)
			}
		})
	}
}
