package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"github.com/xhos/itanoru/internal/database"
	"github.com/xhos/itanoru/internal/sync"
)

type Scheduler struct {
	cron         *cron.Cron
	syncEngine   *sync.Engine
	db           *database.DB
	log          *logrus.Logger
	running      bool
	mutex        sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	maxConcurrent int
	semaphore    chan struct{}
}

type Config struct {
	SyncInterval   string // Cron expression (default: "@hourly")
	MaxConcurrent  int    // Max concurrent syncs (default: 3)
	EnableScheduler bool   // Enable automatic scheduling (default: true)
}

func New(syncEngine *sync.Engine, db *database.DB, logger *logrus.Logger, config Config) *Scheduler {
	if config.SyncInterval == "" {
		config.SyncInterval = "@hourly"
	}
	if config.MaxConcurrent == 0 {
		config.MaxConcurrent = 3
	}
	if !config.EnableScheduler {
		config.EnableScheduler = true
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler{
		cron:          cron.New(cron.WithSeconds()),
		syncEngine:    syncEngine,
		db:            db,
		log:           logger,
		ctx:           ctx,
		cancel:        cancel,
		maxConcurrent: config.MaxConcurrent,
		semaphore:     make(chan struct{}, config.MaxConcurrent),
	}
}

func (s *Scheduler) Start(syncInterval string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		s.log.Warn("Scheduler is already running")
		return nil
	}

	if syncInterval == "" {
		syncInterval = "@hourly"
	}

	s.log.WithField("interval", syncInterval).Info("Starting scheduler")

	_, err := s.cron.AddFunc(syncInterval, s.syncAllPacks)
	if err != nil {
		return err
	}

	s.cron.Start()
	s.running = true

	s.log.Info("Scheduler started successfully")
	return nil
}

func (s *Scheduler) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		s.log.Warn("Scheduler is not running")
		return
	}

	s.log.Info("Stopping scheduler")

	s.cron.Stop()
	s.cancel()
	s.running = false

	s.log.Info("Scheduler stopped successfully")
}

func (s *Scheduler) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}

func (s *Scheduler) syncAllPacks() {
	s.log.Info("Starting scheduled sync for all packs")

	packs, err := s.getAllActivePacks()
	if err != nil {
		s.log.WithError(err).Error("Failed to get active packs for scheduled sync")
		return
	}

	if len(packs) == 0 {
		s.log.Debug("No active packs found for scheduled sync")
		return
	}

	s.log.WithField("pack_count", len(packs)).Info("Found active packs for scheduled sync")

	var wg sync.WaitGroup
	successCount := 0
	failCount := 0
	var mutex sync.Mutex

	for _, pack := range packs {
		wg.Add(1)
		go func(p database.StickerPack) {
			defer wg.Done()

			s.semaphore <- struct{}{}
			defer func() { <-s.semaphore }()

			if err := s.syncEngine.SyncPack(p.ID); err != nil {
				s.log.WithError(err).WithFields(logrus.Fields{
					"pack_id":   p.ID,
					"pack_name": p.TelegramPackName,
				}).Error("Scheduled sync failed for pack")
				
				mutex.Lock()
				failCount++
				mutex.Unlock()
			} else {
				s.log.WithFields(logrus.Fields{
					"pack_id":   p.ID,
					"pack_name": p.TelegramPackName,
				}).Info("Scheduled sync completed successfully for pack")
				
				mutex.Lock()
				successCount++
				mutex.Unlock()
			}

			time.Sleep(500 * time.Millisecond)
		}(pack)
	}

	wg.Wait()

	s.log.WithFields(logrus.Fields{
		"total_packs": len(packs),
		"success":     successCount,
		"failed":      failCount,
	}).Info("Scheduled sync batch completed")
}

func (s *Scheduler) SyncPackManual(packID int) error {
	s.log.WithField("pack_id", packID).Info("Starting manual sync for pack")

	select {
	case s.semaphore <- struct{}{}:
		defer func() { <-s.semaphore }()
	case <-time.After(30 * time.Second):
		return fmt.Errorf("sync queue is full, try again later")
	}

	err := s.syncEngine.SyncPack(packID)
	if err != nil {
		s.log.WithError(err).WithField("pack_id", packID).Error("Manual sync failed")
		return err
	}

	s.log.WithField("pack_id", packID).Info("Manual sync completed successfully")
	return nil
}

func (s *Scheduler) SyncAllPacksManual(userID int) (int, int, error) {
	s.log.WithField("user_id", userID).Info("Starting manual sync for all user packs")

	packs, err := s.db.GetUserPacks(userID)
	if err != nil {
		return 0, 0, err
	}

	if len(packs) == 0 {
		s.log.WithField("user_id", userID).Debug("No packs found for user")
		return 0, 0, nil
	}

	var wg sync.WaitGroup
	successCount := 0
	failCount := 0
	var mutex sync.Mutex

	for _, pack := range packs {
		wg.Add(1)
		go func(p database.StickerPack) {
			defer wg.Done()

			select {
			case s.semaphore <- struct{}{}:
				defer func() { <-s.semaphore }()
			case <-time.After(10 * time.Second):
				s.log.WithField("pack_id", p.ID).Warn("Sync queue timeout, skipping pack")
				mutex.Lock()
				failCount++
				mutex.Unlock()
				return
			}

			if err := s.syncEngine.SyncPack(p.ID); err != nil {
				s.log.WithError(err).WithField("pack_id", p.ID).Error("Manual sync failed for pack")
				mutex.Lock()
				failCount++
				mutex.Unlock()
			} else {
				mutex.Lock()
				successCount++
				mutex.Unlock()
			}

			time.Sleep(300 * time.Millisecond)
		}(pack)
	}

	wg.Wait()

	s.log.WithFields(logrus.Fields{
		"user_id":     userID,
		"total_packs": len(packs),
		"success":     successCount,
		"failed":      failCount,
	}).Info("Manual sync all completed")

	return successCount, failCount, nil
}

func (s *Scheduler) getAllActivePacks() ([]database.StickerPack, error) {
	// This would need to be implemented in the database layer
	// For now, we'll create a simplified version that gets all packs
	// In a real implementation, this would filter for active packs only
	
	// We need to implement a method to get all packs across all users
	// This is a placeholder that would need proper implementation
	return []database.StickerPack{}, nil
}

func (s *Scheduler) GetSyncStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return map[string]interface{}{
		"running":         s.running,
		"max_concurrent":  s.maxConcurrent,
		"queue_capacity":  cap(s.semaphore),
		"queue_used":      len(s.semaphore),
		"next_scheduled":  s.getNextScheduledTime(),
	}
}

func (s *Scheduler) getNextScheduledTime() *time.Time {
	entries := s.cron.Entries()
	if len(entries) == 0 {
		return nil
	}

	next := entries[0].Next
	return &next
}

func (s *Scheduler) AddSyncJob(packID int, delay time.Duration) {
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			if err := s.SyncPackManual(packID); err != nil {
				s.log.WithError(err).WithField("pack_id", packID).Error("Delayed sync job failed")
			}
		case <-s.ctx.Done():
			s.log.WithField("pack_id", packID).Debug("Delayed sync job cancelled")
		}
	}()
}

func (s *Scheduler) GetQueueStatus() (used int, capacity int) {
	return len(s.semaphore), cap(s.semaphore)
}

func (s *Scheduler) WaitForSlot(timeout time.Duration) error {
	select {
	case s.semaphore <- struct{}{}:
		<-s.semaphore
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for sync slot")
	}
}