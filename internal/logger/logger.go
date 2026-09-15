package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

// Level represents a logging level.
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var levelNames = map[Level]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

var nameToLevel = map[string]Level{
	"debug": DEBUG,
	"info":  INFO,
	"warn":  WARN,
	"error": ERROR,
}

// Logger provides structured logging with level filtering.
type Logger struct {
	mu     sync.Mutex
	logger *log.Logger
	level  Level
	output io.Writer
}

// New creates a new Logger with the given level and output.
func New(levelStr string, output io.Writer) *Logger {
	level, ok := nameToLevel[strings.ToLower(levelStr)]
	if !ok {
		level = INFO
	}
	if output == nil {
		output = os.Stderr
	}
	return &Logger{
		logger: log.New(output, "", log.Ldate|log.Ltime|log.Lshortfile),
		level:  level,
		output: output,
	}
}

// Default creates a logger with info level writing to stderr.
func Default() *Logger {
	return New("info", os.Stderr)
}

// SetLevel changes the minimum log level.
func (l *Logger) SetLevel(levelStr string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if level, ok := nameToLevel[strings.ToLower(levelStr)]; ok {
		l.level = level
	}
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(DEBUG, msg, args...)
}

// Info logs an info message.
func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(INFO, msg, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(WARN, msg, args...)
}

// Error logs an error message.
func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(ERROR, msg, args...)
}

func (l *Logger) log(level Level, msg string, args ...interface{}) {
	if level < l.level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Redact sensitive fields based on format string content
	safeArgs := redact(msg, args)
	
	entry := fmt.Sprintf("[%s] %s", levelNames[level], fmt.Sprintf(msg, safeArgs...))
	l.logger.Output(3, entry)
}

// redact filters out potentially sensitive values from log arguments.
// It scans the format string for sensitive field names and redacts the
// corresponding format arguments.
func redact(msg string, args []interface{}) []interface{} {
	sensitive := []string{"password", "secret", "token", "key", "credential"}
	
	// Check if format string contains sensitive field names
	formatLower := strings.ToLower(msg)
	for _, s := range sensitive {
		if strings.Contains(formatLower, s) {
			// Redact all args
			result := make([]interface{}, len(args))
			for i := range args {
				result[i] = "***REDACTED***"
			}
			return result
		}
	}
	
	return args
}
