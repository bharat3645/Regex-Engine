// File: logger/logger.go
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bharat3645/compliance-manager/config"
	"github.com/bharat3645/compliance-manager/types"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AppLogger is a custom logger that writes to slog and emits Wails events.
type AppLogger struct {
	ctx      context.Context
	slog     *slog.Logger
	source   string
	minLevel slog.Level
	logFile  *os.File
}

// New creates a new application logger.
func New(ctx context.Context, source string, levelStr string, logFilePath string) (*AppLogger, error) {
	minLevel := parseLogLevel(levelStr)
	writers := []io.Writer{os.Stdout}
	var logFile *os.File

	if logFilePath != "" {
		absLogPath, err := config.GetPath(logFilePath)
		if err != nil {
			return nil, err
		}
		
		// FIX: Ensure the directory for the log file exists before trying to create the file.
		// This is crucial for running the application from a clean build directory.
		logDir := filepath.Dir(absLogPath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, err
		}

		file, err := os.OpenFile(absLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		writers = append(writers, file)
		logFile = file
	}

	multiWriter := io.MultiWriter(writers...)
	slogHandler := slog.NewJSONHandler(multiWriter, nil)
	slogLogger := slog.New(slogHandler).With("source", source)

	return &AppLogger{
		ctx:      ctx,
		slog:     slogLogger,
		source:   source,
		minLevel: minLevel,
		logFile:  logFile,
	}, nil
}

func (l *AppLogger) Close() {
	if l.logFile != nil {
		l.logFile.Close()
	}
}

func (l *AppLogger) log(level slog.Level, msg string, args ...any) {
	if l.ctx != nil && level >= l.minLevel {
		entry := types.LogEntry{
			Time:    time.Now(),
			Level:   level.String(),
			Msg:     msg,
			Details: make(map[string]any),
		}
		for i := 0; i < len(args); i += 2 {
			if key, ok := args[i].(string); ok {
				entry.Details[key] = args[i+1]
			}
		}
		runtime.EventsEmit(l.ctx, "log:message", entry)
	}
	l.slog.Log(l.ctx, level, msg, args...)
}

func (l *AppLogger) Info(msg string, args ...any)  { l.log(slog.LevelInfo, msg, args...) }
func (l *AppLogger) Warn(msg string, args ...any)  { l.log(slog.LevelWarn, msg, args...) }
func (l *AppLogger) Error(msg string, args ...any) { l.log(slog.LevelError, msg, args...) }
func (l *AppLogger) Debug(msg string, args ...any) { l.log(slog.LevelDebug, msg, args...) }

func parseLogLevel(levelStr string) slog.Level {
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
