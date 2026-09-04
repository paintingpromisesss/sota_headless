package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// ParseLevel parses a string log level into slog.Level.
func ParseLevel(lvl string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(lvl)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "info":
		fallthrough
	default:
		return slog.LevelInfo
	}
}

// Setup initializes and sets the default slog logger with the given level, format, and writer.
func Setup(levelStr, formatStr string, out io.Writer) *slog.Logger {
	if out == nil {
		out = os.Stderr
	}
	level := ParseLevel(levelStr)

	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, a.Value.Time().Format("2006-01-02 15:04:05"))
			}
			return a
		},
	}

	var handler slog.Handler
	if strings.EqualFold(strings.TrimSpace(formatStr), "json") {
		handler = slog.NewJSONHandler(out, opts)
	} else {
		handler = slog.NewTextHandler(out, opts)
	}

	l := slog.New(handler)
	slog.SetDefault(l)
	return l
}
