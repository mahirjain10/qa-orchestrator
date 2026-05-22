package logging

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var logFile *os.File

func Init(logDir string) error {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	logPath := filepath.Join(logDir, "app.log")

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	logFile = f

	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	multi := zerolog.MultiLevelWriter(f, os.Stderr)
	log.Logger = zerolog.New(multi).With().Timestamp().Logger()

	return nil
}

func InitFileOnly(logDir string) error {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	logPath := filepath.Join(logDir, "app.log")

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	logFile = f

	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = zerolog.New(f).With().Timestamp().Logger()

	return nil
}

func Close() {
	if logFile != nil {
		logFile.Close()
	}
}
