package payment

import "context"

// Method represents a payment method type.
type Method string

const (
	MethodCoin    Method = "coin"
	MethodVoucher Method = "voucher"
	MethodGCash   Method = "gcash"
	MethodMaya    Method = "maya"
)

// Status represents the status of a payment.
type Status string

const (
	PaymentPending   Status = "pending"
	PaymentCompleted Status = "completed"
	PaymentFailed    Status = "failed"
	PaymentCancelled Status = "cancelled"
)

// Request represents a payment initiation request.
type Request struct {
	DeviceID int64   `json:"device_id"`
	Method   Method  `json:"method"`
	Amount   float64 `json:"amount"`
	PackageID int64  `json:"package_id,omitempty"`
}

// Result represents the outcome of a payment.
type Result struct {
	TransactionID int64   `json:"transaction_id"`
	Status        Status  `json:"status"`
	Amount        float64 `json:"amount"`
	MBGranted     float64 `json:"mb_granted"`
	TimeGranted   int     `json:"time_granted_seconds"`
	Reference     string  `json:"reference,omitempty"`
}

// MethodHandler defines the interface for payment method implementations.
type MethodHandler interface {
	// Name returns the payment method identifier.
	Name() Method
	// Initiate begins a payment session.
	Initiate(ctx context.Context, req Request) (*Result, error)
	// Cancel cancels an in-progress payment.
	Cancel(ctx context.Context, transactionID int64) error
	// Status returns the current status of a payment.
	Status(ctx context.Context, transactionID int64) (Status, error)
}

// Transaction represents a recorded payment transaction.
type Transaction struct {
	ID            int64   `json:"id"`
	DeviceID      int64   `json:"device_id"`
	SessionID     *int64  `json:"session_id,omitempty"`
	PaymentMethod Method  `json:"payment_method"`
	Amount        float64 `json:"amount"`
	MBGranted     float64 `json:"mb_granted"`
	TimeGranted   int     `json:"time_granted_seconds"`
	Status        Status  `json:"status"`
	Reference     string  `json:"reference,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

// TransactionRepository defines transaction persistence operations.
type TransactionRepository interface {
	Create(tx *Transaction) (*Transaction, error)
	GetByID(id int64) (*Transaction, error)
	UpdateStatus(id int64, status Status) error
	ListByDevice(deviceID int64, limit int) ([]*Transaction, error)
	List(limit, offset int) ([]*Transaction, error)
	Count() (int64, error)
}
