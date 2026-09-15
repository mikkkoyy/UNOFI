package pricing

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/unofi/unofi/internal/database"
)

// SQLRepository implements Repository using SQLite.
type SQLRepository struct {
	db *database.DB
}

// NewSQLRepository creates a new SQL-backed package repository.
func NewSQLRepository(db *database.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

// Create inserts a new package.
func (r *SQLRepository) Create(pkg *Package) (*Package, error) {
	result, err := r.db.Exec(
		`INSERT INTO packages(name, description, price, mb_amount, time_seconds, is_active, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		pkg.Name, pkg.Description, pkg.Price, pkg.MBAmount, pkg.TimeSeconds,
		pkg.IsActive, time.Now(), time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("create package: %w", err)
	}

	pkg.ID, _ = result.LastInsertId()
	pkg.CreatedAt = time.Now()
	pkg.UpdatedAt = time.Now()
	return pkg, nil
}

// GetByID retrieves a package by ID.
func (r *SQLRepository) GetByID(id int64) (*Package, error) {
	row := r.db.QueryRow(
		`SELECT id, name, description, price, mb_amount, time_seconds, is_active, created_at, updated_at
		 FROM packages WHERE id = ?`, id,
	)
	return scanPackage(row)
}

// Update modifies a package.
func (r *SQLRepository) Update(pkg *Package) error {
	_, err := r.db.Exec(
		`UPDATE packages SET name = ?, description = ?, price = ?, mb_amount = ?,
		 time_seconds = ?, is_active = ?, updated_at = ? WHERE id = ?`,
		pkg.Name, pkg.Description, pkg.Price, pkg.MBAmount, pkg.TimeSeconds,
		pkg.IsActive, time.Now(), pkg.ID,
	)
	return err
}

// Delete removes a package.
func (r *SQLRepository) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM packages WHERE id = ?`, id)
	return err
}

// List returns packages, optionally filtered to active only.
func (r *SQLRepository) List(activeOnly bool) ([]*Package, error) {
	var query string
	if activeOnly {
		query = `SELECT id, name, description, price, mb_amount, time_seconds, is_active, created_at, updated_at
		        FROM packages WHERE is_active = 1 ORDER BY price ASC`
	} else {
		query = `SELECT id, name, description, price, mb_amount, time_seconds, is_active, created_at, updated_at
		        FROM packages ORDER BY price ASC`
	}

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packages []*Package
	for rows.Next() {
		pkg, err := scanPackage(rows)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}
	return packages, rows.Err()
}

func scanPackage(scanner interface{ Scan(dest ...interface{}) error }) (*Package, error) {
	var p Package
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(
		&p.ID, &p.Name, &p.Description, &p.Price, &p.MBAmount,
		&p.TimeSeconds, &p.IsActive, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan package: %w", err)
	}
	if createdAt.Valid {
		p.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		p.UpdatedAt = updatedAt.Time
	}
	return &p, nil
}
