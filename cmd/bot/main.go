package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xhos/itanoru/internal/bot"
	"github.com/xhos/itanoru/internal/config"
	"github.com/xhos/itanoru/internal/database"
	"github.com/xhos/itanoru/internal/image"
	"github.com/xhos/itanoru/internal/pinterest"
	"github.com/xhos/itanoru/internal/scheduler"
	"github.com/xhos/itanoru/internal/sync"
	"github.com/xhos/itanoru/internal/telegram"
	"github.com/xhos/itanoru/pkg/logger"
	"github.com/xhos/itanoru/pkg/utils"

	tgbot "github.com/go-telegram/bot"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New(logger.Config{
		Level:      cfg.LogLevel,
		Format:     cfg.LogFormat,
		OutputPath: "./logs/itanoru.log",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	log.Info("Starting Itanoru Pinterest-to-Telegram Sticker Bot")
	log.Info(cfg.String())

	// Ensure required directories exist
	if err := utils.EnsureDir(cfg.TempDir); err != nil {
		log.WithError(err).Fatal("Failed to create temp directory")
	}

	if err := utils.EnsureDir("./data"); err != nil {
		log.WithError(err).Fatal("Failed to create data directory")
	}

	if err := utils.EnsureDir("./logs"); err != nil {
		log.WithError(err).Fatal("Failed to create logs directory")
	}

	// Initialize database
	log.Info("Initializing database")
	db, err := database.NewDB(cfg.DatabasePath, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize database")
	}
	defer db.Close()

	// Initialize Telegram client
	log.Info("Initializing Telegram client")
	tgClient, err := telegram.NewClient(cfg.BotToken, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize Telegram client")
	}

	// Initialize Pinterest downloader
	log.Info("Initializing Pinterest downloader")
	pinterestDownloader := pinterest.NewDownloader(log, pinterest.DownloaderConfig{
		GalleryDLPath:   cfg.GalleryDLPath,
		TempDir:         cfg.TempDir,
		MaxRetries:      cfg.MaxRetries,
		Timeout:         time.Duration(cfg.DownloadTimeout) * time.Second,
		RateLimit:       cfg.RateLimit,
		MaxPins:         cfg.MaxStickers,
		SkipExisting:    false,
		WriteMetadata:   true,
		IncludeVideos:   false,
	})

	// Initialize image processor
	log.Info("Initializing image processor")
	imageProcessor := image.NewProcessor(log, image.ProcessConfig{
		MaxSize:     cfg.MaxImageSize,
		MaxFileSize: cfg.MaxFileSize,
		Quality:     cfg.ImageQuality,
	})

	// Initialize sync engine
	log.Info("Initializing sync engine")
	syncEngine := sync.NewEngine(db, tgClient, pinterestDownloader, imageProcessor, log)

	// Initialize scheduler
	log.Info("Initializing scheduler")
	syncScheduler := scheduler.New(syncEngine, db, log, scheduler.Config{
		SyncInterval:    cfg.SyncInterval,
		MaxConcurrent:   cfg.MaxConcurrentSyncs,
		EnableScheduler: true,
	})

	// Initialize bot handler
	log.Info("Initializing bot handler")
	botHandler := bot.NewHandler(db, log, syncEngine)

	// Set up bot with handler
	tgBot := tgClient.GetBot()
	tgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "", tgbot.MatchTypePrefix, botHandler.HandleUpdate)

	// Start scheduler
	log.Info("Starting scheduler")
	if err := syncScheduler.Start(cfg.SyncInterval); err != nil {
		log.WithError(err).Fatal("Failed to start scheduler")
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start bot in background
	go func() {
		log.Info("Starting Telegram bot")
		tgBot.Start(ctx)
	}()

	// Start cleanup routine
	go func() {
		ticker := time.NewTicker(cfg.CleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Debug("Running cleanup routine")
				if err := utils.CleanupTempFiles(cfg.TempDir, 24*time.Hour); err != nil {
					log.WithError(err).Error("Failed to cleanup temp files")
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	log.Info("Itanoru bot is running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	<-sigChan
	log.Info("Shutdown signal received, starting graceful shutdown...")

	// Cancel context to stop all goroutines
	cancel()

	// Stop scheduler
	log.Info("Stopping scheduler")
	syncScheduler.Stop()

	// Give some time for cleanup
	time.Sleep(2 * time.Second)

	log.Info("Itanoru bot stopped successfully")
}