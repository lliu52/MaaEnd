package dailyrewardtelegram

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

type recordingSender struct {
	mu       sync.Mutex
	messages []string
	notify   chan string
	err      error
}

func newRecordingSender() *recordingSender {
	return &recordingSender{notify: make(chan string, 8)}
}

func (s *recordingSender) Send(_ context.Context, text string) error {
	s.mu.Lock()
	s.messages = append(s.messages, text)
	s.mu.Unlock()
	s.notify <- text
	return s.err
}

func newTestSink(t *testing.T, planned bool) (*Sink, *recordingSender, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.json")
	sender := newRecordingSender()
	return newSink(sender, &stateStore{path: path}, planned), sender, path
}

func waitMessage(t *testing.T, sender *recordingSender) string {
	t.Helper()
	select {
	case message := <-sender.notify:
		return message
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Telegram message")
		return ""
	}
}

func waitStateRemoved(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("state file was not removed: %s", path)
}

func TestDailyRewardSuccessOnly(t *testing.T) {
	sink, sender, path := newTestSink(t, false)
	sink.OnTaskerTask(nil, maa.EventStatusStarting, maa.TaskerTaskDetail{TaskID: 7, Entry: dailyRewardEntry})
	sink.OnTaskerTask(nil, maa.EventStatusSucceeded, maa.TaskerTaskDetail{TaskID: 7, Entry: dailyRewardEntry})

	if message := waitMessage(t, sender); message != "✅ 日常奖励领取：成功" {
		t.Fatalf("unexpected message: %q", message)
	}
	waitStateRemoved(t, path)
}

func TestOtherTasksDoNotSendNotifications(t *testing.T) {
	sink, sender, _ := newTestSink(t, false)
	detail := maa.TaskerTaskDetail{TaskID: 6, Entry: "AutoStockpileMain"}
	sink.OnTaskerTask(nil, maa.EventStatusStarting, detail)
	sink.OnTaskerTask(nil, maa.EventStatusFailed, detail)

	select {
	case message := <-sender.notify:
		t.Fatalf("unexpected notification for unrelated task: %q", message)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestDailyRewardFailureIncludesNodeAndFocus(t *testing.T) {
	sink, sender, _ := newTestSink(t, false)
	sink.OnTaskerTask(nil, maa.EventStatusStarting, maa.TaskerTaskDetail{TaskID: 8, Entry: dailyRewardEntry})
	sink.OnNodePipelineNode(nil, maa.EventStatusFailed, maa.NodePipelineNodeDetail{
		TaskID: 8,
		Name:   "DailyTaskClaimActivityRewards",
		Focus:  "领取活跃度奖励失败",
	})
	sink.OnTaskerTask(nil, maa.EventStatusFailed, maa.TaskerTaskDetail{TaskID: 8, Entry: dailyRewardEntry})

	message := waitMessage(t, sender)
	for _, expected := range []string{"❌ 日常奖励领取：失败", "DailyTaskClaimActivityRewards", "领取活跃度奖励失败"} {
		if !strings.Contains(message, expected) {
			t.Fatalf("message %q does not contain %q", message, expected)
		}
	}
}

func TestPlannedTaskSkippedAtNormalShutdown(t *testing.T) {
	sink, sender, _ := newTestSink(t, true)
	sink.OnTaskerTask(nil, maa.EventStatusStarting, maa.TaskerTaskDetail{TaskID: 1, Entry: "AutoStockpileMain"})
	sink.OnTaskerTask(nil, maa.EventStatusStarting, maa.TaskerTaskDetail{TaskID: 2, Entry: "MXU_KILLPROC"})

	message := waitMessage(t, sender)
	if !strings.Contains(message, "⏭️ 日常奖励领取：跳过") || !strings.Contains(message, "未执行到该任务") {
		t.Fatalf("unexpected message: %q", message)
	}
}

func TestInterruptedRunningTaskRecoveredAsFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := &stateStore{path: path}
	err := store.save(persistedState{
		RunID:    "old-run",
		OwnerPID: 2147483000,
		Status:   statusRunning,
		TaskID:   9,
		LastNode: "DailyTaskEnterTab",
	})
	if err != nil {
		t.Fatal(err)
	}

	sender := newRecordingSender()
	_ = newSink(sender, store, true)
	message := waitMessage(t, sender)
	for _, expected := range []string{"检测到上次运行中断", "❌ 日常奖励领取：失败", "OneDragon", "DailyTaskEnterTab"} {
		if !strings.Contains(message, expected) {
			t.Fatalf("message %q does not contain %q", message, expected)
		}
	}
	waitStateRemoved(t, path)
}

func TestInterruptedPendingTaskRecoveredAsSkipped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := &stateStore{path: path}
	if err := store.save(persistedState{RunID: "old-run", OwnerPID: 2147483000, Status: statusPending}); err != nil {
		t.Fatal(err)
	}

	sender := newRecordingSender()
	_ = newSink(sender, store, true)
	message := waitMessage(t, sender)
	if !strings.Contains(message, "⏭️ 日常奖励领取：跳过") || !strings.Contains(message, "执行到该任务前") {
		t.Fatalf("unexpected message: %q", message)
	}
}
