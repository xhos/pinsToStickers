package logger

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

type Config struct {
	Level      string
	Format     string
	OutputPath string
	MaxSize    int64
	MaxAge     time.Duration
}

func New(config Config) (*logrus.Logger, error) {
	logger := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// Set log format
	switch config.Format {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	default:
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
		})
	}

	// Set output
	if config.OutputPath != "" {
		// Ensure log directory exists
		logDir := filepath.Dir(config.OutputPath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, err
		}

		logFile, err := os.OpenFile(config.OutputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}

		// Log to both file and stdout
		logger.SetOutput(io.MultiWriter(os.Stdout, logFile))
	} else {
		logger.SetOutput(os.Stdout)
	}

	return logger, nil
}

func NewDefault() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
	})
	return logger
}

func WithFields(logger *logrus.Logger, fields map[string]interface{}) *logrus.Entry {
	return logger.WithFields(fields)
}

func WithComponent(logger *logrus.Logger, component string) *logrus.Entry {
	return logger.WithField("component", component)
}

func WithRequestID(logger *logrus.Logger, requestID string) *logrus.Entry {
	return logger.WithField("request_id", requestID)
}

func WithUserID(logger *logrus.Logger, userID int64) *logrus.Entry {
	return logger.WithField("user_id", userID)
}

func WithPackID(logger *logrus.Logger, packID int) *logrus.Entry {
	return logger.WithField("pack_id", packID)
}