package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps the application database connection.
type DB struct {
	conn *sql.DB
}

// Open initializes a new database connection with WAL mode.
func Open(dataPath string, busyTimeout int) (*DB, error) {
	if dataPath != ":memory:" {
		dir := filepath.Dir(dataPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create data directory: %w", err)
		}
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)", dataPath, busyTimeout)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	conn.SetMaxOpenConns(1)
	conn.SetConnMaxLifetime(0)

	return &DB{conn: conn}, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying sql.DB.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// Begin starts a new transaction.
func (db *DB) Begin() (*sql.Tx, error) {
	return db.conn.Begin()
}

// Exec executes a query without returning rows.
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.conn.Exec(query, args...)
}

// QueryRow executes a query that returns a single row.
func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.conn.QueryRow(query, args...)
}

// Query executes a query that returns multiple rows.
func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.conn.Query(query, args...)
}

// Migrate creates all required tables.
func (db *DB) Migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS admin_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS admin_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT NOT NULL UNIQUE,
			user_id INTEGER NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES admin_users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mac_addr TEXT NOT NULL UNIQUE,
			ip_addr TEXT DEFAULT '',
			hostname TEXT DEFAULT '',
			online INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			voucher_id INTEGER,
			payment_id INTEGER,
			mb_limit REAL DEFAULT 0,
			mb_used REAL DEFAULT 0,
			time_limit_seconds INTEGER DEFAULT 0,
			time_used_seconds INTEGER DEFAULT 0,
			expires_at DATETIME,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			session_id INTEGER,
			payment_method TEXT NOT NULL,
			amount REAL NOT NULL,
			mb_granted REAL DEFAULT 0,
			time_granted_seconds INTEGER DEFAULT 0,
			status TEXT DEFAULT 'completed',
			reference TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE SET NULL
		)`,
		`CREATE TABLE IF NOT EXISTS vouchers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT NOT NULL UNIQUE,
			package_id INTEGER NOT NULL,
			status TEXT DEFAULT 'unused',
			used_by_device_id INTEGER,
			used_at DATETIME,
			expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (used_by_device_id) REFERENCES devices(id) ON DELETE SET NULL
		)`,
		`CREATE TABLE IF NOT EXISTS packages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			price REAL NOT NULL,
			mb_amount REAL DEFAULT 0,
			time_seconds INTEGER DEFAULT 0,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_device ON sessions(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_device ON transactions(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_vouchers_code ON vouchers(code)`,
		`CREATE INDEX IF NOT EXISTS idx_vouchers_status ON vouchers(status)`,
		`CREATE INDEX IF NOT EXISTS idx_admin_sessions_token ON admin_sessions(token)`,
		`CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires ON admin_sessions(expires_at)`,
	}

	for _, stmt := range statements {
		if _, err := db.conn.Exec(stmt); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return nil
}
