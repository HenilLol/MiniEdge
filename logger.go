// Package logger provides a sanitized, structured logger built on the
// standard library only. It prevents log injection, redacts secrets,
// and caps field sizes. It never crashes or blocks request processing.
package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
	"unicode"
)

const (
	maxFieldBytes = 512  // per-field cap (SEC-05, SEC-09)
	maxEventBytes = 8192 // total log event cap (SEC-09)
)

// sensitiveKeys are header/field names whose values are always redacted.
var sensitiveKeys = map[string]bool{
	"authorization":   true,
	"cookie":          true,
	"set-cookie":      true,
	"proxy-authorize": true,
	"x-api-key":       true,
	"x-auth-token":    true,
	"secret":          true,
	"password":        true,
	"token":           true,
}

// Logger wraps stdlib log with sanitization and redaction.
type Logger struct {
	l *log.Logger
}

// New creates a Logger writing to stderr with timestamps.
func New() *Logger {
	return &Logger{l: log.New(os.Stderr, "", log.LstdFlags|log.LUTC)}
}

// sanitize removes control characters (including CR/LF) and caps length.
// This prevents log injection (SEC-09, T-11).
func sanitize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			b.WriteRune(' ')
		} else if unicode.IsControl(r) {
			// drop other control characters entirely
		} else {
			b.WriteRune(r)
		}
		if b.Len() >= maxFieldBytes {
			b.WriteString("...[truncated]")
			break
		}
	}
	return b.String()
}

// redact replaces a value with REDACTED if the key is sensitive.
func redact(key, value string) string {
	if sensitiveKeys[strings.ToLower(key)] {
		return "[REDACTED]"
	}
	return sanitize(value)
}

// Field represents a log key/value pair.
type Field struct {
	Key   string
	Value string
}

// F is a convenience constructor for Field.
func F(key, value string) Field { return Field{Key: sanitize(key), Value: redact(key, value)} }

// Info logs an informational event with optional fields.
func (lg *Logger) Info(event string, fields ...Field) {
	lg.emit("INFO", event, fields)
}

// Warn logs a warning event with optional fields.
func (lg *Logger) Warn(event string, fields ...Field) {
	lg.emit("WARN", event, fields)
}

// Error logs an error event with optional fields. Never panics.
func (lg *Logger) Error(event string, fields ...Field) {
	lg.emit("ERROR", event, fields)
}

func (lg *Logger) emit(level, event string, fields []Field) {
	// Build a simple key=value line; cap total size.
	var sb strings.Builder
	sb.WriteString(level)
	sb.WriteString(" event=")
	sb.WriteString(sanitize(event))
	for _, f := range fields {
		sb.WriteString(fmt.Sprintf(" %s=%q", f.Key, f.Value))
		if sb.Len() >= maxEventBytes {
			sb.WriteString(" ...[truncated]")
			break
		}
	}
	// stdlib log.Output never panics on write errors; logging must not block.
	_ = lg.l.Output(2, sb.String())
}
