package pinterest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type Downloader struct {
	log     *logrus.Logger
	tempDir string
	config  DownloaderConfig
}

type DownloaderConfig struct {
	GalleryDLPath   string
	TempDir         string
	MaxRetries      int
	Timeout         time.Duration
	RateLimit       string
	MaxPins         int
	SkipExisting    bool
	WriteMetadata   bool
	IncludeVideos   bool
}

func NewDownloader(logger *logrus.Logger, config DownloaderConfig) *Downloader {
	if config.GalleryDLPath == "" {
		config.GalleryDLPath = "gallery-dl"
	}
	if config.TempDir == "" {
		config.TempDir = "/tmp/pinterest-downloads"
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RateLimit == "" {
		config.RateLimit = "1.0-2.0"
	}
	if config.MaxPins == 0 {
		config.MaxPins = 120
	}
	
	config.WriteMetadata = true
	config.SkipExisting = false
	config.IncludeVideos = false

	return &Downloader{
		log:     logger,
		tempDir: config.TempDir,
		config:  config,
	}
}

func (d *Downloader) DownloadBoard(boardURL string) (*DownloadResult, error) {
	d.log.WithField("board_url", boardURL).Info("Starting Pinterest board download")

	if !d.isValidPinterestURL(boardURL) {
		return nil, fmt.Errorf("invalid Pinterest board URL: %s", boardURL)
	}

	downloadDir := filepath.Join(d.tempDir, d.generateDirName(boardURL))
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create download directory: %w", err)
	}
	defer os.RemoveAll(downloadDir)

	configPath, err := d.createGalleryDLConfig(downloadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create gallery-dl config: %w", err)
	}
	defer os.Remove(configPath)

	metadataFile := filepath.Join(downloadDir, "metadata.jsonl")
	
	cmd := exec.Command(d.config.GalleryDLPath,
		"--config", configPath,
		"--write-metadata",
		"--output", metadataFile,
		"--format", "json",
		"--no-download",
		boardURL,
	)

	d.log.WithField("command", cmd.String()).Debug("Executing gallery-dl command")

	output, err := cmd.CombinedOutput()
	if err != nil {
		d.log.WithError(err).WithField("output", string(output)).Error("gallery-dl command failed")
		return nil, fmt.Errorf("gallery-dl failed: %w\nOutput: %s", err, string(output))
	}

	pins, board, err := d.parseMetadata(metadataFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	if len(pins) > d.config.MaxPins {
		d.log.WithFields(logrus.Fields{
			"total_pins": len(pins),
			"max_pins":   d.config.MaxPins,
		}).Warn("Pinterest board has too many pins, truncating")
		pins = pins[:d.config.MaxPins]
	}

	result := &DownloadResult{
		Pins:         pins,
		Board:        board,
		SuccessCount: len(pins),
		FailedCount:  0,
		Errors:       nil,
	}

	d.log.WithFields(logrus.Fields{
		"board_name":    board.Name,
		"pins_count":    len(pins),
		"success_count": result.SuccessCount,
	}).Info("Pinterest board download completed")

	return result, nil
}

func (d *Downloader) parseMetadata(metadataFile string) ([]Pin, Board, error) {
	file, err := os.Open(metadataFile)
	if err != nil {
		return nil, Board{}, fmt.Errorf("failed to open metadata file: %w", err)
	}
	defer file.Close()

	var pins []Pin
	var board Board
	position := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var metadata GalleryDLMetadata
		if err := json.Unmarshal([]byte(line), &metadata); err != nil {
			d.log.WithError(err).WithField("line", line).Warn("Failed to parse metadata line")
			continue
		}

		if metadata.Category != "pinterest" || metadata.Subcategory != "board" {
			continue
		}

		if metadata.MediaType == "video" && !d.config.IncludeVideos {
			continue
		}

		if board.ID == "" {
			board = Board{
				ID:          metadata.Board.ID,
				Name:        metadata.Board.Name,
				URL:         metadata.Board.URL,
				Username:    metadata.User.Username,
				Description: "",
			}
		}

		createdAt := time.Now()
		if metadata.Date != "" {
			if parsed, err := time.Parse("2006-01-02", metadata.Date); err == nil {
				createdAt = parsed
			}
		}

		pin := Pin{
			ID:          metadata.ID,
			Title:       metadata.Title,
			Description: metadata.Description,
			ImageURL:    metadata.URL,
			LargeURL:    metadata.URL,
			OriginalURL: metadata.URL,
			BoardID:     metadata.Board.ID,
			BoardName:   metadata.Board.Name,
			Username:    metadata.User.Username,
			CreatedAt:   createdAt,
			Position:    position,
		}

		pins = append(pins, pin)
		position++
	}

	if err := scanner.Err(); err != nil {
		return nil, Board{}, fmt.Errorf("error reading metadata file: %w", err)
	}

	board.PinCount = len(pins)

	return pins, board, nil
}

func (d *Downloader) createGalleryDLConfig(downloadDir string) (string, error) {
	config := map[string]interface{}{
		"extractor": map[string]interface{}{
			"pinterest": map[string]interface{}{
				"board": map[string]interface{}{
					"sections": false,
					"videos":   d.config.IncludeVideos,
				},
			},
			"base-directory": downloadDir,
			"skip":           d.config.SkipExisting,
			"retries":        d.config.MaxRetries,
			"timeout":        int(d.config.Timeout.Seconds()),
			"rate":           d.config.RateLimit,
			"write-metadata": d.config.WriteMetadata,
		},
		"output": map[string]interface{}{
			"mode":     "json",
			"progress": false,
		},
	}

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(downloadDir, "gallery-dl.json")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return "", fmt.Errorf("failed to write config file: %w", err)
	}

	return configPath, nil
}

func (d *Downloader) isValidPinterestURL(url string) bool {
	pinterestPattern := regexp.MustCompile(`^https?://(?:www\.)?pinterest\.com/[^/]+/[^/]+/?$`)
	return pinterestPattern.MatchString(url)
}

func (d *Downloader) generateDirName(boardURL string) string {
	urlParts := strings.Split(strings.TrimSuffix(boardURL, "/"), "/")
	if len(urlParts) >= 2 {
		username := urlParts[len(urlParts)-2]
		boardName := urlParts[len(urlParts)-1]
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		return fmt.Sprintf("%s_%s_%s", username, boardName, timestamp[len(timestamp)-6:])
	}
	
	return fmt.Sprintf("pinterest_board_%d", time.Now().Unix())
}

func (d *Downloader) GetPinImageData(pin Pin) ([]byte, error) {
	cmd := exec.Command(d.config.GalleryDLPath,
		"--no-metadata",
		"--output", "-", 
		pin.ImageURL,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to download pin image: %w", err)
	}

	return output, nil
}