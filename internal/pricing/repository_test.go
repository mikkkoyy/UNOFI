package pricing

import (
	"testing"

	"github.com/unofi/unofi/internal/database"
)

func TestSQLRepositoryCreate(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	pkg := &Package{
		Name:        "Test Package",
		Description: "1 hour of internet",
		Price:       10,
		MBAmount:    500,
		TimeSeconds: 3600,
		IsActive:    true,
	}

	created, err := repo.Create(pkg)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if created.ID == 0 {
		t.Error("package ID should not be 0")
	}
	if created.Name != "Test Package" {
		t.Errorf("expected name Test Package, got %s", created.Name)
	}
}

func TestSQLRepositoryGetByID(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	pkg := &Package{Name: "Test", Price: 10, IsActive: true}
	created, _ := repo.Create(pkg)

	got, err := repo.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID error: %v", err)
	}
	if got == nil {
		t.Fatal("package should be found")
	}
	if got.Price != 10 {
		t.Errorf("expected price 10, got %f", got.Price)
	}
}

func TestSQLRepositoryList(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	_, _ = repo.Create(&Package{Name: "Active1", Price: 5, IsActive: true})
	_, _ = repo.Create(&Package{Name: "Active2", Price: 10, IsActive: true})
	_, _ = repo.Create(&Package{Name: "Inactive", Price: 20, IsActive: false})

	// List all
	all, err := repo.List(false)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 packages, got %d", len(all))
	}

	// List active only
	active, err := repo.List(true)
	if err != nil {
		t.Fatalf("List active error: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("expected 2 active packages, got %d", len(active))
	}
}

func TestPricingEngineCalculateGrant(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)
	engine := NewPricingEngine(repo)

	// Create a package
	pkg := &Package{
		Name:        "₱10 Package",
		Price:       10,
		MBAmount:    500,
		TimeSeconds: 3600,
		IsActive:    true,
	}
	created, _ := repo.Create(pkg)

	mb, seconds, err := engine.CalculateGrant(created.ID)
	if err != nil {
		t.Fatalf("CalculateGrant error: %v", err)
	}
	if mb != 500 {
		t.Errorf("expected 500 MB, got %f", mb)
	}
	if seconds != 3600 {
		t.Errorf("expected 3600 seconds, got %d", seconds)
	}
}

func TestPricingEngineCalculateGrantInactive(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)
	engine := NewPricingEngine(repo)

	pkg := &Package{Name: "Inactive", Price: 10, IsActive: false}
	created, _ := repo.Create(pkg)

	_, _, err := engine.CalculateGrant(created.ID)
	if err == nil {
		t.Error("expected error for inactive package")
	}
}

func TestPricingEngineCalculateGrantNotFound(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)
	engine := NewPricingEngine(repo)

	_, _, err := engine.CalculateGrant(999)
	if err == nil {
		t.Error("expected error for non-existent package")
	}
}
