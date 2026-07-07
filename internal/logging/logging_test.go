package logging

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/AkashiSN/pod-log-preserver/internal/config"
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
	if out := captureLog(t, func() { Debug(config.Config{LogLevel: "info"}, "hidden %d", 1) }); out != "" {
		t.Errorf("Debug at info level emitted %q, want nothing", out)
	}
	out := captureLog(t, func() { Debug(config.Config{LogLevel: "debug"}, "shown %d", 1) })
	if !strings.Contains(out, "shown 1") || !strings.Contains(out, "[DEBUG]") {
		t.Errorf("Debug at debug level = %q, want it to contain the message", out)
	}
}

// TestLogInfoGatedByLevel: info lines appear at info and debug, but an unknown
// level suppresses them.
func TestLogInfoGatedByLevel(t *testing.T) {
	for _, lvl := range []string{"info", "debug"} {
		out := captureLog(t, func() { Info(config.Config{LogLevel: lvl}, "hello") })
		if !strings.Contains(out, "hello") {
			t.Errorf("Info at %q level = %q, want the message", lvl, out)
		}
	}
	if out := captureLog(t, func() { Info(config.Config{LogLevel: ""}, "hi") }); out != "" {
		t.Errorf("Info at empty level emitted %q, want nothing", out)
	}
}

// TestLogWarnAndErrorAlwaysEmit: warn/error are unconditional.
func TestLogWarnAndErrorAlwaysEmit(t *testing.T) {
	if out := captureLog(t, func() { Warn("w %d", 2) }); !strings.Contains(out, "w 2") {
		t.Errorf("Warn = %q, want it to contain the message", out)
	}
	if out := captureLog(t, func() { Error("e %d", 3) }); !strings.Contains(out, "e 3") {
		t.Errorf("Error = %q, want it to contain the message", out)
	}
}
