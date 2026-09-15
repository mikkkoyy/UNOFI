package payment

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/unofi/unofi/internal/database"
)

// SQLTransactionRepository implements TransactionRepository using SQLite.
type SQLTransactionRepository struct {
	db *database.DB
}

// NewSQLTransactionRepository creates a new SQL-backed transaction repository.
func NewSQLTransactionRepository(db *database.DB) *SQLTransactionRepository {
	return &SQLTransactionRepository{db: db}
}

// Create inserts a new transaction.
func (r *SQLTransactionRepository) Create(tx *Transaction) (*Transaction, error) {
	var sessionID interface{}
	if tx.SessionID != nil {
		sessionID = *tx.SessionID
	}

	result, err := r.db.Exec(
		`INSERT INTO transactions(device_id, session_id, payment_method, amount, mb_granted,
			time_granted_seconds, status, reference, created_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tx.DeviceID, sessionID, tx.PaymentMethod, tx.Amount, tx.MBGranted,
		tx.TimeGranted, tx.Status, tx.Reference, time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	tx.ID, _ = result.LastInsertId()
	tx.CreatedAt = time.Now().Format(time.RFC3339)
	return tx, nil
}

// GetByID retrieves a transaction by ID.
func (r *SQLTransactionRepository) GetByID(id int64) (*Transaction, error) {
	row := r.db.QueryRow(
		`SELECT id, device_id, session_id, payment_method, amount, mb_granted,
		        time_granted_seconds, status, reference, created_at
		 FROM transactions WHERE id = ?`, id,
	)
	return scanTransaction(row)
}

// UpdateStatus updates a transaction's status.
func (r *SQLTransactionRepository) UpdateStatus(id int64, status Status) error {
	_, err := r.db.Exec(
		`UPDATE transactions SET status = ? WHERE id = ?`,
		status, id,
	)
	return err
}

// ListByDevice returns transactions for a device.
func (r *SQLTransactionRepository) ListByDevice(deviceID int64, limit int) ([]*Transaction, error) {
	rows, err := r.db.Query(
		`SELECT id, device_id, session_id, payment_method, amount, mb_granted,
		        time_granted_seconds, status, reference, created_at
		 FROM transactions WHERE device_id = ? ORDER BY created_at DESC LIMIT ?`,
		deviceID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTransactions(rows)
}

// List returns a paginated list of transactions.
func (r *SQLTransactionRepository) List(limit, offset int) ([]*Transaction, error) {
	rows, err := r.db.Query(
		`SELECT id, device_id, session_id, payment_method, amount, mb_granted,
		        time_granted_seconds, status, reference, created_at
		 FROM transactions ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTransactions(rows)
}

// Count returns the total number of transactions.
func (r *SQLTransactionRepository) Count() (int64, error) {
	var count int64
	err := r.db.QueryRow(`SELECT COUNT(*) FROM transactions`).Scan(&count)
	return count, err
}

func scanTransaction(scanner interface{ Scan(dest ...interface{}) error }) (*Transaction, error) {
	var tx Transaction
	var sessionID sql.NullInt64
	err := scanner.Scan(
		&tx.ID, &tx.DeviceID, &sessionID, &tx.PaymentMethod, &tx.Amount,
		&tx.MBGranted, &tx.TimeGranted, &tx.Status, &tx.Reference, &tx.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan transaction: %w", err)
	}
	if sessionID.Valid {
		tx.SessionID = &sessionID.Int64
	}
	return &tx, nil
}

func scanTransactions(rows *sql.Rows) ([]*Transaction, error) {
	var transactions []*Transaction
	for rows.Next() {
		tx, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}
	return transactions, rows.Err()
}
