package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	globalLogger *slog.Logger
	mu           sync.Mutex
)

// New initializes the logger
func New(devMode bool) *slog.Logger {
	mu.Lock()
	defer mu.Unlock()

	var handler slog.Handler
	if devMode {
		handler = &DevHandler{
			w: os.Stdout,
		}
	} else {
		level := slog.LevelInfo
		if strings.ToLower(os.Getenv("LIGHTHOUSE_LOG_LEVEL")) == "debug" {
			level = slog.LevelDebug
		}
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}
	globalLogger = slog.New(handler)
	return globalLogger
}

// Get returns the global logger
func Get() *slog.Logger {
	mu.Lock()
	defer mu.Unlock()
	if globalLogger == nil {
		globalLogger = slog.Default()
	}
	return globalLogger
}

// DevHandler formats logs for human readability with color levels
type DevHandler struct {
	w      io.Writer
	attrs  []slog.Attr
	group  string
}

func (h *DevHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= slog.LevelDebug
}

func (h *DevHandler) Handle(ctx context.Context, r slog.Record) error {
	// Format time: 2006/01/02 15:04:05
	tStr := r.Time.Format("2006/01/02 15:04:05")

	// Format level with colors (6 characters wide total)
	var lvlStr string
	switch r.Level {
	case slog.LevelDebug:
		lvlStr = "\033[36mDEBUG \033[0m" // Cyan
	case slog.LevelInfo:
		lvlStr = "\033[32mINFO  \033[0m" // Green
	case slog.LevelWarn:
		lvlStr = "\033[33mWARN  \033[0m" // Yellow
	case slog.LevelError:
		lvlStr = "\033[31mERROR \033[0m" // Red
	default:
		lvlStr = fmt.Sprintf("%-6s", r.Level.String())
	}

	var component string
	var parts []string

	addAttr := func(a slog.Attr) {
		if a.Key == "component" {
			component = strings.ToUpper(a.Value.String())
		} else {
			valStr := a.Value.String()
			if strings.ContainsAny(valStr, " \t\n\r\"\\") {
				valStr = fmt.Sprintf("%q", valStr)
			}
			parts = append(parts, fmt.Sprintf("%s=%s", a.Key, valStr))
		}
	}

	for _, a := range h.attrs {
		addAttr(a)
	}

	r.Attrs(func(a slog.Attr) bool {
		addAttr(a)
		return true
	})

	compStr := ""
	if component != "" {
		compStr = fmt.Sprintf(" [%s]", component)
	}

	attrsStr := ""
	if len(parts) > 0 {
		attrsStr = " " + strings.Join(parts, " ")
	}

	line := fmt.Sprintf("%s %s%s %s%s\n", tStr, lvlStr, compStr, r.Message, attrsStr)
	_, err := io.WriteString(h.w, line)
	return err
}

func (h *DevHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &DevHandler{
		w:     h.w,
		attrs: newAttrs,
		group: h.group,
	}
}

func (h *DevHandler) WithGroup(name string) slog.Handler {
	return &DevHandler{
		w:     h.w,
		attrs: h.attrs,
		group: name,
	}
}
