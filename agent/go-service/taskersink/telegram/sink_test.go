package telegram

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

func TestFormatRunSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		summary  runSummary
		expected []string
	}{
		{
			name:     "all succeeded",
			summary:  runSummary{total: 3, succeeded: 3},
			expected: []string{"✅ MAA 运行完成", "任务成功 3/3"},
		},
		{
			name: "includes failed task log",
			summary: runSummary{
				total:     3,
				succeeded: 2,
				failed: []failedTask{
					{entry: "DailyRewardStart", log: "最后节点：DailyRewardClaim"},
				},
			},
			expected: []string{
				"❌ MAA 运行完成",
				"任务成功 2/3",
				"❌ DailyRewardStart",
				"日志：最后节点：DailyRewardClaim",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			message := formatRunSummary(test.summary)
			for _, expected := range test.expected {
				if !strings.Contains(message, expected) {
					t.Errorf("message does not contain %q:\n%s", expected, message)
				}
			}
		})
	}
}

func TestSinkReportsOneMessagePerRunBoundary(t *testing.T) {
	t.Parallel()

	sink := &Sink{
		client: newBotClient(http.DefaultClient, "", Config{}),
		queue:  make(chan notification, 4),
	}
	finalMessage := make(chan string, 1)
	sink.sender = func(text string) { finalMessage <- text }
	first := maa.TaskerTaskDetail{TaskID: 1, Entry: "DailyRewardStart"}
	second := maa.TaskerTaskDetail{TaskID: 2, Entry: "CreditShoppingMain"}

	sink.OnTaskerTask(nil, maa.EventStatusStarting, first)
	if message := receiveNotification(t, sink.queue); message.text != "✅ MAA 开始运行" {
		t.Fatalf("starting message = %q", message.text)
	}

	sink.OnTaskerTask(nil, maa.EventStatusSucceeded, first)
	sink.OnTaskerTask(nil, maa.EventStatusStarting, second)
	sink.OnTaskerTask(nil, maa.EventStatusFailed, second)
	sink.OnTaskerTask(nil, maa.EventStatusStarting, maa.TaskerTaskDetail{Entry: "MXU_KILLPROC"})

	summary := receiveText(t, finalMessage)
	for _, expected := range []string{
		"❌ MAA 运行完成",
		"任务成功 1/2",
		"❌ CreditShoppingMain",
		"日志：最后节点未知",
	} {
		if !strings.Contains(summary, expected) {
			t.Errorf("summary does not contain %q:\n%s", expected, summary)
		}
	}
}

func TestSummaryWaitsForShutdownTask(t *testing.T) {
	t.Parallel()

	sink := &Sink{
		client: newBotClient(http.DefaultClient, "", Config{}),
		queue:  make(chan notification, 4),
	}
	finalMessage := make(chan string, 1)
	sink.sender = func(text string) { finalMessage <- text }
	first := maa.TaskerTaskDetail{TaskID: 1, Entry: "FirstTask"}
	second := maa.TaskerTaskDetail{TaskID: 2, Entry: "SecondTask"}

	sink.OnTaskerTask(nil, maa.EventStatusStarting, first)
	_ = receiveNotification(t, sink.queue)
	sink.OnTaskerTask(nil, maa.EventStatusSucceeded, first)
	sink.OnTaskerTask(nil, maa.EventStatusStarting, second)
	sink.OnTaskerTask(nil, maa.EventStatusSucceeded, second)

	select {
	case item := <-finalMessage:
		t.Fatalf("received a summary before shutdown: %q", item)
	case <-time.After(100 * time.Millisecond):
	}

	sink.OnTaskerTask(nil, maa.EventStatusStarting, maa.TaskerTaskDetail{Entry: "CloseGame"})
	summary := receiveText(t, finalMessage)
	if !strings.Contains(summary, "任务成功 2/2") {
		t.Fatalf("summary = %q", summary)
	}
}

func TestShutdownWaitsForFinalMessage(t *testing.T) {
	t.Parallel()

	sendStarted := make(chan string, 1)
	releaseSend := make(chan struct{})
	sink := &Sink{
		client: newBotClient(http.DefaultClient, "", Config{}),
		queue:  make(chan notification, 1),
		summary: runSummary{
			active:    true,
			total:     2,
			succeeded: 1,
		},
	}
	sink.sender = func(text string) {
		sendStarted <- text
		<-releaseSend
	}
	finished := make(chan struct{})
	go func() {
		sink.OnTaskerTask(nil, maa.EventStatusStarting, maa.TaskerTaskDetail{Entry: "MXU_KILLPROC"})
		close(finished)
	}()

	var message string
	select {
	case message = <-sendStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for final Telegram send")
	}
	if !strings.Contains(message, "任务成功 1/2") {
		t.Fatalf("summary = %q", message)
	}

	select {
	case <-finished:
		t.Fatal("shutdown callback returned before Telegram send completed")
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseSend)
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown callback did not return after Telegram send completed")
	}
}

func TestSummarySurvivesAgentRestart(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "telegram-run-summary.json")
	firstAgent := &Sink{
		client:    newBotClient(http.DefaultClient, "", Config{}),
		queue:     make(chan notification, 1),
		statePath: statePath,
	}
	task := maa.TaskerTaskDetail{TaskID: 1, Entry: "DailyRewardStart"}
	firstAgent.OnTaskerTask(nil, maa.EventStatusStarting, task)
	_ = receiveNotification(t, firstAgent.queue)
	firstAgent.OnTaskerTask(nil, maa.EventStatusSucceeded, task)

	secondAgent := &Sink{
		client:    newBotClient(http.DefaultClient, "", Config{}),
		queue:     make(chan notification, 1),
		statePath: statePath,
	}
	secondAgent.summary = secondAgent.loadSummary()
	if !secondAgent.summary.active || secondAgent.summary.total != 1 || secondAgent.summary.succeeded != 1 {
		t.Fatalf("restored summary = %+v", secondAgent.summary)
	}

	finalMessage := make(chan string, 1)
	secondAgent.sender = func(text string) { finalMessage <- text }
	secondAgent.OnTaskerTask(nil, maa.EventStatusStarting, maa.TaskerTaskDetail{Entry: "MXU_KILLPROC"})
	if message := receiveText(t, finalMessage); !strings.Contains(message, "任务成功 1/1") {
		t.Fatalf("summary = %q", message)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("summary state was not removed: %v", err)
	}
}

func receiveNotification(t *testing.T, queue <-chan notification) notification {
	t.Helper()
	select {
	case item := <-queue:
		return item
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for notification")
		return notification{}
	}
}

func receiveText(t *testing.T, messages <-chan string) string {
	t.Helper()
	select {
	case message := <-messages:
		return message
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for message")
		return ""
	}
}
