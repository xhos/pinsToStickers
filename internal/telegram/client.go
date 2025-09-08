package telegram

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/sirupsen/logrus"
)

type Client struct {
	bot *bot.Bot
	log *logrus.Logger
}

func NewClient(token string, logger *logrus.Logger) (*Client, error) {
	opts := []bot.Option{
		bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {}),
	}

	b, err := bot.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	return &Client{
		bot: b,
		log: logger,
	}, nil
}

func (c *Client) CreateStickerSet(ctx context.Context, userID int64, name, title string, stickerData []byte, emoji string) error {
	c.log.WithFields(logrus.Fields{
		"user_id": userID,
		"name":    name,
		"title":   title,
		"emoji":   emoji,
	}).Info("Creating new sticker set")

	params := &bot.CreateNewStickerSetParams{
		UserID:        userID,
		Name:          name,
		Title:         title,
		StickerFormat: models.StickerFormatStatic,
		Stickers: []models.InputSticker{
			{
				Sticker:       &models.InputFileUpload{Filename: "sticker.webp", Data: bytes.NewReader(stickerData)},
				EmojiList:     []string{emoji},
				Format:        models.StickerFormatStatic,
			},
		},
	}

	ok, err := c.bot.CreateNewStickerSet(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to create sticker set: %w", err)
	}

	if !ok {
		return fmt.Errorf("telegram returned false for sticker set creation")
	}

	c.log.WithField("name", name).Info("Successfully created sticker set")
	return nil
}

func (c *Client) AddStickerToSet(ctx context.Context, userID int64, name string, stickerData []byte, emoji string) error {
	c.log.WithFields(logrus.Fields{
		"user_id": userID,
		"name":    name,
		"emoji":   emoji,
	}).Debug("Adding sticker to set")

	sticker := models.InputSticker{
		Sticker:   &models.InputFileUpload{Filename: "sticker.webp", Data: bytes.NewReader(stickerData)},
		EmojiList: []string{emoji},
		Format:    models.StickerFormatStatic,
	}

	params := &bot.AddStickerToSetParams{
		UserID:  userID,
		Name:    name,
		Sticker: sticker,
	}

	ok, err := c.bot.AddStickerToSet(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to add sticker to set: %w", err)
	}

	if !ok {
		return fmt.Errorf("telegram returned false for adding sticker to set")
	}

	return nil
}

func (c *Client) DeleteStickerFromSet(ctx context.Context, stickerFileID string) error {
	c.log.WithField("sticker_file_id", stickerFileID).Debug("Deleting sticker from set")

	params := &bot.DeleteStickerFromSetParams{
		Sticker: stickerFileID,
	}

	ok, err := c.bot.DeleteStickerFromSet(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to delete sticker from set: %w", err)
	}

	if !ok {
		return fmt.Errorf("telegram returned false for deleting sticker from set")
	}

	return nil
}

func (c *Client) GetStickerSet(ctx context.Context, name string) (*models.StickerSet, error) {
	c.log.WithField("name", name).Debug("Getting sticker set")

	params := &bot.GetStickerSetParams{
		Name: name,
	}

	stickerSet, err := c.bot.GetStickerSet(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get sticker set: %w", err)
	}

	return stickerSet, nil
}

func (c *Client) SetStickerPositionInSet(ctx context.Context, stickerFileID string, position int) error {
	c.log.WithFields(logrus.Fields{
		"sticker_file_id": stickerFileID,
		"position":        position,
	}).Debug("Setting sticker position in set")

	params := &bot.SetStickerPositionInSetParams{
		Sticker:  stickerFileID,
		Position: position,
	}

	ok, err := c.bot.SetStickerPositionInSet(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to set sticker position: %w", err)
	}

	if !ok {
		return fmt.Errorf("telegram returned false for setting sticker position")
	}

	return nil
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	params := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}

	_, err := c.bot.SendMessage(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

func (c *Client) UploadStickerFile(ctx context.Context, userID int64, stickerData []byte) (string, error) {
	c.log.WithField("user_id", userID).Debug("Uploading sticker file")

	params := &bot.UploadStickerFileParams{
		UserID:        userID,
		Sticker:       &models.InputFileUpload{Filename: "sticker.webp", Data: bytes.NewReader(stickerData)},
		StickerFormat: models.StickerFormatStatic,
	}

	file, err := c.bot.UploadStickerFile(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to upload sticker file: %w", err)
	}

	return file.FileID, nil
}

func (c *Client) ValidateStickerSet(ctx context.Context, name string) error {
	stickerSet, err := c.GetStickerSet(ctx, name)
	if err != nil {
		return fmt.Errorf("sticker set validation failed: %w", err)
	}

	if stickerSet == nil {
		return fmt.Errorf("sticker set not found")
	}

	c.log.WithFields(logrus.Fields{
		"name":           stickerSet.Name,
		"title":          stickerSet.Title,
		"sticker_count":  len(stickerSet.Stickers),
	}).Debug("Sticker set validation successful")

	return nil
}

func (c *Client) RetryWithBackoff(ctx context.Context, operation func() error, maxRetries int) error {
	var err error
	backoff := time.Second

	for i := 0; i < maxRetries; i++ {
		err = operation()
		if err == nil {
			return nil
		}

		c.log.WithFields(logrus.Fields{
			"attempt":     i + 1,
			"max_retries": maxRetries,
			"backoff":     backoff,
			"error":       err,
		}).Warn("Operation failed, retrying with backoff")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries, err)
}

func (c *Client) GetBot() *bot.Bot {
	return c.bot
}

type StickerUploadResult struct {
	FileID   string
	Position int
	Success  bool
	Error    error
}

func (c *Client) BatchUploadStickers(ctx context.Context, userID int64, stickerDataList [][]byte, emojis []string) []StickerUploadResult {
	results := make([]StickerUploadResult, len(stickerDataList))

	for i, stickerData := range stickerDataList {
		emoji := "📌"
		if i < len(emojis) && emojis[i] != "" {
			emoji = emojis[i]
		}

		fileID, err := c.UploadStickerFile(ctx, userID, stickerData)
		results[i] = StickerUploadResult{
			FileID:   fileID,
			Position: i,
			Success:  err == nil,
			Error:    err,
		}

		if err != nil {
			c.log.WithFields(logrus.Fields{
				"position": i,
				"error":    err,
			}).Error("Failed to upload sticker in batch")
		}

		time.Sleep(100 * time.Millisecond)
	}

	return results
}

func (c *Client) IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	return fmt.Sprintf("%v", errStr) == "Too Many Requests" ||
		   fmt.Sprintf("%v", errStr) == "FLOOD_WAIT" ||
		   fmt.Sprintf("%v", errStr) == "STICKERSET_INVALID"
}

func (c *Client) GetRateLimitWaitTime(err error) time.Duration {
	if !c.IsRateLimitError(err) {
		return 0
	}

	return 5 * time.Second
}