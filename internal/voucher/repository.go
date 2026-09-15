package voucher

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/unofi/unofi/internal/database"
)

// SQLRepository implements VoucherRepository using SQLite.
type SQLRepository struct {
	db *database.DB
}

// NewSQLRepository creates a new SQL-backed voucher repository.
func NewSQLRepository(db *database.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

// Create inserts a new voucher.
func (r *SQLRepository) Create(v *Voucher) (*Voucher, error) {
	var expiresAt interface{}
	if !v.ExpiresAt.IsZero() {
		expiresAt = v.ExpiresAt
	}

	status := v.Status
	if status == "" {
		status = VoucherUnused
	}

	result, err := r.db.Exec(
		`INSERT INTO vouchers(code, package_id, status, expires_at, created_at)
		 VALUES(?, ?, ?, ?, ?)`,
		v.Code, v.PackageID, status, expiresAt, time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("create voucher: %w", err)
	}

	v.ID, _ = result.LastInsertId()
	v.Status = status
	v.CreatedAt = time.Now()
	return v, nil
}

// GetByCode retrieves a voucher by its code.
func (r *SQLRepository) GetByCode(code string) (*Voucher, error) {
	row := r.db.QueryRow(
		`SELECT id, code, package_id, status, used_by_device_id, used_at, expires_at, created_at
		 FROM vouchers WHERE code = ?`, code,
	)
	return scanVoucher(row)
}

// MarkUsed marks a voucher as used by a device.
func (r *SQLRepository) MarkUsed(code string, deviceID int64) error {
	_, err := r.db.Exec(
		`UPDATE vouchers SET status = ?, used_by_device_id = ?, used_at = ? WHERE code = ? AND status = ?`,
		VoucherUsed, deviceID, time.Now(), code, VoucherUnused,
	)
	return err
}

// List returns a paginated list of vouchers.
func (r *SQLRepository) List(limit, offset int) ([]*Voucher, error) {
	rows, err := r.db.Query(
		`SELECT id, code, package_id, status, used_by_device_id, used_at, expires_at, created_at
		 FROM vouchers ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vouchers []*Voucher
	for rows.Next() {
		v, err := scanVoucher(rows)
		if err != nil {
			return nil, err
		}
		vouchers = append(vouchers, v)
	}
	return vouchers, rows.Err()
}

// Count returns the total number of vouchers.
func (r *SQLRepository) Count() (int64, error) {
	var count int64
	err := r.db.QueryRow(`SELECT COUNT(*) FROM vouchers`).Scan(&count)
	return count, err
}

func scanVoucher(scanner interface{ Scan(dest ...interface{}) error }) (*Voucher, error) {
	var v Voucher
	var usedByDeviceID sql.NullInt64
	var usedAt, expiresAt, createdAt sql.NullTime

	err := scanner.Scan(
		&v.ID, &v.Code, &v.PackageID, &v.Status,
		&usedByDeviceID, &usedAt, &expiresAt, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan voucher: %w", err)
	}

	if usedByDeviceID.Valid {
		v.UsedByDeviceID = &usedByDeviceID.Int64
	}
	if usedAt.Valid {
		v.UsedAt = usedAt.Time
	}
	if expiresAt.Valid {
		v.ExpiresAt = expiresAt.Time
	}
	if createdAt.Valid {
		v.CreatedAt = createdAt.Time
	}

	return &v, nil
}
