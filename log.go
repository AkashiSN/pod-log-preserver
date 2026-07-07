package main

import "log"

// Leveled logging on top of the standard logger. Timestamps are left to the
// container runtime (main sets log.SetFlags(0)). LOG_LEVEL selects verbosity:
// "debug" shows everything; "info" (the default) shows info/warn/error; any
// other value shows only warn/error.

func logDebug(cfg Config, format string, args ...any) {
	if cfg.LogLevel == "debug" {
		log.Printf("[DEBUG] "+format, args...)
	}
}

func logInfo(cfg Config, format string, args ...any) {
	if cfg.LogLevel == "debug" || cfg.LogLevel == "info" {
		log.Printf("[INFO]  "+format, args...)
	}
}

func logWarn(format string, args ...any) {
	log.Printf("[WARN]  "+format, args...)
}

func logError(format string, args ...any) {
	log.Printf("[ERROR] "+format, args...)
}
