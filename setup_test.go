package appointmentbooking

import (
	"context"
	"testing"
	"time"

	"github.com/tinywasm/orm"
)

type MockStaffReader struct {
	Exists bool
	Err    error
}

func (m *MockStaffReader) StaffExists(tenantID, staffID string) (bool, error) {
	return m.Exists, m.Err
}

type MockCatalogReader struct {
	Exists bool
	Err    error
}

func (m *MockCatalogReader) ServiceExists(tenantID, serviceID string) (bool, error) {
	return m.Exists, m.Err
}

type MockDirectoryReader struct {
	Exists bool
	Err    error
}

func (m *MockDirectoryReader) ClientExists(tenantID, clientID string) (bool, error) {
	return m.Exists, m.Err
}

type MockEventPublisher struct {
	PublishedEvents []string
	Err             error
}

func (m *MockEventPublisher) Publish(ctx context.Context, event string, payload any) error {
	m.PublishedEvents = append(m.PublishedEvents, event)
	return m.Err
}

func SetupDependencies() Deps {
	return Deps{
		Staff:     &MockStaffReader{Exists: true},
		Catalog:   &MockCatalogReader{Exists: true},
		Directory: &MockDirectoryReader{Exists: true},
		Publisher: &MockEventPublisher{},
	}
}

// RunServicePureTests tests generic logic of the service (Availability, FSM changes, CreateReservation)
// without depending on standard lib SQLite, so it can run on WASM.
func RunServicePureTests(t *testing.T, s SchedulingService, repo *Repository, db *orm.DB) {
	ctx := context.Background()

	t.Run("CreateReservation_Success", func(t *testing.T) {
		// Insert active EmployeeServiceConfig
		cfg := EmployeeServiceConfig{
			TenantID:      "t1",
			StaffID:       "s1",
			ServiceID:     "srv1",
			DurationMin:   30,
			BufferMin:     0,
			IsActive:      true,
			PriceOverride: 100,
		}
		if err := repo.InsertEmployeeServiceConfig(cfg); err != nil {
			t.Fatalf("InsertEmployeeServiceConfig: %v", err)
		}
		// find auto-generated ID
		cfgs, _ := repo.ListEmployeeServiceConfigByStaff("t1", "s1")
		cfgID := cfgs[0].ID

		// Create Calendar Config
		if err := s.UpsertCalendarConfig(ctx, WorkCalendarConfig{
			TenantID: "t1",
			StaffID:  "s1",
			Timezone: "UTC",
			IsActive: true,
		}); err != nil {
			t.Fatalf("UpsertCalendarConfig: %v", err)
		}

		// Weekly calendar
		if err := s.UpsertWeeklyCalendar(ctx, WorkCalendarWeekly{
			TenantID:  "t1",
			StaffID:   "s1",
			DayOfWeek: 1, // Monday
			WorkStart: 540, // 09:00
			WorkFinish: 1020, // 17:00
			IsActive:  true,
		}); err != nil {
			t.Fatalf("UpsertWeeklyCalendar: %v", err)
		}

		targetDay := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC) // Jan 6, 2025 is Monday
		slotStartUTC := targetDay.Unix() + 540*60 // 09:00 UTC

		// Test ListAvailability
		slots, err := s.ListAvailability(ctx, "t1", "s1", cfgID, targetDay.Unix(), targetDay.Unix())
		if err != nil {
			t.Fatalf("ListAvailability: %v", err)
		}
		if len(slots) == 0 {
			t.Fatalf("expected some slots")
		}

		cmd := CreateReservationCmd{
			TenantID:                "t1",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            slotStartUTC,
			Notes:                   "Test note",
		}
		res, err := s.CreateReservation(ctx, cmd)
		if err != nil {
			t.Fatalf("CreateReservation: %v", err)
		}

		if res.Status != StatusPending {
			t.Fatalf("expected status pending, got %s", res.Status)
		}
		if res.Notes != "Test note" {
			t.Fatalf("expected notes to match")
		}

		// ChangeStatus
		err = s.ChangeReservationStatus(ctx, ChangeStatusCmd{
			TenantID: "t1",
			ID:       res.ID,
			Event:    EventConfirm,
			ActorID:  "u1",
			Revision: 0,
		})
		if err != nil {
			t.Fatalf("ChangeReservationStatus (Confirm): %v", err)
		}

		got, err := s.GetReservation(ctx, "t1", res.ID)
		if err != nil {
			t.Fatalf("GetReservation: %v", err)
		}
		if got.Status != StatusConfirmed {
			t.Fatalf("expected status confirmed, got %s", got.Status)
		}

		// ChangeStatus (NoShow)
		err = s.ChangeReservationStatus(ctx, ChangeStatusCmd{
			TenantID: "t1",
			ID:       res.ID,
			Event:    EventNoShow,
			ActorID:  "u1",
			Revision: 1,
		})
		if err != nil {
			t.Fatalf("ChangeReservationStatus (NoShow): %v", err)
		}
	})

	t.Run("CreateReservation_SlotTaken", func(t *testing.T) {
		// Insert active EmployeeServiceConfig
		cfg := EmployeeServiceConfig{
			TenantID:      "t2",
			StaffID:       "s2",
			ServiceID:     "srv2",
			DurationMin:   60,
			BufferMin:     0,
			IsActive:      true,
			PriceOverride: 100,
		}
		if err := repo.InsertEmployeeServiceConfig(cfg); err != nil {
			t.Fatalf("InsertEmployeeServiceConfig: %v", err)
		}
		cfgs, _ := repo.ListEmployeeServiceConfigByStaff("t2", "s2")
		cfgID := cfgs[0].ID

		// Create Calendar Config
		s.UpsertCalendarConfig(ctx, WorkCalendarConfig{
			TenantID: "t2",
			StaffID:  "s2",
			Timezone: "UTC",
			IsActive: true,
		})

		s.UpsertWeeklyCalendar(ctx, WorkCalendarWeekly{
			TenantID:  "t2",
			StaffID:   "s2",
			DayOfWeek: 2, // Tuesday
			WorkStart: 540,
			WorkFinish: 600, // 09:00 to 10:00 - exactly 1 hour
			IsActive:  true,
		})

		targetDay := time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC) // Jan 7, 2025 is Tuesday
		slotStartUTC := targetDay.Unix() + 540*60 // 09:00 UTC

		cmd := CreateReservationCmd{
			TenantID:                "t2",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            slotStartUTC,
		}

		_, err := s.CreateReservation(ctx, cmd)
		if err != nil {
			t.Fatalf("first CreateReservation should succeed, got: %v", err)
		}

		// Second reservation on same slot
		_, err = s.CreateReservation(ctx, cmd)
		if err != ErrSlotTaken {
			t.Fatalf("expected ErrSlotTaken, got: %v", err)
		}
	})

	t.Run("ExpirePendingReservations", func(t *testing.T) {
		cfg := EmployeeServiceConfig{
			TenantID:    "t3",
			StaffID:     "s3",
			ServiceID:   "srv3",
			DurationMin: 30,
			IsActive:    true,
		}
		repo.InsertEmployeeServiceConfig(cfg)
		cfgs, _ := repo.ListEmployeeServiceConfigByStaff("t3", "s3")
		cfgID := cfgs[0].ID

		s.UpsertCalendarConfig(ctx, WorkCalendarConfig{
			TenantID: "t3",
			StaffID:  "s3",
			Timezone: "UTC",
			IsActive: true,
		})

		s.UpsertWeeklyCalendar(ctx, WorkCalendarWeekly{
			TenantID:  "t3",
			StaffID:   "s3",
			DayOfWeek: 3, // Wednesday
			WorkStart: 540,
			WorkFinish: 600,
			IsActive:  true,
		})

		targetDay := time.Date(2025, 1, 8, 0, 0, 0, 0, time.UTC) // Jan 8, 2025 is Wednesday
		slotStartUTC := targetDay.Unix() + 540*60 // 09:00 UTC

		cmd := CreateReservationCmd{
			TenantID:                "t3",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            slotStartUTC,
		}

		res, err := s.CreateReservation(ctx, cmd)
		if err != nil {
			t.Fatalf("CreateReservation: %v", err)
		}

		// Expire everything before slotStartUTC + 1 hour
		count, err := s.ExpirePendingReservations(ctx, "t3", slotStartUTC + 3600)
		if err != nil {
			t.Fatalf("ExpirePendingReservations: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 expired reservation, got %d", count)
		}

		got, _ := s.GetReservation(ctx, "t3", res.ID)
		if got.Status != StatusExpired {
			t.Fatalf("expected expired status, got %s", got.Status)
		}
	})
}
