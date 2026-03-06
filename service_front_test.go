//go:build wasm

package appointmentbooking

import (
	"testing"

	"github.com/tinywasm/orm"
)

func TestService_Pure(t *testing.T) {
	// For WASM tests we need an ORM map backend
	// tinywasm/orm by default uses map driver when not initialized with explicit backend for WASM in memory tests usually
	db := orm.NewMemoryDB()

	repo, err := NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}

	deps := SetupDependencies()
	svc := &schedulingService{
		db:        db,
		repo:      repo,
		staff:     deps.Staff,
		catalog:   deps.Catalog,
		directory: deps.Directory,
		pub:       deps.Publisher,
	}

	RunServicePureTests(t, svc, repo, db)
}
