package hangtest

import "testing"

func TestTestActionDefaults(t *testing.T) {
	if actionName != "TelegramNoLogHangTestAction" {
		t.Fatalf("action name = %q", actionName)
	}
	if defaultSilenceSeconds != 300 {
		t.Fatalf("default silence = %d", defaultSilenceSeconds)
	}
}
