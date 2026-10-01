package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Connect(dbPath string) (*sql.DB, error) {
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create db directory: %w", err)
			}
		}
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25) // SQLite WAL mode allows concurrent readers

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := Migrate(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Printf("database connected and migrated successfully: %s", dbPath)
	return db, nil
}

func Migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		first_name TEXT DEFAULT '',
		avatar_url TEXT DEFAULT '',
		bio TEXT DEFAULT '',
		city TEXT DEFAULT '',
		postal_code TEXT DEFAULT '',
		latitude REAL DEFAULT 0.0,
		longitude REAL DEFAULT 0.0,
		is_verified INTEGER DEFAULT 0,
		is_banned INTEGER DEFAULT 0,
		role TEXT DEFAULT 'user',
		completed_trades_count INTEGER DEFAULT 0,
		rating_average REAL DEFAULT 0.0,
		rating_count INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS items (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		title TEXT NOT NULL,
		description TEXT DEFAULT '',
		category TEXT NOT NULL,
		condition TEXT NOT NULL,
		estimated_value REAL NOT NULL,
		min_swap_value REAL DEFAULT 0.0,
		trade_interests TEXT DEFAULT '[]',
		images TEXT DEFAULT '[]',
		city TEXT DEFAULT '',
		latitude REAL DEFAULT 0.0,
		longitude REAL DEFAULT 0.0,
		status TEXT DEFAULT 'available',
		views_count INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS favorites (
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, item_id)
	);

	CREATE TABLE IF NOT EXISTS swaps (
		id TEXT PRIMARY KEY,
		initiator_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		recipient_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		target_item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
		status TEXT NOT NULL DEFAULT 'pending',
		cash_adjustment REAL DEFAULT 0.0,
		message TEXT DEFAULT '',
		handover_method TEXT DEFAULT 'personal',
		initiator_confirmed INTEGER DEFAULT 0,
		recipient_confirmed INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS swap_items (
		swap_id TEXT NOT NULL REFERENCES swaps(id) ON DELETE CASCADE,
		item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
		role TEXT NOT NULL, -- 'offered' or 'target'
		PRIMARY KEY (swap_id, item_id)
	);

	CREATE TABLE IF NOT EXISTS swap_history (
		id TEXT PRIMARY KEY,
		swap_id TEXT NOT NULL REFERENCES swaps(id) ON DELETE CASCADE,
		actor_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		action TEXT NOT NULL,
		message TEXT DEFAULT '',
		cash_adjustment REAL DEFAULT 0.0,
		offered_item_ids TEXT DEFAULT '[]',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		swap_id TEXT NOT NULL REFERENCES swaps(id) ON DELETE CASCADE,
		sender_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		text TEXT DEFAULT '',
		image_url TEXT DEFAULT '',
		is_system INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS reviews (
		id TEXT PRIMARY KEY,
		swap_id TEXT NOT NULL REFERENCES swaps(id) ON DELETE CASCADE,
		reviewer_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		target_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		rating INTEGER NOT NULL,
		reliability INTEGER DEFAULT 5,
		communication INTEGER DEFAULT 5,
		item_condition INTEGER DEFAULT 5,
		adherence INTEGER DEFAULT 5,
		comment TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(swap_id, reviewer_id)
	);

	CREATE TABLE IF NOT EXISTS notifications (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		type TEXT NOT NULL,
		title TEXT NOT NULL,
		body TEXT NOT NULL,
		entity_type TEXT DEFAULT '',
		entity_id TEXT DEFAULT '',
		is_read INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS reports (
		id TEXT PRIMARY KEY,
		reporter_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		target_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
		target_item_id TEXT REFERENCES items(id) ON DELETE SET NULL,
		reason TEXT NOT NULL,
		description TEXT DEFAULT '',
		status TEXT DEFAULT 'open',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS blocks (
		blocker_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		blocked_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (blocker_id, blocked_id)
	);

	CREATE INDEX IF NOT EXISTS idx_items_user_id ON items(user_id);
	CREATE INDEX IF NOT EXISTS idx_items_status ON items(status);
	CREATE INDEX IF NOT EXISTS idx_items_category ON items(category);
	CREATE INDEX IF NOT EXISTS idx_swaps_initiator ON swaps(initiator_id);
	CREATE INDEX IF NOT EXISTS idx_swaps_recipient ON swaps(recipient_id);
	CREATE INDEX IF NOT EXISTS idx_messages_swap_id ON messages(swap_id);
	CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id, is_read);
	`

	_, err := db.Exec(schema)
	return err
}
