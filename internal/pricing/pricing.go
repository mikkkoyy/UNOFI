package pricing

import (
	"fmt"
	"time"
)

// Package represents a purchasable data/time package.
type Package struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	MBAmount    float64   `json:"mb_amount"`
	TimeSeconds int       `json:"time_seconds"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Repository defines package persistence operations.
type Repository interface {
	Create(pkg *Package) (*Package, error)
	GetByID(id int64) (*Package, error)
	Update(pkg *Package) error
	Delete(id int64) error
	List(activeOnly bool) ([]*Package, error)
}

// PricingEngine calculates the data/time grant for a given payment.
type PricingEngine struct {
	repo Repository
}

// NewPricingEngine creates a new pricing engine.
func NewPricingEngine(repo Repository) *PricingEngine {
	return &PricingEngine{repo: repo}
}

// CalculateGrant returns the data and time allocation for a package.
func (e *PricingEngine) CalculateGrant(packageID int64) (mb float64, seconds int, err error) {
	pkg, err := e.repo.GetByID(packageID)
	if err != nil {
		return 0, 0, fmt.Errorf("get package: %w", err)
	}
	if pkg == nil {
		return 0, 0, fmt.Errorf("package not found: %d", packageID)
	}
	if !pkg.IsActive {
		return 0, 0, fmt.Errorf("package not active: %d", packageID)
	}
	return pkg.MBAmount, pkg.TimeSeconds, nil
}

// GetPackage retrieves a package by ID.
func (e *PricingEngine) GetPackage(id int64) (*Package, error) {
	return e.repo.GetByID(id)
}

// ListPackages returns available packages.
func (e *PricingEngine) ListPackages(activeOnly bool) ([]*Package, error) {
	return e.repo.List(activeOnly)
}
