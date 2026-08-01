package parentwatch

import (
	"sync/atomic"
	"testing"
)

func TestRunExitHandlers(t *testing.T) {
	exitHandlersMu.Lock()
	original := exitHandlers
	exitHandlers = nil
	exitHandlersMu.Unlock()
	t.Cleanup(func() {
		exitHandlersMu.Lock()
		exitHandlers = original
		exitHandlersMu.Unlock()
	})

	var calls atomic.Int32
	RegisterExitHandler(func() { calls.Add(1) })
	RegisterExitHandler(func() { calls.Add(1) })
	runExitHandlers()
	if got := calls.Load(); got != 2 {
		t.Fatalf("exit handler calls = %d, want 2", got)
	}
}
