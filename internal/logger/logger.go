package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// Logger provides structured logging
type Logger struct {
	*log.Logger
	level Level
}

// Level represents log level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var defaultLogger *Logger

func init() {
	defaultLogger = &Logger{
		Logger: log.New(os.Stdout, "", 0),
		level:  LevelInfo,
	}
}

// SetLevel sets the log level
func SetLevel(level Level) {
	defaultLogger.level = level
}

// Debug logs a debug message
func Debug(format string, v ...interface{}) {
	if defaultLogger.level <= LevelDebug {
		defaultLogger.logf("DEBUG", format, v...)
	}
}

// Info logs an info message
func Info(format string, v ...interface{}) {
	if defaultLogger.level <= LevelInfo {
		defaultLogger.logf("INFO", format, v...)
	}
}

// Warn logs a warning message
func Warn(format string, v ...interface{}) {
	if defaultLogger.level <= LevelWarn {
		defaultLogger.logf("WARN", format, v...)
	}
}

// Error logs an error message
func Error(format string, v ...interface{}) {
	if defaultLogger.level <= LevelError {
		defaultLogger.logf("ERROR", format, v...)
	}
}

// ErrorWithErr logs an error message with an error
func ErrorWithErr(msg string, err error, v ...interface{}) {
	if err != nil {
		// Build key-value pairs for structured logging
		args := make([]interface{}, 0, len(v)+2)
		args = append(args, v...)
		args = append(args, "error", err.Error())
		defaultLogger.logf("ERROR", msg, args...)
	} else {
		defaultLogger.logf("ERROR", msg, v...)
	}
}

func (l *Logger) logf(level, format string, v ...interface{}) {
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	
	// Format key-value pairs if present
	if len(v) > 0 && len(v)%2 == 0 {
		// Key-value pairs
		var parts []string
		for i := 0; i < len(v); i += 2 {
			if i+1 < len(v) {
				parts = append(parts, fmt.Sprintf("%v=%v", v[i], v[i+1]))
			}
		}
		if len(parts) > 0 {
			l.Printf("[%s] [%s] %s %s", timestamp, level, format, strings.Join(parts, " "))
		} else {
			l.Printf("[%s] [%s] %s", timestamp, level, format)
		}
	} else {
		// Regular format string
		l.Printf("[%s] [%s] "+format, append([]interface{}{timestamp, level}, v...)...)
	}
}
