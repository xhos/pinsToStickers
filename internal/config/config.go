package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Config struct {
	// Telegram Bot Configuration
	BotToken string `mapstructure:"bot_token"`

	// Database Configuration  
	DatabasePath string `mapstructure:"database_path"`

	// Logging Configuration
	LogLevel  string `mapstructure:"log_level"`
	LogFormat string `mapstructure:"log_format"`

	// Sync Configuration
	SyncInterval       string        `mapstructure:"sync_interval"`
	MaxConcurrentSyncs int           `mapstructure:"max_concurrent_syncs"`
	SyncTimeout        time.Duration `mapstructure:"sync_timeout"`

	// Pinterest/Gallery-dl Configuration
	GalleryDLPath    string `mapstructure:"gallery_dl_path"`
	GalleryDLConfig  string `mapstructure:"gallery_dl_config"`
	RateLimit        string `mapstructure:"rate_limit"`
	MaxRetries       int    `mapstructure:"max_retries"`
	DownloadTimeout  int    `mapstructure:"download_timeout"`

	// Image Processing Configuration
	TempDir         string `mapstructure:"temp_dir"`
	MaxImageSize    int    `mapstructure:"max_image_size"`
	MaxFileSize     int64  `mapstructure:"max_file_size"`
	ImageQuality    int    `mapstructure:"image_quality"`
	MaxStickers     int    `mapstructure:"max_stickers"`

	// Server Configuration
	Port            int    `mapstructure:"port"`
	Host            string `mapstructure:"host"`
	EnableMetrics   bool   `mapstructure:"enable_metrics"`
	MetricsPath     string `mapstructure:"metrics_path"`

	// Performance Configuration
	WorkerPoolSize  int           `mapstructure:"worker_pool_size"`
	RequestTimeout  time.Duration `mapstructure:"request_timeout"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

func Load() (*Config, error) {
	config := &Config{
		// Set defaults
		BotToken:           "",
		DatabasePath:       "./data/bot.db",
		LogLevel:          "info",
		LogFormat:         "text",
		SyncInterval:      "@hourly",
		MaxConcurrentSyncs: 3,
		SyncTimeout:       10 * time.Minute,
		GalleryDLPath:     "gallery-dl",
		GalleryDLConfig:   "./config/gallery-dl.conf",
		RateLimit:         "1.0-2.0",
		MaxRetries:        3,
		DownloadTimeout:   30,
		TempDir:           "./temp",
		MaxImageSize:      512,
		MaxFileSize:       512 * 1024, // 512KB
		ImageQuality:      85,
		MaxStickers:       120,
		Port:             8080,
		Host:             "0.0.0.0",
		EnableMetrics:    false,
		MetricsPath:      "/metrics",
		WorkerPoolSize:   5,
		RequestTimeout:   30 * time.Second,
		CleanupInterval:  24 * time.Hour,
	}

	// Load from environment variables
	config.loadFromEnv()

	// Try to load from config file if it exists
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/itanoru")

	// Read config file if it exists
	if err := viper.ReadInConfig(); err == nil {
		if err := viper.Unmarshal(config); err != nil {
			return nil, err
		}
	}

	// Override with environment variables again (highest priority)
	config.loadFromEnv()

	// Validate required configuration
	if err := config.validate(); err != nil {
		return nil, err
	}

	return config, nil
}

func (c *Config) loadFromEnv() {
	if token := os.Getenv("BOT_TOKEN"); token != "" {
		c.BotToken = token
	}
	if dbPath := os.Getenv("DB_PATH"); dbPath != "" {
		c.DatabasePath = dbPath
	}
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		c.LogLevel = logLevel
	}
	if logFormat := os.Getenv("LOG_FORMAT"); logFormat != "" {
		c.LogFormat = logFormat
	}
	if syncInterval := os.Getenv("SYNC_INTERVAL"); syncInterval != "" {
		c.SyncInterval = syncInterval
	}
	if maxSyncs := os.Getenv("MAX_CONCURRENT_SYNCS"); maxSyncs != "" {
		if val, err := strconv.Atoi(maxSyncs); err == nil {
			c.MaxConcurrentSyncs = val
		}
	}
	if tempDir := os.Getenv("TEMP_DIR"); tempDir != "" {
		c.TempDir = tempDir
	}
	if rateLimit := os.Getenv("GALLERY_DL_RATE_LIMIT"); rateLimit != "" {
		c.RateLimit = rateLimit
	}
	if maxStickers := os.Getenv("MAX_STICKERS"); maxStickers != "" {
		if val, err := strconv.Atoi(maxStickers); err == nil {
			c.MaxStickers = val
		}
	}
	if port := os.Getenv("PORT"); port != "" {
		if val, err := strconv.Atoi(port); err == nil {
			c.Port = val
		}
	}
	if host := os.Getenv("HOST"); host != "" {
		c.Host = host
	}
	if enableMetrics := os.Getenv("ENABLE_METRICS"); enableMetrics == "true" {
		c.EnableMetrics = true
	}
}

func (c *Config) validate() error {
	if c.BotToken == "" {
		return fmt.Errorf("BOT_TOKEN is required")
	}

	// Validate log level
	_, err := logrus.ParseLevel(c.LogLevel)
	if err != nil {
		return fmt.Errorf("invalid log level: %s", c.LogLevel)
	}

	// Validate sync interval (cron expression)
	if c.SyncInterval == "" {
		c.SyncInterval = "@hourly"
	}

	// Validate numeric values
	if c.MaxConcurrentSyncs <= 0 {
		c.MaxConcurrentSyncs = 3
	}
	if c.MaxStickers <= 0 || c.MaxStickers > 120 {
		c.MaxStickers = 120
	}
	if c.Port <= 0 || c.Port > 65535 {
		c.Port = 8080
	}

	return nil
}

func (c *Config) GetLogLevel() logrus.Level {
	level, err := logrus.ParseLevel(c.LogLevel)
	if err != nil {
		return logrus.InfoLevel
	}
	return level
}

func (c *Config) IsDevelopment() bool {
	return c.LogLevel == "debug" || c.LogLevel == "trace"
}

func (c *Config) GetDatabaseURL() string {
	return c.DatabasePath
}

func (c *Config) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c *Config) GetTempDir() string {
	return c.TempDir
}

func (c *Config) String() string {
	// Create a safe copy of config without sensitive information
	safe := *c
	safe.BotToken = "***REDACTED***"
	
	return fmt.Sprintf(`Configuration:
  Bot Token: %s
  Database Path: %s
  Log Level: %s
  Sync Interval: %s
  Max Concurrent Syncs: %d
  Max Stickers: %d
  Temp Dir: %s
  Port: %d
  Gallery-dl Rate Limit: %s`,
		safe.BotToken,
		safe.DatabasePath,
		safe.LogLevel,
		safe.SyncInterval,
		safe.MaxConcurrentSyncs,
		safe.MaxStickers,
		safe.TempDir,
		safe.Port,
		safe.RateLimit,
	)
}