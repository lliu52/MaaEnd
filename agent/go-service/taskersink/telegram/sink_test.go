package telegram

import (
	"net/http"
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
					{entry: "DailyRewardStart", log: "最后节点 DailyRewardClaim"},
				},
			},
			expected: []string{
				"❌ MAA 运行完成",
				"任务成功 2/3",
				"❌ DailyRewardStart",
				"日志：最后节点 DailyRewardClaim",
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
	first := maa.TaskerTaskDetail{TaskID: 1, Entry: "DailyRewardStart"}
	second := maa.TaskerTaskDetail{TaskID: 2, Entry: "CreditShoppingMain"}

	sink.OnTaskerTask(nil, maa.EventStatusStarting, first)
	if message := receiveNotification(t, sink.queue); message.text != "✅ MAA 开始运行" {
		t.Fatalf("starting message = %q", message.text)
	}

	sink.OnTaskerTask(nil, maa.EventStatusSucceeded, first)
	sink.OnTaskerTask(nil, maa.EventStatusStarting, second)
	sink.OnTaskerTask(nil, maa.EventStatusFailed, second)

	summary := receiveNotification(t, sink.queue)
	for _, expected := range []string{
		"❌ MAA 运行完成",
		"任务成功 1/2",
		"❌ CreditShoppingMain",
		"日志：最后节点未知",
	} {
		if !strings.Contains(summary.text, expected) {
			t.Errorf("summary does not contain %q:\n%s", expected, summary.text)
		}
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
