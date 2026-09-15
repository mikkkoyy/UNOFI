package voucher

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Voucher represents a redeemable voucher code.
type Voucher struct {
	ID              int64     `json:"id"`
	Code            string    `json:"code"`
	PackageID       int64     `json:"package_id"`
	Status          string    `json:"status"`
	UsedByDeviceID  *int64    `json:"used_by_device_id,omitempty"`
	UsedAt          time.Time `json:"used_at,omitempty"`
	ExpiresAt       time.Time `json:"expires_at,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// VoucherStatus represents possible voucher states.
const (
	VoucherUnused  = "unused"
	VoucherUsed    = "used"
	VoucherExpired = "expired"
	VoucherRevoked = "revoked"
)

// Repository defines voucher persistence operations.
type Repository interface {
	Create(v *Voucher) (*Voucher, error)
	GetByCode(code string) (*Voucher, error)
	MarkUsed(code string, deviceID int64) error
	List(limit, offset int) ([]*Voucher, error)
	Count() (int64, error)
}

// GenerateCode creates a random alphanumeric voucher code.
func GenerateCode(length int) (string, error) {
	bytes := make([]byte, length/2+1)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random: %w", err)
	}
	code := hex.EncodeToString(bytes)
	return code[:length], nil
}

// IsValid checks if the voucher can be redeemed.
func (v *Voucher) IsValid() bool {
	if v.Status != VoucherUnused {
		return false
	}
	if !v.ExpiresAt.IsZero() && time.Now().After(v.ExpiresAt) {
		return false
	}
	return true
}
