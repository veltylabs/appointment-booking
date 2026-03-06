//go:build !wasm

package appointmentbooking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tinywasm/sqlite"
)

func TestService_Back(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}
	defer db.Close()

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

	// Run pure tests first on sqlite
	t.Run("PureTests", func(t *testing.T) {
		RunServicePureTests(t, svc, repo, db)
	})

	// Run integration/concurrency specific tests
	t.Run("Integration_Concurrency", func(t *testing.T) {
		ctx := context.Background()

		// Setup config
		cfg := EmployeeServiceConfig{
			TenantID:      "t99",
			StaffID:       "s99",
			ServiceID:     "srv99",
			DurationMin:   30,
			IsActive:      true,
			PriceOverride: 100,
		}
		repo.InsertEmployeeServiceConfig(cfg)
		cfgs, _ := repo.ListEmployeeServiceConfigByStaff("t99", "s99")
		cfgID := cfgs[0].ID

		s := svc
		s.UpsertCalendarConfig(ctx, WorkCalendarConfig{
			TenantID: "t99",
			StaffID:  "s99",
			Timezone: "UTC",
			IsActive: true,
		})
		s.UpsertWeeklyCalendar(ctx, WorkCalendarWeekly{
			TenantID:  "t99",
			StaffID:   "s99",
			DayOfWeek: 4, // Thursday
			WorkStart: 540,
			WorkFinish: 600,
			IsActive:  true,
		})

		targetDay := time.Date(2025, 1, 9, 0, 0, 0, 0, time.UTC) // Jan 9, 2025 is Thursday
		slotStartUTC := targetDay.Unix() + 540*60

		res, err := s.CreateReservation(ctx, CreateReservationCmd{
			TenantID:                "t99",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            slotStartUTC,
		})
		if err != nil {
			t.Fatalf("CreateReservation: %v", err)
		}

		// Test Conflict / Revision System
		err1 := s.ChangeReservationStatus(ctx, ChangeStatusCmd{
			TenantID:  "t99",
			ID:        res.ID,
			Event:     EventConfirm,
			ActorID:   "u1",
			PaymentID: "pay1",
			Revision:  0, // Correct revision
		})
		if err1 != nil {
			t.Fatalf("First change should succeed, got: %v", err1)
		}

		err2 := s.ChangeReservationStatus(ctx, ChangeStatusCmd{
			TenantID: "t99",
			ID:       res.ID,
			Event:    EventCancel,
			ActorID:  "u1",
			Revision: 0, // Wrong revision, should be 1
		})
		if !errors.Is(err2, ErrConflict) {
			t.Fatalf("Second change should fail with ErrConflict, got: %v", err2)
		}

		// Verify event publisher received events
		pub := deps.Publisher.(*MockEventPublisher)
		foundCreated := false
		foundConfirmed := false
		for _, e := range pub.PublishedEvents {
			if e == EventReservationCreated {
				foundCreated = true
			}
			if e == EventReservationConfirmed {
				foundConfirmed = true
			}
		}
		if !foundCreated {
			t.Fatalf("expected EventReservationCreated to be published")
		}
		if !foundConfirmed {
			t.Fatalf("expected EventReservationConfirmed to be published")
		}
	})
}
