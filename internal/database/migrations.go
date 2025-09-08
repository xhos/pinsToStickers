package database

import (
	"fmt"
)

func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			telegram_id INTEGER UNIQUE NOT NULL,
			username TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		`CREATE TABLE IF NOT EXISTS sticker_packs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			telegram_pack_name TEXT UNIQUE NOT NULL,
			pinterest_board_url TEXT NOT NULL,
			board_name TEXT,
			telegram_pack_title TEXT,
			sticker_count INTEGER DEFAULT 0,
			last_sync_at TIMESTAMP,
			sync_status TEXT DEFAULT 'pending',
			error_message TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id, pinterest_board_url)
		)`,
		
		`CREATE TABLE IF NOT EXISTS stickers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pack_id INTEGER NOT NULL,
			pinterest_pin_id TEXT NOT NULL,
			telegram_file_id TEXT,
			emoji TEXT DEFAULT '📌',
			position INTEGER NOT NULL,
			image_url TEXT,
			pin_title TEXT,
			sync_status TEXT DEFAULT 'pending',
			error_message TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (pack_id) REFERENCES sticker_packs(id) ON DELETE CASCADE,
			UNIQUE(pack_id, pinterest_pin_id)
		)`,
		
		`CREATE TABLE IF NOT EXISTS sync_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pack_id INTEGER NOT NULL,
			sync_type TEXT NOT NULL,
			status TEXT NOT NULL,
			pins_added INTEGER DEFAULT 0,
			pins_removed INTEGER DEFAULT 0,
			pins_failed INTEGER DEFAULT 0,
			error_message TEXT,
			started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP,
			FOREIGN KEY (pack_id) REFERENCES sticker_packs(id) ON DELETE CASCADE
		)`,
		
		`CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id)`,
		`CREATE INDEX IF NOT EXISTS idx_packs_user_id ON sticker_packs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_packs_sync_status ON sticker_packs(sync_status)`,
		`CREATE INDEX IF NOT EXISTS idx_stickers_pack_id ON stickers(pack_id)`,
		`CREATE INDEX IF NOT EXISTS idx_stickers_pin_id ON stickers(pinterest_pin_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_logs_pack_id ON sync_logs(pack_id)`,
	}

	for i, migration := range migrations {
		if _, err := db.conn.Exec(migration); err != nil {
			return fmt.Errorf("failed to execute migration %d: %w", i+1, err)
		}
	}

	db.log.Info("Database migrations completed successfully")
	return nil
}