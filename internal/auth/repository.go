package auth

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/unofi/unofi/internal/database"
)

// AdminUser represents an administrator account.
type AdminUser struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Repository defines admin user persistence operations.
type Repository interface {
	Create(username, passwordHash string) (*AdminUser, error)
	GetByID(id int64) (*AdminUser, error)
	GetByUsername(username string) (*AdminUser, error)
	UpdatePassword(id int64, passwordHash string) error
	Delete(id int64) error
	Count() (int64, error)
}

// SQLRepository implements Repository using SQLite.
type SQLRepository struct {
	db *database.DB
}

// NewSQLRepository creates a new SQL-backed admin repository.
func NewSQLRepository(db *database.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

// Create inserts a new admin user.
func (r *SQLRepository) Create(username, passwordHash string) (*AdminUser, error) {
	result, err := r.db.Exec(
		`INSERT INTO admin_users(username, password_hash, created_at, updated_at)
		 VALUES(?, ?, ?, ?)`,
		username, passwordHash, time.Now(), time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("create admin: %w", err)
	}

	id, _ := result.LastInsertId()
	return &AdminUser{
		ID:           id,
		Username:     username,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}, nil
}

// GetByID retrieves an admin user by ID.
func (r *SQLRepository) GetByID(id int64) (*AdminUser, error) {
	row := r.db.QueryRow(
		`SELECT id, username, password_hash, created_at, updated_at
		 FROM admin_users WHERE id = ?`, id,
	)
	return scanAdmin(row)
}

// GetByUsername retrieves an admin user by username.
func (r *SQLRepository) GetByUsername(username string) (*AdminUser, error) {
	row := r.db.QueryRow(
		`SELECT id, username, password_hash, created_at, updated_at
		 FROM admin_users WHERE username = ?`, username,
	)
	return scanAdmin(row)
}

// UpdatePassword changes an admin user's password.
func (r *SQLRepository) UpdatePassword(id int64, passwordHash string) error {
	_, err := r.db.Exec(
		`UPDATE admin_users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		passwordHash, time.Now(), id,
	)
	return err
}

// Delete removes an admin user.
func (r *SQLRepository) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM admin_users WHERE id = ?`, id)
	return err
}

// Count returns the total number of admin users.
func (r *SQLRepository) Count() (int64, error) {
	var count int64
	err := r.db.QueryRow(`SELECT COUNT(*) FROM admin_users`).Scan(&count)
	return count, err
}

func scanAdmin(scanner interface{ Scan(dest ...interface{}) error }) (*AdminUser, error) {
	var u AdminUser
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&u.ID, &u.Username, &u.PasswordHash, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan admin: %w", err)
	}
	if createdAt.Valid {
		u.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		u.UpdatedAt = updatedAt.Time
	}
	return &u, nil
}
