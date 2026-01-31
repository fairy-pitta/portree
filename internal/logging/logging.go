package logging

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// Level represents log verbosity.
type Level int

const (
	LevelQuiet   Level = -1
	LevelNormal  Level = 0
	LevelVerbose Level = 1
	LevelDebug   Level = 2
)

// Logger provides leveled logging output.
type Logger struct {
	mu    sync.RWMutex
	level Level
	out   io.Writer
}

var std = &Logger{level: LevelNormal, out: os.Stderr}

// SetLevel sets the global log level.
func SetLevel(l Level) {
	std.mu.Lock()
	std.level = l
	std.mu.Unlock()
}

// GetLevel returns the current log level.
func GetLevel() Level {
	std.mu.RLock()
	defer std.mu.RUnlock()
	return std.level
}

// IsVerbose returns true if verbose or debug output is enabled.
func IsVerbose() bool {
	return GetLevel() >= LevelVerbose
}

// IsDebug returns true if debug output is enabled.
func IsDebug() bool {
	return GetLevel() >= LevelDebug
}

// IsQuiet returns true if quiet mode is enabled.
func IsQuiet() bool {
	return GetLevel() <= LevelQuiet
}

// Info prints a message at normal level.
func Info(format string, args ...any) {
	std.mu.Lock()
	defer std.mu.Unlock()
	if std.level >= LevelNormal {
		_, _ = fmt.Fprintf(std.out, format+"\n", args...)
	}
}

// Verbose prints a message at verbose level.
func Verbose(format string, args ...any) {
	std.mu.Lock()
	defer std.mu.Unlock()
	if std.level >= LevelVerbose {
		_, _ = fmt.Fprintf(std.out, format+"\n", args...)
	}
}

// Debug prints a message at debug level.
func Debug(format string, args ...any) {
	std.mu.Lock()
	defer std.mu.Unlock()
	if std.level >= LevelDebug {
		_, _ = fmt.Fprintf(std.out, "[debug] "+format+"\n", args...)
	}
}

// Warn always prints unless in quiet mode.
func Warn(format string, args ...any) {
	std.mu.Lock()
	defer std.mu.Unlock()
	if std.level > LevelQuiet {
		_, _ = fmt.Fprintf(std.out, "warning: "+format+"\n", args...)
	}
}

// Error always prints.
func Error(format string, args ...any) {
	std.mu.Lock()
	defer std.mu.Unlock()
	_, _ = fmt.Fprintf(std.out, "error: "+format+"\n", args...)
}
