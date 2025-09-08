package image

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
)

type Processor struct {
	log        *logrus.Logger
	maxSize    int
	maxFileSize int64
}

type ProcessConfig struct {
	MaxSize     int   // Max width/height in pixels (default: 512)
	MaxFileSize int64 // Max file size in bytes (default: 512KB)
	Quality     int   // WebP quality 0-100 (default: 85)
}

func NewProcessor(logger *logrus.Logger, config ProcessConfig) *Processor {
	if config.MaxSize == 0 {
		config.MaxSize = 512
	}
	if config.MaxFileSize == 0 {
		config.MaxFileSize = 512 * 1024 // 512KB
	}
	if config.Quality == 0 {
		config.Quality = 85
	}

	return &Processor{
		log:         logger,
		maxSize:     config.MaxSize,
		maxFileSize: config.MaxFileSize,
	}
}

func (p *Processor) ProcessImageFromURL(imageURL string) ([]byte, error) {
	p.log.WithField("image_url", imageURL).Debug("Processing image from URL")

	imageData, err := p.downloadImage(imageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}

	return p.ProcessImageData(imageData)
}

func (p *Processor) ProcessImageData(imageData []byte) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	p.log.WithFields(logrus.Fields{
		"original_format": format,
		"original_size":   fmt.Sprintf("%dx%d", img.Bounds().Dx(), img.Bounds().Dy()),
	}).Debug("Decoded original image")

	processedImg := p.processForSticker(img)

	webpData, err := p.encodeAsWebP(processedImg, 85)
	if err != nil {
		return nil, fmt.Errorf("failed to encode as WebP: %w", err)
	}

	if int64(len(webpData)) > p.maxFileSize {
		p.log.WithField("original_size", len(webpData)).Debug("Image too large, reducing quality")
		
		quality := 75
		for quality >= 30 && int64(len(webpData)) > p.maxFileSize {
			webpData, err = p.encodeAsWebP(processedImg, quality)
			if err != nil {
				return nil, fmt.Errorf("failed to encode as WebP with quality %d: %w", quality, err)
			}
			quality -= 10
		}

		if int64(len(webpData)) > p.maxFileSize {
			return nil, fmt.Errorf("unable to compress image below %d bytes", p.maxFileSize)
		}
	}

	p.log.WithFields(logrus.Fields{
		"final_size":      len(webpData),
		"processed_dims":  fmt.Sprintf("%dx%d", processedImg.Bounds().Dx(), processedImg.Bounds().Dy()),
	}).Debug("Image processing completed")

	return webpData, nil
}

func (p *Processor) processForSticker(img image.Image) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	maxDim := float64(p.maxSize)
	aspectRatio := float64(width) / float64(height)

	var newWidth, newHeight int

	if math.Abs(aspectRatio-1.0) < 0.1 {
		newWidth = p.maxSize
		newHeight = p.maxSize
		resized := imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)
		return resized
	}

	if width > height {
		newWidth = int(maxDim)
		newHeight = int(maxDim / aspectRatio)
	} else {
		newHeight = int(maxDim)
		newWidth = int(maxDim * aspectRatio)
	}

	if newWidth > p.maxSize {
		newWidth = p.maxSize
		newHeight = int(float64(newWidth) / aspectRatio)
	}
	if newHeight > p.maxSize {
		newHeight = p.maxSize
		newWidth = int(float64(newHeight) * aspectRatio)
	}

	resized := imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)

	if newWidth == p.maxSize && newHeight == p.maxSize {
		return resized
	}

	canvas := image.NewRGBA(image.Rect(0, 0, p.maxSize, p.maxSize))
	
	transparent := color.RGBA{0, 0, 0, 0}
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{transparent}, image.Point{}, draw.Src)

	offsetX := (p.maxSize - newWidth) / 2
	offsetY := (p.maxSize - newHeight) / 2
	
	draw.Draw(canvas, image.Rect(offsetX, offsetY, offsetX+newWidth, offsetY+newHeight),
		resized, image.Point{}, draw.Over)

	return canvas
}

func (p *Processor) encodeAsWebP(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	
	options := &webp.Options{
		Lossless: false,
		Quality:  float32(quality),
	}

	err := webp.Encode(&buf, img, options)
	if err != nil {
		return nil, fmt.Errorf("failed to encode WebP: %w", err)
	}

	return buf.Bytes(), nil
}

func (p *Processor) downloadImage(imageURL string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Itanoru-Bot/1.0")
	req.Header.Set("Accept", "image/*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("received empty image data")
	}

	return data, nil
}

func (p *Processor) DetectImageFormat(data []byte) string {
	if len(data) < 12 {
		return "unknown"
	}

	if bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		return "png"
	}
	
	if bytes.HasPrefix(data, []byte("\xFF\xD8\xFF")) {
		return "jpeg"
	}
	
	if bytes.HasPrefix(data, []byte("RIFF")) && bytes.Contains(data[8:12], []byte("WEBP")) {
		return "webp"
	}
	
	if bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")) {
		return "gif"
	}

	return "unknown"
}

func (p *Processor) ConvertToRGBA(img image.Image) *image.RGBA {
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	return rgba
}

func (p *Processor) ValidateImageDimensions(img image.Image) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width == 0 || height == 0 {
		return fmt.Errorf("invalid image dimensions: %dx%d", width, height)
	}

	maxDimension := 2048
	if width > maxDimension || height > maxDimension {
		return fmt.Errorf("image too large: %dx%d (max: %dx%d)", width, height, maxDimension, maxDimension)
	}

	return nil
}

func (p *Processor) CreateThumbnail(img image.Image, size int) image.Image {
	return imaging.Thumbnail(img, size, size, imaging.Lanczos)
}

func (p *Processor) IsAnimated(data []byte) bool {
	format := p.DetectImageFormat(data)
	return format == "gif"
}

func (p *Processor) GetImageInfo(data []byte) (width, height int, format string, err error) {
	img, formatName, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	return bounds.Dx(), bounds.Dy(), formatName, nil
}