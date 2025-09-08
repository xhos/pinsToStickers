package sync

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xhos/itanoru/internal/database"
	"github.com/xhos/itanoru/internal/image"
	"github.com/xhos/itanoru/internal/pinterest"
	"github.com/xhos/itanoru/internal/telegram"
)

type Engine struct {
	db          *database.DB
	telegram    *telegram.Client
	pinterest   *pinterest.Downloader
	imageProc   *image.Processor
	log         *logrus.Logger
	maxStickers int
}

type SyncResult struct {
	PackID       int
	Added        int
	Removed      int
	Failed       int
	Success      bool
	Error        error
	Duration     time.Duration
}

func NewEngine(db *database.DB, tg *telegram.Client, pinterest *pinterest.Downloader, imageProc *image.Processor, logger *logrus.Logger) *Engine {
	return &Engine{
		db:          db,
		telegram:    tg,
		pinterest:   pinterest,
		imageProc:   imageProc,
		log:         logger,
		maxStickers: 120, // Telegram limit
	}
}

func (e *Engine) SyncPack(packID int) error {
	startTime := time.Now()
	
	e.log.WithField("pack_id", packID).Info("Starting pack synchronization")

	pack, err := e.getPackByID(packID)
	if err != nil {
		return fmt.Errorf("failed to get pack: %w", err)
	}

	if err := e.db.UpdatePackSyncStatus(packID, "syncing", ""); err != nil {
		e.log.WithError(err).Error("Failed to update pack status to syncing")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result := e.performSync(ctx, pack)
	
	e.logSyncResult(packID, result, time.Since(startTime))

	status := "completed"
	errorMsg := ""
	if !result.Success {
		status = "error"
		errorMsg = result.Error.Error()
	}

	if err := e.db.UpdatePackSyncStatus(packID, status, errorMsg); err != nil {
		e.log.WithError(err).Error("Failed to update pack sync status")
	}

	return result.Error
}

func (e *Engine) performSync(ctx context.Context, pack *database.StickerPack) *SyncResult {
	result := &SyncResult{
		PackID: pack.ID,
	}

	pinsResult, err := e.pinterest.DownloadBoard(pack.PinterestBoardURL)
	if err != nil {
		result.Error = fmt.Errorf("failed to download Pinterest board: %w", err)
		return result
	}

	e.log.WithFields(logrus.Fields{
		"pack_id":    pack.ID,
		"pins_count": len(pinsResult.Pins),
		"board_name": pinsResult.Board.Name,
	}).Info("Downloaded Pinterest board data")

	if len(pinsResult.Pins) > e.maxStickers {
		e.log.WithFields(logrus.Fields{
			"total_pins":   len(pinsResult.Pins),
			"max_stickers": e.maxStickers,
		}).Warn("Pinterest board has too many pins, truncating")
		pinsResult.Pins = pinsResult.Pins[:e.maxStickers]
	}

	existingStickers, err := e.db.GetPackStickers(pack.ID)
	if err != nil {
		result.Error = fmt.Errorf("failed to get existing stickers: %w", err)
		return result
	}

	toAdd, toRemove := e.calculateDifferences(pinsResult.Pins, existingStickers)

	e.log.WithFields(logrus.Fields{
		"pack_id":   pack.ID,
		"to_add":    len(toAdd),
		"to_remove": len(toRemove),
		"existing":  len(existingStickers),
	}).Info("Calculated sync differences")

	user, err := e.getUserByID(pack.UserID)
	if err != nil {
		result.Error = fmt.Errorf("failed to get user: %w", err)
		return result
	}

	packExists := len(existingStickers) > 0
	if !packExists && len(toAdd) > 0 {
		if err := e.createNewStickerPack(ctx, pack, user, toAdd[0]); err != nil {
			result.Error = fmt.Errorf("failed to create sticker pack: %w", err)
			return result
		}
		toAdd = toAdd[1:]
		result.Added++
	}

	for _, sticker := range toRemove {
		if err := e.removeSticker(ctx, sticker); err != nil {
			e.log.WithError(err).WithField("sticker_id", sticker.ID).Error("Failed to remove sticker")
			result.Failed++
		} else {
			result.Removed++
		}
	}

	for _, pin := range toAdd {
		if err := e.addSticker(ctx, pack, user, pin); err != nil {
			e.log.WithError(err).WithField("pin_id", pin.ID).Error("Failed to add sticker")
			result.Failed++
		} else {
			result.Added++
		}
		
		time.Sleep(200 * time.Millisecond)
	}

	result.Success = result.Error == nil

	e.log.WithFields(logrus.Fields{
		"pack_id": pack.ID,
		"added":   result.Added,
		"removed": result.Removed,
		"failed":  result.Failed,
		"success": result.Success,
	}).Info("Sync operation completed")

	return result
}

func (e *Engine) createNewStickerPack(ctx context.Context, pack *database.StickerPack, user *database.User, firstPin pinterest.Pin) error {
	e.log.WithFields(logrus.Fields{
		"pack_name": pack.TelegramPackName,
		"user_id":   user.TelegramID,
		"pin_id":    firstPin.ID,
	}).Info("Creating new Telegram sticker pack")

	imageData, err := e.imageProc.ProcessImageFromURL(firstPin.ImageURL)
	if err != nil {
		return fmt.Errorf("failed to process first pin image: %w", err)
	}

	emoji := e.getEmojiForPin(firstPin)
	
	err = e.telegram.CreateStickerSet(ctx, user.TelegramID, pack.TelegramPackName, pack.TelegramPackTitle, imageData, emoji)
	if err != nil {
		return fmt.Errorf("failed to create Telegram sticker set: %w", err)
	}

	sticker, err := e.db.CreateSticker(pack.ID, firstPin.ID, firstPin.ImageURL, firstPin.Title, 0)
	if err != nil {
		return fmt.Errorf("failed to create sticker in database: %w", err)
	}

	stickerSet, err := e.telegram.GetStickerSet(ctx, pack.TelegramPackName)
	if err == nil && len(stickerSet.Stickers) > 0 {
		fileID := stickerSet.Stickers[0].FileID
		if err := e.db.UpdateStickerTelegramID(sticker.ID, fileID); err != nil {
			e.log.WithError(err).Error("Failed to update sticker telegram ID")
		}
	}

	return nil
}

func (e *Engine) addSticker(ctx context.Context, pack *database.StickerPack, user *database.User, pin pinterest.Pin) error {
	e.log.WithField("pin_id", pin.ID).Debug("Adding sticker to pack")

	imageData, err := e.imageProc.ProcessImageFromURL(pin.ImageURL)
	if err != nil {
		return fmt.Errorf("failed to process pin image: %w", err)
	}

	emoji := e.getEmojiForPin(pin)

	err = e.telegram.AddStickerToSet(ctx, user.TelegramID, pack.TelegramPackName, imageData, emoji)
	if err != nil {
		return fmt.Errorf("failed to add sticker to Telegram set: %w", err)
	}

	sticker, err := e.db.CreateSticker(pack.ID, pin.ID, pin.ImageURL, pin.Title, pin.Position)
	if err != nil {
		return fmt.Errorf("failed to create sticker in database: %w", err)
	}

	stickerSet, err := e.telegram.GetStickerSet(ctx, pack.TelegramPackName)
	if err == nil && len(stickerSet.Stickers) > 0 {
		fileID := stickerSet.Stickers[len(stickerSet.Stickers)-1].FileID
		if err := e.db.UpdateStickerTelegramID(sticker.ID, fileID); err != nil {
			e.log.WithError(err).Error("Failed to update sticker telegram ID")
		}
	}

	return nil
}

func (e *Engine) removeSticker(ctx context.Context, sticker database.Sticker) error {
	e.log.WithField("sticker_id", sticker.ID).Debug("Removing sticker from pack")

	if sticker.TelegramFileID != "" {
		if err := e.telegram.DeleteStickerFromSet(ctx, sticker.TelegramFileID); err != nil {
			e.log.WithError(err).Error("Failed to delete sticker from Telegram set")
		}
	}

	if err := e.db.DeleteSticker(sticker.ID); err != nil {
		return fmt.Errorf("failed to delete sticker from database: %w", err)
	}

	return nil
}

func (e *Engine) calculateDifferences(pins []pinterest.Pin, existing []database.Sticker) (toAdd []pinterest.Pin, toRemove []database.Sticker) {
	pinIDSet := make(map[string]pinterest.Pin)
	for _, pin := range pins {
		pinIDSet[pin.ID] = pin
	}

	existingIDSet := make(map[string]database.Sticker)
	for _, sticker := range existing {
		existingIDSet[sticker.PinterestPinID] = sticker
	}

	for _, pin := range pins {
		if _, exists := existingIDSet[pin.ID]; !exists {
			toAdd = append(toAdd, pin)
		}
	}

	for _, sticker := range existing {
		if _, exists := pinIDSet[sticker.PinterestPinID]; !exists {
			toRemove = append(toRemove, sticker)
		}
	}

	return toAdd, toRemove
}

func (e *Engine) getEmojiForPin(pin pinterest.Pin) string {
	title := strings.ToLower(pin.Title + " " + pin.Description)
	
	if strings.Contains(title, "heart") || strings.Contains(title, "love") {
		return "❤️"
	}
	if strings.Contains(title, "happy") || strings.Contains(title, "smile") {
		return "😊"
	}
	if strings.Contains(title, "cute") || strings.Contains(title, "kawaii") {
		return "🥰"
	}
	if strings.Contains(title, "food") || strings.Contains(title, "recipe") {
		return "🍽️"
	}
	if strings.Contains(title, "art") || strings.Contains(title, "drawing") {
		return "🎨"
	}
	if strings.Contains(title, "nature") || strings.Contains(title, "plant") {
		return "🌿"
	}
	if strings.Contains(title, "cat") || strings.Contains(title, "kitten") {
		return "🐱"
	}
	if strings.Contains(title, "dog") || strings.Contains(title, "puppy") {
		return "🐶"
	}

	return "📌"
}

func (e *Engine) getPackByID(packID int) (*database.StickerPack, error) {
	// This is a simplified version - in real implementation, we'd need to add this method to the database
	// For now, we'll return a placeholder error
	return nil, fmt.Errorf("getPackByID method needs to be implemented in database layer")
}

func (e *Engine) getUserByID(userID int) (*database.User, error) {
	// This is a simplified version - in real implementation, we'd need to add this method to the database
	// For now, we'll return a placeholder error
	return nil, fmt.Errorf("getUserByID method needs to be implemented in database layer")
}

func (e *Engine) logSyncResult(packID int, result *SyncResult, duration time.Duration) {
	logFields := logrus.Fields{
		"pack_id":  packID,
		"added":    result.Added,
		"removed":  result.Removed,
		"failed":   result.Failed,
		"success":  result.Success,
		"duration": duration,
	}

	if result.Error != nil {
		logFields["error"] = result.Error.Error()
		e.log.WithFields(logFields).Error("Pack sync failed")
	} else {
		e.log.WithFields(logFields).Info("Pack sync completed successfully")
	}
}