package voucher

import (
	"testing"

	"github.com/unofi/unofi/internal/database"
)

func TestGenerateCode(t *testing.T) {
	code, err := GenerateCode(8)
	if err != nil {
		t.Fatalf("GenerateCode error: %v", err)
	}
	if len(code) != 8 {
		t.Errorf("expected code length 8, got %d", len(code))
	}

	// Different codes should be generated
	code2, _ := GenerateCode(8)
	if code == code2 {
		t.Error("generated codes should be unique")
	}
}

func TestSQLRepositoryCreate(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	v := &Voucher{
		Code:      "ABC12345",
		PackageID: 1,
		Status:    VoucherUnused,
	}

	created, err := repo.Create(v)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if created.ID == 0 {
		t.Error("voucher ID should not be 0")
	}
	if created.Status != VoucherUnused {
		t.Errorf("expected status %s, got %s", VoucherUnused, created.Status)
	}
}

func TestSQLRepositoryGetByCode(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	_, _ = repo.Create(&Voucher{Code: "TESTCODE", PackageID: 1})

	got, err := repo.GetByCode("TESTCODE")
	if err != nil {
		t.Fatalf("GetByCode error: %v", err)
	}
	if got == nil {
		t.Fatal("voucher should be found")
	}
	if got.Code != "TESTCODE" {
		t.Errorf("expected code TESTCODE, got %s", got.Code)
	}
}

func TestSQLRepositoryMarkUsed(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	_, _ = repo.Create(&Voucher{Code: "TESTCODE", PackageID: 1})

	err := repo.MarkUsed("TESTCODE", 5)
	if err != nil {
		t.Fatalf("MarkUsed error: %v", err)
	}

	got, _ := repo.GetByCode("TESTCODE")
	if got.Status != VoucherUsed {
		t.Errorf("expected status %s, got %s", VoucherUsed, got.Status)
	}
	if got.UsedByDeviceID == nil || *got.UsedByDeviceID != 5 {
		t.Errorf("expected used by device 5, got %v", got.UsedByDeviceID)
	}
}

func TestSQLRepositoryMarkUsedAlreadyUsed(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	_, _ = repo.Create(&Voucher{Code: "TESTCODE", PackageID: 1})

	// Mark used first time
	_ = repo.MarkUsed("TESTCODE", 5)

	// Try marking used again (should fail silently or error)
	err := repo.MarkUsed("TESTCODE", 6)
	if err != nil {
		t.Fatalf("MarkUsed error on second call: %v", err)
	}

	// Should still be marked as used by first device
	got, _ := repo.GetByCode("TESTCODE")
	if *got.UsedByDeviceID != 5 {
		t.Errorf("expected still used by device 5, got %d", *got.UsedByDeviceID)
	}
}

func TestVoucherIsValid(t *testing.T) {
	tests := []struct {
		name    string
		voucher Voucher
		valid   bool
	}{
		{
			name:    "unused voucher",
			voucher: Voucher{Status: VoucherUnused},
			valid:   true,
		},
		{
			name:    "used voucher",
			voucher: Voucher{Status: VoucherUsed},
			valid:   false,
		},
		{
			name:    "revoked voucher",
			voucher: Voucher{Status: VoucherRevoked},
			valid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.voucher.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}
