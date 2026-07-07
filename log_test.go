package main

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

// captureLog redirects the standard logger into a buffer for the duration of
// fn and returns what was written. Flags are zeroed so assertions match only
// the message. Tests using it must not run in parallel (the logger is global).
func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	oldOut, oldFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(oldOut)
		log.SetFlags(oldFlags)
	})
	fn()
	return buf.String()
}

// TestLogDebugGatedByLevel: debug lines appear only at LOG_LEVEL=debug.
func TestLogDebugGatedByLevel(t *testing.T) {
	if out := captureLog(t, func() { logDebug(Config{LogLevel: "info"}, "hidden %d", 1) }); out != "" {
		t.Errorf("logDebug at info level emitted %q, want nothing", out)
	}
	out := captureLog(t, func() { logDebug(Config{LogLevel: "debug"}, "shown %d", 1) })
	if !strings.Contains(out, "shown 1") || !strings.Contains(out, "[DEBUG]") {
		t.Errorf("logDebug at debug level = %q, want it to contain the message", out)
	}
}

// TestLogInfoGatedByLevel: info lines appear at info and debug, but an unknown
// level suppresses them.
func TestLogInfoGatedByLevel(t *testing.T) {
	for _, lvl := range []string{"info", "debug"} {
		out := captureLog(t, func() { logInfo(Config{LogLevel: lvl}, "hello") })
		if !strings.Contains(out, "hello") {
			t.Errorf("logInfo at %q level = %q, want the message", lvl, out)
		}
	}
	if out := captureLog(t, func() { logInfo(Config{LogLevel: ""}, "hi") }); out != "" {
		t.Errorf("logInfo at empty level emitted %q, want nothing", out)
	}
}

// TestLogWarnAndErrorAlwaysEmit: warn/error are unconditional.
func TestLogWarnAndErrorAlwaysEmit(t *testing.T) {
	if out := captureLog(t, func() { logWarn("w %d", 2) }); !strings.Contains(out, "w 2") {
		t.Errorf("logWarn = %q, want it to contain the message", out)
	}
	if out := captureLog(t, func() { logError("e %d", 3) }); !strings.Contains(out, "e 3") {
		t.Errorf("logError = %q, want it to contain the message", out)
	}
}
