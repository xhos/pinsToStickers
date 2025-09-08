package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

type DB struct {
	conn *sql.DB
	log  *logrus.Logger
}

func NewDB(dbPath string, logger *logrus.Logger) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{
		conn: conn,
		log:  logger,
	}

	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) GetUserByTelegramID(telegramID int64) (*User, error) {
	query := `SELECT id, telegram_id, username, created_at, updated_at FROM users WHERE telegram_id = ?`
	
	var user User
	err := db.conn.QueryRow(query, telegramID).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.CreatedAt, &user.UpdatedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	return &user, nil
}

func (db *DB) CreateUser(telegramID int64, username string) (*User, error) {
	query := `INSERT INTO users (telegram_id, username, created_at, updated_at) 
			  VALUES (?, ?, ?, ?) RETURNING id, created_at, updated_at`
	
	now := time.Now()
	var user User
	err := db.conn.QueryRow(query, telegramID, username, now, now).Scan(
		&user.ID, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	
	user.TelegramID = telegramID
	user.Username = username
	return &user, nil
}

func (db *DB) GetOrCreateUser(telegramID int64, username string) (*User, error) {
	user, err := db.GetUserByTelegramID(telegramID)
	if err != nil {
		return nil, err
	}
	
	if user == nil {
		return db.CreateUser(telegramID, username)
	}
	
	return user, nil
}

func (db *DB) CreateStickerPack(userID int, packName, boardURL, boardTitle, packTitle string) (*StickerPack, error) {
	query := `INSERT INTO sticker_packs (user_id, telegram_pack_name, pinterest_board_url, board_name, telegram_pack_title, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id, created_at, updated_at`
	
	now := time.Now()
	var pack StickerPack
	err := db.conn.QueryRow(query, userID, packName, boardURL, boardTitle, packTitle, now, now).Scan(
		&pack.ID, &pack.CreatedAt, &pack.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sticker pack: %w", err)
	}
	
	pack.UserID = userID
	pack.TelegramPackName = packName
	pack.PinterestBoardURL = boardURL
	pack.BoardName = boardTitle
	pack.TelegramPackTitle = packTitle
	pack.SyncStatus = "pending"
	
	return &pack, nil
}

func (db *DB) GetUserPacks(userID int) ([]StickerPack, error) {
	query := `SELECT id, user_id, telegram_pack_name, pinterest_board_url, board_name, 
			  telegram_pack_title, sticker_count, last_sync_at, sync_status, error_message,
			  created_at, updated_at FROM sticker_packs WHERE user_id = ? ORDER BY created_at DESC`
	
	rows, err := db.conn.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user packs: %w", err)
	}
	defer rows.Close()
	
	var packs []StickerPack
	for rows.Next() {
		var pack StickerPack
		err := rows.Scan(
			&pack.ID, &pack.UserID, &pack.TelegramPackName, &pack.PinterestBoardURL,
			&pack.BoardName, &pack.TelegramPackTitle, &pack.StickerCount, &pack.LastSyncAt,
			&pack.SyncStatus, &pack.ErrorMessage, &pack.CreatedAt, &pack.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pack: %w", err)
		}
		packs = append(packs, pack)
	}
	
	return packs, nil
}

func (db *DB) GetPackByName(userID int, packName string) (*StickerPack, error) {
	query := `SELECT id, user_id, telegram_pack_name, pinterest_board_url, board_name,
			  telegram_pack_title, sticker_count, last_sync_at, sync_status, error_message,
			  created_at, updated_at FROM sticker_packs WHERE user_id = ? AND telegram_pack_name = ?`
	
	var pack StickerPack
	err := db.conn.QueryRow(query, userID, packName).Scan(
		&pack.ID, &pack.UserID, &pack.TelegramPackName, &pack.PinterestBoardURL,
		&pack.BoardName, &pack.TelegramPackTitle, &pack.StickerCount, &pack.LastSyncAt,
		&pack.SyncStatus, &pack.ErrorMessage, &pack.CreatedAt, &pack.UpdatedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pack: %w", err)
	}
	
	return &pack, nil
}

func (db *DB) UpdatePackSyncStatus(packID int, status string, errorMessage string) error {
	query := `UPDATE sticker_packs SET sync_status = ?, error_message = ?, updated_at = ? WHERE id = ?`
	
	_, err := db.conn.Exec(query, status, errorMessage, time.Now(), packID)
	if err != nil {
		return fmt.Errorf("failed to update pack sync status: %w", err)
	}
	
	return nil
}

func (db *DB) GetPackStickers(packID int) ([]Sticker, error) {
	query := `SELECT id, pack_id, pinterest_pin_id, telegram_file_id, emoji, position,
			  image_url, pin_title, sync_status, error_message, created_at, updated_at
			  FROM stickers WHERE pack_id = ? ORDER BY position`
	
	rows, err := db.conn.Query(query, packID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pack stickers: %w", err)
	}
	defer rows.Close()
	
	var stickers []Sticker
	for rows.Next() {
		var sticker Sticker
		err := rows.Scan(
			&sticker.ID, &sticker.PackID, &sticker.PinterestPinID, &sticker.TelegramFileID,
			&sticker.Emoji, &sticker.Position, &sticker.ImageURL, &sticker.PinTitle,
			&sticker.SyncStatus, &sticker.ErrorMessage, &sticker.CreatedAt, &sticker.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sticker: %w", err)
		}
		stickers = append(stickers, sticker)
	}
	
	return stickers, nil
}

func (db *DB) CreateSticker(packID int, pinID, imageURL, title string, position int) (*Sticker, error) {
	query := `INSERT INTO stickers (pack_id, pinterest_pin_id, image_url, pin_title, position, emoji, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id, created_at, updated_at`
	
	now := time.Now()
	var sticker Sticker
	err := db.conn.QueryRow(query, packID, pinID, imageURL, title, position, "📌", now, now).Scan(
		&sticker.ID, &sticker.CreatedAt, &sticker.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sticker: %w", err)
	}
	
	sticker.PackID = packID
	sticker.PinterestPinID = pinID
	sticker.ImageURL = imageURL
	sticker.PinTitle = title
	sticker.Position = position
	sticker.Emoji = "📌"
	sticker.SyncStatus = "pending"
	
	return &sticker, nil
}

func (db *DB) UpdateStickerTelegramID(stickerID int, telegramFileID string) error {
	query := `UPDATE stickers SET telegram_file_id = ?, sync_status = 'uploaded', updated_at = ? WHERE id = ?`
	
	_, err := db.conn.Exec(query, telegramFileID, time.Now(), stickerID)
	if err != nil {
		return fmt.Errorf("failed to update sticker telegram ID: %w", err)
	}
	
	return nil
}

func (db *DB) DeleteSticker(stickerID int) error {
	query := `DELETE FROM stickers WHERE id = ?`
	
	_, err := db.conn.Exec(query, stickerID)
	if err != nil {
		return fmt.Errorf("failed to delete sticker: %w", err)
	}
	
	return nil
}