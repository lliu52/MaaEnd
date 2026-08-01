// Package parentwatch 在后台监视父进程存活情况：父进程一旦退出立即结束当前进程。
//
// 用途：MaaFramework 主进程（cpp-algo / go-service 的父进程）异常退出后，
// MaaAgentServer 仍会阻塞在 IPC 上不会自动结束，残留为孤儿 agent。
// 启动 watcher 后会有一个独立 goroutine 每秒检查一次父进程：
//   - 父进程仍在  → 继续监视。
//   - 父进程已退出 → 直接 os.Exit(0) 结束当前进程。
//
// 平台差异：
//   - Windows：启动时 OpenProcess(SYNCHRONIZE) 持有句柄，循环里
//     只用 WaitForSingleObject(0) 探活，规避 PID 复用造成的误判。
//   - POSIX：syscall.Kill(pid, 0) 探活，ESRCH 视为已退出。
package parentwatch

import (
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// PollInterval 是父进程存活检测的默认轮询间隔。
const PollInterval = 1 * time.Second

// ExitHandlerTimeout bounds cleanup work when the parent process disappears.
// The parent is already gone at this point, so the Agent must not remain alive
// indefinitely because a network-backed handler is stuck.
const ExitHandlerTimeout = 12 * time.Second

// logComponent 是本包统一的 zerolog `component` 字段值。
const logComponent = "parent-watcher"

var (
	exitHandlersMu sync.Mutex
	exitHandlers   []func()
)

// RegisterExitHandler adds bounded best-effort cleanup that runs before the
// Agent exits after losing its parent process.
func RegisterExitHandler(handler func()) {
	if handler == nil {
		return
	}
	exitHandlersMu.Lock()
	exitHandlers = append(exitHandlers, handler)
	exitHandlersMu.Unlock()
}

// Start 启动父进程监视器。
// 进程启动时获取父进程 PID 并打开句柄（Windows）/记录 PID（POSIX），
// 之后在后台 goroutine 中按 PollInterval 周期性检查。
// 仅应在程序启动阶段调用一次，goroutine 与进程同生命周期。
func Start() {
	parentPID := os.Getppid()
	if parentPID <= 1 {
		log.Warn().
			Str("component", logComponent).
			Int("parent_pid", parentPID).
			Msg("invalid parent pid, watcher disabled")
		return
	}

	watcher, err := newWatcher(parentPID)
	if err != nil {
		log.Warn().
			Err(err).
			Str("component", logComponent).
			Int("parent_pid", parentPID).
			Msg("failed to initialize parent watcher")
		return
	}

	log.Info().
		Str("component", logComponent).
		Int("parent_pid", parentPID).
		Msg("parent process watcher started")

	go runLoop(watcher, parentPID)
}

func runLoop(w watcher, parentPID int) {
	defer w.Close()

	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for range ticker.C {
		alive, checkErr := w.IsAlive()
		if checkErr != nil {
			log.Warn().
				Err(checkErr).
				Str("component", logComponent).
				Int("parent_pid", parentPID).
				Msg("parent liveness check failed")
			continue
		}
		if !alive {
			log.Warn().
				Str("component", logComponent).
				Int("parent_pid", parentPID).
				Msg("parent process has exited; shutting down")
			runExitHandlers()
			os.Exit(0)
		}
	}
}

func runExitHandlers() {
	exitHandlersMu.Lock()
	handlers := append([]func(){}, exitHandlers...)
	exitHandlersMu.Unlock()
	if len(handlers) == 0 {
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, handler := range handlers {
			handler()
		}
	}()

	select {
	case <-done:
	case <-time.After(ExitHandlerTimeout):
		log.Warn().
			Str("component", logComponent).
			Dur("timeout", ExitHandlerTimeout).
			Msg("parent exit handlers timed out")
	}
}

// watcher 是平台相关的探活句柄抽象。
type watcher interface {
	IsAlive() (bool, error)
	Close()
}
