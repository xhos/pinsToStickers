package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func GenerateRequestID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func SanitizeFilename(filename string) string {
	// Remove invalid characters for filenames
	reg := regexp.MustCompile(`[<>:"/\\|?*]`)
	sanitized := reg.ReplaceAllString(filename, "_")
	
	// Limit length to 100 characters
	if len(sanitized) > 100 {
		sanitized = sanitized[:100]
	}
	
	return sanitized
}

func EnsureDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}

func CleanupTempFiles(tempDir string, maxAge time.Duration) error {
	return filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && time.Since(info.ModTime()) > maxAge {
			return os.Remove(path)
		}
		
		return nil
	})
}

func TruncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	
	if maxLength <= 3 {
		return s[:maxLength]
	}
	
	return s[:maxLength-3] + "..."
}

func IsValidURL(url string) bool {
	urlPattern := regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
	return urlPattern.MatchString(url)
}

func IsPinterestBoardURL(url string) bool {
	pinterestPattern := regexp.MustCompile(`^https?://(?:www\.)?pinterest\.com/[^/]+/[^/]+/?$`)
	return pinterestPattern.MatchString(url)
}

func ExtractBoardInfoFromURL(url string) (username, boardName string, err error) {
	url = strings.TrimSuffix(url, "/")
	parts := strings.Split(url, "/")
	
	if len(parts) < 5 {
		return "", "", fmt.Errorf("invalid Pinterest board URL")
	}
	
	username = parts[len(parts)-2]
	boardName = parts[len(parts)-1]
	
	return username, boardName, nil
}

func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
}

func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func StringInSlice(str string, slice []string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func UniqueStrings(slice []string) []string {
	keys := make(map[string]bool)
	var unique []string
	
	for _, str := range slice {
		if !keys[str] {
			keys[str] = true
			unique = append(unique, str)
		}
	}
	
	return unique
}

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Retry(attempts int, sleep time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i < attempts-1 {
			time.Sleep(sleep)
			sleep *= 2 // Exponential backoff
		}
	}
	return err
}

func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func SafeStringPointer(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func SafeStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}