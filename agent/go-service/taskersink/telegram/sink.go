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
	"CloseGame":        {},
	"MXU_KILLPROC":     {},
	"__MXU_KILLPROC__": {},
}

type notification struct {
	text string
}

type failedTask struct {
	entry string
	log   string
}

type taskState string

const (
	taskPending   taskState = "pending"
	taskRunning   taskState = "running"
	taskSucceeded taskState = "succeeded"
	taskFailed    taskState = "failed"
)

type plannedTask struct {
	name    string
	entry   string
	taskID  uint64
	state   taskState
	keyInfo string
}

type runSummary struct {
	active    bool
	total     int
	succeeded int
	failed    []failedTask
	tasks     []plannedTask
}

// Sink reports run boundaries and persists enough progress to report an
// interrupted run without querying a potentially wedged Tasker.
type Sink struct {
	client *botClient
	queue  chan notification

	mu             sync.Mutex
	stateMu        sync.Mutex
	summary        runSummary
	planTemplate   []plannedTask
	lastNodeByTask map[uint64]string
	sender         func(string)
	statePath      string
}

func newSink(cfg Config, client *http.Client, endpoint string) *Sink {
	sink := &Sink{
		client:         newBotClient(client, endpoint, cfg),
		queue:          make(chan notification, notificationQueueSize),
		statePath:      defaultStatePath(),
		planTemplate:   loadTaskPlan(),
		lastNodeByTask: make(map[uint64]string),
	}
	sink.summary = sink.loadSummary()
	if !sink.summary.active {
		sink.recoverInterruptedSummary()
	}
	go sink.run()
	return sink
}

// OnTaskerTask collects top-level task results into a single run summary.
func (s *Sink) OnTaskerTask(_ *maa.Tasker, event maa.EventStatus, detail maa.TaskerTaskDetail) {
	if strings.HasPrefix(detail.Entry, "__PRETASK__") {
		return
	}
	switch event {
	case maa.EventStatusStarting:
		s.onTaskStarting(detail)
	case maa.EventStatusSucceeded:
		s.onTaskFinished(detail, true)
	case maa.EventStatusFailed:
		s.onTaskFinished(detail, false)
	}
}

func (s *Sink) onTaskStarting(detail maa.TaskerTaskDetail) {
	if _, isShutdownTask := shutdownTaskEntries[detail.Entry]; isShutdownTask {
		s.mu.Lock()
		if !s.summary.active {
			s.mu.Unlock()
			return
		}
		summary := cloneSummary(s.summary)
		s.summary = runSummary{}
		s.mu.Unlock()

		// The shutdown task can terminate the host before its terminal callback.
		s.send(formatRunSummary(summary))
		s.clearSummary()
		return
	}

	s.mu.Lock()
	isNewRun := !s.summary.active
	if isNewRun {
		s.summary = runSummary{active: true, tasks: cloneTasks(s.planTemplate)}
	}
	s.summary.total++
	markTaskStarting(&s.summary, detail)
	summary := cloneSummary(s.summary)
	s.mu.Unlock()
	s.persistSummary(summary)

	if isNewRun {
		s.enqueue(formatRunStarted())
	}
}

func (s *Sink) onTaskFinished(detail maa.TaskerTaskDetail, succeeded bool) {
	s.mu.Lock()
	if !s.summary.active {
		s.mu.Unlock()
		return
	}
	keyInfo := s.lastNodeByTask[detail.TaskID]
	if succeeded {
		s.summary.succeeded++
	} else {
		logText := nodeLog(keyInfo)
		s.summary.failed = append(s.summary.failed, failedTask{entry: detail.Entry, log: logText})
	}
	markTaskFinished(&s.summary, detail, succeeded, keyInfo)
	summary := cloneSummary(s.summary)
	delete(s.lastNodeByTask, detail.TaskID)
	s.mu.Unlock()
	s.persistSummary(summary)
}

func markTaskStarting(summary *runSummary, detail maa.TaskerTaskDetail) {
	for i := range summary.tasks {
		if summary.tasks[i].state == taskPending {
			summary.tasks[i].state = taskRunning
			summary.tasks[i].taskID = detail.TaskID
			summary.tasks[i].entry = detail.Entry
			summary.tasks[i].keyInfo = detail.Entry
			return
		}
	}
	summary.tasks = append(summary.tasks, plannedTask{
		name: detail.Entry, entry: detail.Entry, taskID: detail.TaskID, state: taskRunning, keyInfo: detail.Entry,
	})
}

func markTaskFinished(summary *runSummary, detail maa.TaskerTaskDetail, succeeded bool, keyInfo string) {
	state := taskFailed
	if succeeded {
		state = taskSucceeded
	}
	for i := range summary.tasks {
		if summary.tasks[i].taskID == detail.TaskID && summary.tasks[i].state == taskRunning {
			summary.tasks[i].state = state
			summary.tasks[i].keyInfo = keyInfo
			return
		}
	}
}

func nodeLog(node string) string {
	if strings.TrimSpace(node) == "" {
		return "最后节点未知"
	}
	if isFailureKeyInfo(node) {
		return node
	}
	return fmt.Sprintf("最后节点：%s", node)
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

func formatInterruptedSummary(summary runSummary) string {
	return formatInterruptedSummaryWithHeading(summary, "⚠️ MAA 无日志卡死，外部监控即将重启")
}

func formatRecoveredSummary(summary runSummary) string {
	return formatInterruptedSummaryWithHeading(summary, "⚠️ MAA 检测到上次运行无日志中断，外部监控已重启")
}

func formatInterruptedSummaryWithHeading(summary runSummary, heading string) string {
	var succeeded, failed, running, pending []plannedTask
	for _, task := range summary.tasks {
		switch task.state {
		case taskSucceeded:
			succeeded = append(succeeded, task)
		case taskFailed:
			failed = append(failed, task)
		case taskRunning:
			running = append(running, task)
		default:
			pending = append(pending, task)
		}
	}

	var builder strings.Builder
	builder.WriteString(heading)
	writeTaskSection(&builder, "✅ 已成功", succeeded, false)
	writeTaskSection(&builder, "❌ 已失败", failed, true)
	writeTaskSection(&builder, "⏳ 执行中", running, true)
	writeTaskSection(&builder, "⏭ 未执行", pending, false)
	if len(summary.tasks) == 0 {
		fmt.Fprintf(&builder, "\n\n✅ 已成功：%d\n❌ 已失败：%d\n⚠️ 未能读取完整任务计划", summary.succeeded, len(summary.failed))
	}
	return builder.String()
}

func writeTaskSection(builder *strings.Builder, title string, tasks []plannedTask, withInfo bool) {
	fmt.Fprintf(builder, "\n\n%s (%d)", title, len(tasks))
	for _, task := range tasks {
		name := task.name
		if strings.TrimSpace(name) == "" {
			name = task.entry
		}
		fmt.Fprintf(builder, "\n- %s", name)
		if withInfo {
			fmt.Fprintf(builder, "\n  %s", strings.ReplaceAll(nodeLog(task.keyInfo), "\n", "\n  "))
		}
	}
}

// onParentExit is called after MaaEnd's parent disappears. It deliberately
// performs a synchronous send because the process exits immediately afterward.
func (s *Sink) onParentExit() {
	s.mu.Lock()
	if !s.summary.active {
		log.Warn().
			Str("component", "telegram").
			Msg("parent exited with no active run; interrupted notification skipped")
		s.mu.Unlock()
		return
	}
	summary := cloneSummary(s.summary)
	s.summary = runSummary{}
	s.mu.Unlock()

	log.Warn().
		Str("component", "telegram").
		Int("configured_tasks", len(summary.tasks)).
		Int("started_tasks", summary.total).
		Int("succeeded_tasks", summary.succeeded).
		Int("failed_tasks", len(summary.failed)).
		Str("state_path", s.statePath).
		Msg("parent exited during active run; sending interrupted Telegram snapshot synchronously")
	s.send(formatInterruptedSummary(summary))
	log.Info().
		Str("component", "telegram").
		Msg("interrupted Telegram snapshot send attempt finished; clearing state")
	s.clearSummary()
}

func cloneSummary(summary runSummary) runSummary {
	cloned := summary
	cloned.failed = append([]failedTask(nil), summary.failed...)
	cloned.tasks = cloneTasks(summary.tasks)
	return cloned
}

func cloneTasks(tasks []plannedTask) []plannedTask {
	return append([]plannedTask(nil), tasks...)
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
