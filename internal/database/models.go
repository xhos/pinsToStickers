package database

import (
	"time"
)

type User struct {
	ID         int       `json:"id" db:"id"`
	TelegramID int64     `json:"telegram_id" db:"telegram_id"`
	Username   string    `json:"username" db:"username"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

type StickerPack struct {
	ID                  int       `json:"id" db:"id"`
	UserID              int       `json:"user_id" db:"user_id"`
	TelegramPackName    string    `json:"telegram_pack_name" db:"telegram_pack_name"`
	PinterestBoardURL   string    `json:"pinterest_board_url" db:"pinterest_board_url"`
	BoardName           string    `json:"board_name" db:"board_name"`
	TelegramPackTitle   string    `json:"telegram_pack_title" db:"telegram_pack_title"`
	StickerCount        int       `json:"sticker_count" db:"sticker_count"`
	LastSyncAt          *time.Time `json:"last_sync_at" db:"last_sync_at"`
	SyncStatus          string    `json:"sync_status" db:"sync_status"`
	ErrorMessage        string    `json:"error_message" db:"error_message"`
	CreatedAt           time.Time `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time `json:"updated_at" db:"updated_at"`
}

type Sticker struct {
	ID               int       `json:"id" db:"id"`
	PackID           int       `json:"pack_id" db:"pack_id"`
	PinterestPinID   string    `json:"pinterest_pin_id" db:"pinterest_pin_id"`
	TelegramFileID   string    `json:"telegram_file_id" db:"telegram_file_id"`
	Emoji            string    `json:"emoji" db:"emoji"`
	Position         int       `json:"position" db:"position"`
	ImageURL         string    `json:"image_url" db:"image_url"`
	PinTitle         string    `json:"pin_title" db:"pin_title"`
	SyncStatus       string    `json:"sync_status" db:"sync_status"`
	ErrorMessage     string    `json:"error_message" db:"error_message"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

type SyncLog struct {
	ID            int       `json:"id" db:"id"`
	PackID        int       `json:"pack_id" db:"pack_id"`
	SyncType      string    `json:"sync_type" db:"sync_type"`
	Status        string    `json:"status" db:"status"`
	PinsAdded     int       `json:"pins_added" db:"pins_added"`
	PinsRemoved   int       `json:"pins_removed" db:"pins_removed"`
	PinsFailed    int       `json:"pins_failed" db:"pins_failed"`
	ErrorMessage  string    `json:"error_message" db:"error_message"`
	StartedAt     time.Time `json:"started_at" db:"started_at"`
	CompletedAt   *time.Time `json:"completed_at" db:"completed_at"`
}