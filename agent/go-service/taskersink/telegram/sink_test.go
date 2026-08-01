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
	summary := runSummary{
		total: 3, succeeded: 2,
		failed: []failedTask{{entry: "DailyRewardStart", log: "最后节点：DailyRewardClaim"}},
	}
	message := formatRunSummary(summary)
	for _, expected := range []string{"❌ MAA 运行完成", "任务成功 2/3", "❌ DailyRewardStart", "日志：最后节点：DailyRewardClaim"} {
		if !strings.Contains(message, expected) {
			t.Errorf("message does not contain %q:\n%s", expected, message)
		}
	}
}

func TestFailureUsesTrackedNodeWithoutReverseQuery(t *testing.T) {
	t.Parallel()
	sink := testSink()
	task := maa.TaskerTaskDetail{TaskID: 7, Entry: "SellProduct"}
	sink.OnTaskerTask(nil, maa.EventStatusStarting, task)
	_ = receiveNotification(t, sink.queue)
	sink.OnNodePipelineNode(nil, maa.EventStatusStarting, maa.NodePipelineNodeDetail{TaskID: 7, Name: "SellProductConfirm"})
	sink.OnTaskerTask(nil, maa.EventStatusFailed, task)

	if got := sink.summary.failed[0].log; got != "最后节点：SellProductConfirm" {
		t.Fatalf("failure log = %q", got)
	}
}

func TestPretaskDoesNotConsumeTaskPlan(t *testing.T) {
	t.Parallel()
	sink := testSink()
	sink.planTemplate = []plannedTask{{name: "售卖产品", state: taskPending}}
	sink.OnTaskerTask(nil, maa.EventStatusStarting, maa.TaskerTaskDetail{TaskID: 1, Entry: "__PRETASK__WakeGame"})
	if sink.summary.active {
		t.Fatal("pretask unexpectedly started a Telegram run")
	}
	sink.OnTaskerTask(nil, maa.EventStatusStarting, maa.TaskerTaskDetail{TaskID: 2, Entry: "SellProduct"})
	if sink.summary.tasks[0].taskID != 2 || sink.summary.tasks[0].state != taskRunning {
		t.Fatalf("task plan = %+v", sink.summary.tasks)
	}
	if sink.summary.tasks[0].keyInfo != "SellProduct" {
		t.Fatalf("running task fallback key info = %q", sink.summary.tasks[0].keyInfo)
	}
}

func TestSinkReportsOneMessagePerRunBoundary(t *testing.T) {
	t.Parallel()
	sink := testSink()
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
	for _, expected := range []string{"❌ MAA 运行完成", "任务成功 1/2", "❌ CreditShoppingMain", "日志：最后节点未知"} {
		if !strings.Contains(summary, expected) {
			t.Errorf("summary does not contain %q:\n%s", expected, summary)
		}
	}
}

func TestInterruptedSummaryIncludesAllTaskStates(t *testing.T) {
	t.Parallel()
	summary := runSummary{tasks: []plannedTask{
		{name: "领取奖励", state: taskSucceeded},
		{name: "售卖产品", state: taskFailed, keyInfo: "SellProductConfirm"},
		{name: "自动囤货", state: taskRunning, keyInfo: "OpenStockpile"},
		{name: "好友互动", state: taskPending},
	}}
	message := formatInterruptedSummary(summary)
	for _, expected := range []string{
		"⚠️ MAA 无日志卡死，外部监控即将重启",
		"✅ 已成功 (1)", "领取奖励",
		"❌ 已失败 (1)", "售卖产品", "最后节点：SellProductConfirm",
		"⏳ 执行中 (1)", "自动囤货", "最后节点：OpenStockpile",
		"⏭ 未执行 (1)", "好友互动",
	} {
		if !strings.Contains(message, expected) {
			t.Errorf("message does not contain %q:\n%s", expected, message)
		}
	}
}

func TestParentExitSendsInterruptedSummaryAndClearsState(t *testing.T) {
	t.Parallel()
	statePath := filepath.Join(t.TempDir(), "telegram-run-summary.json")
	sink := testSink()
	sink.statePath = statePath
	sink.summary = runSummary{active: true, tasks: []plannedTask{{name: "自动囤货", state: taskRunning}}}
	sink.persistSummary(sink.summary)
	messages := make(chan string, 1)
	sink.sender = func(text string) { messages <- text }

	sink.onParentExit()
	if message := receiveText(t, messages); !strings.Contains(message, "⏳ 执行中 (1)") {
		t.Fatalf("summary = %q", message)
	}
	if sink.summary.active {
		t.Fatal("summary remains active")
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("summary state was not removed: %v", err)
	}
}

func TestShutdownWaitsForFinalMessage(t *testing.T) {
	t.Parallel()
	sendStarted := make(chan string, 1)
	releaseSend := make(chan struct{})
	sink := testSink()
	sink.summary = runSummary{active: true, total: 2, succeeded: 1}
	sink.sender = func(text string) { sendStarted <- text; <-releaseSend }
	finished := make(chan struct{})
	go func() {
		sink.OnTaskerTask(nil, maa.EventStatusStarting, maa.TaskerTaskDetail{Entry: "MXU_KILLPROC"})
		close(finished)
	}()

	if message := receiveText(t, sendStarted); !strings.Contains(message, "任务成功 1/2") {
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
		t.Fatal("shutdown callback did not return")
	}
}

func TestSummarySurvivesAgentRestart(t *testing.T) {
	t.Parallel()
	statePath := filepath.Join(t.TempDir(), "telegram-run-summary.json")
	firstAgent := testSink()
	firstAgent.statePath = statePath
	firstAgent.planTemplate = []plannedTask{{name: "每日奖励", state: taskPending}, {name: "自动囤货", state: taskPending}}
	task := maa.TaskerTaskDetail{TaskID: 1, Entry: "DailyRewardStart"}
	firstAgent.OnTaskerTask(nil, maa.EventStatusStarting, task)
	_ = receiveNotification(t, firstAgent.queue)
	firstAgent.OnTaskerTask(nil, maa.EventStatusSucceeded, task)

	secondAgent := testSink()
	secondAgent.statePath = statePath
	secondAgent.summary = secondAgent.loadSummary()
	if !secondAgent.summary.active || secondAgent.summary.tasks[0].state != taskSucceeded || secondAgent.summary.tasks[1].state != taskPending {
		t.Fatalf("restored summary = %+v", secondAgent.summary)
	}
}

func TestRecoverInterruptedSummaryAfterProcessTreeKill(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stalePath := filepath.Join(dir, "telegram-run-summary-36268.json")
	currentPath := filepath.Join(dir, "telegram-run-summary-37132.json")
	staleSink := testSink()
	staleSink.statePath = stalePath
	staleSink.persistSummary(runSummary{
		active: true, total: 3, succeeded: 1,
		tasks: []plannedTask{
			{name: "已完成任务", state: taskSucceeded},
			{name: "卡死测试", entry: "TelegramHangTestStart", state: taskRunning, keyInfo: "TelegramHangTestStart"},
			{name: "后续任务", state: taskPending},
		},
	})

	sink := testSink()
	sink.statePath = currentPath
	messages := make(chan string, 1)
	sink.sender = func(text string) { messages <- text }
	sink.recoverInterruptedSummary()

	message := receiveText(t, messages)
	for _, expected := range []string{
		"检测到上次运行无日志中断，外部监控已重启",
		"✅ 已成功 (1)", "⏳ 执行中 (1)", "卡死测试",
		"最后节点：TelegramHangTestStart", "⏭ 未执行 (1)",
	} {
		if !strings.Contains(message, expected) {
			t.Errorf("message does not contain %q:\n%s", expected, message)
		}
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("recovered state was not removed: %v", err)
	}
}

func TestRecoveryIgnoresGracefullyStoppedSummary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	staleSink := testSink()
	staleSink.statePath = filepath.Join(dir, "telegram-run-summary-100.json")
	staleSink.persistSummary(runSummary{
		active: true, total: 1, succeeded: 1,
		tasks: []plannedTask{{name: "已完成任务", state: taskSucceeded}},
	})

	sink := testSink()
	sink.statePath = filepath.Join(dir, "telegram-run-summary-200.json")
	messages := make(chan string, 1)
	sink.sender = func(text string) { messages <- text }
	sink.recoverInterruptedSummary()
	select {
	case message := <-messages:
		t.Fatalf("unexpected recovered message: %q", message)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestLoadTaskPlanFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "mxu-MaaEnd.json")
	data := `{"lastActiveInstanceId":"selected","instances":[{"id":"other","tasks":[]},{"id":"selected","controllerName":"Win32","tasks":[{"taskName":"SellProduct","customName":"售卖产品","enabled":true},{"taskName":"AutoStockpile","enabled":true,"enabledByController":{"Win32":true}},{"taskName":"DisabledForController","enabled":true,"enabledByController":{"Win32":false}},{"taskName":"Disabled","enabled":false},{"taskName":"CloseGame","enabled":true}]}]}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := loadTaskPlanFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || plan[0].name != "售卖产品" || plan[1].name != "AutoStockpile" {
		t.Fatalf("plan = %+v", plan)
	}
}

func testSink() *Sink {
	return &Sink{
		client:         newBotClient(http.DefaultClient, "", Config{}),
		queue:          make(chan notification, 8),
		lastNodeByTask: make(map[uint64]string),
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
