// Package logging provides leveled logging on top of the standard logger.
// Timestamps are left to the container runtime (main sets log.SetFlags(0)).
// LOG_LEVEL (config.Config.LogLevel) selects verbosity: "debug" shows
// everything; "info" (the default) shows info/warn/error; any other value shows
// only warn/error.
package logging

import (
	"log"

	"github.com/AkashiSN/pod-log-preserver/internal/config"
)

// Debug logs at debug level, emitted only when LOG_LEVEL=debug.
func Debug(cfg config.Config, format string, args ...any) {
	if cfg.LogLevel == "debug" {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// Info logs at info level, emitted at LOG_LEVEL debug or info.
func Info(cfg config.Config, format string, args ...any) {
	if cfg.LogLevel == "debug" || cfg.LogLevel == "info" {
		log.Printf("[INFO]  "+format, args...)
	}
}

// Warn logs at warn level; always emitted.
func Warn(format string, args ...any) {
	log.Printf("[WARN]  "+format, args...)
}

// Error logs at error level; always emitted.
func Error(format string, args ...any) {
	log.Printf("[ERROR] "+format, args...)
}
