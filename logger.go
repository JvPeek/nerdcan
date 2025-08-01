package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
	CRISIS
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	case CRISIS:
		return "CRISIS"
	default:
		return "UNKNOWN"
	}
}

type LogEntry struct {
	Level   LogLevel
	File    string
	Line    int
	Message string
}

var (
	logMutex    sync.Mutex
	logMessages []LogEntry
)

// Log logs a message with the given level, automatically capturing file and line number.
func Log(level LogLevel, format string, a ...interface{}) {
	logInternal(level, 2, format, a...)
}

// logInternal is the internal logging function that allows specifying the call stack skip level.
func logInternal(level LogLevel, skip int, format string, a ...interface{}) {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		file = "<unknown>"
		line = 0
	}

	// Extract just the filename from the full path
	fileName := filepath.Base(file)

	logMutex.Lock()
	defer logMutex.Unlock()
	logMessages = append(logMessages, LogEntry{
		Level:   level,
		File:    fileName,
		Line:    line,
		Message: fmt.Sprintf(format, a...),
	})
}

func GetLogs() []LogEntry {
	logMutex.Lock()
	defer logMutex.Unlock()
	return logMessages
}
