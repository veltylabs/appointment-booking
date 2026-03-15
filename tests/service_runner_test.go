package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tinywasm/orm"
	ab "github.com/veltylabs/appointment-booking"
)

func RunServiceValidationTests(t *testing.T, s ab.SchedulingService, repo *ab.Repository, db *orm.DB, deps ab.Deps) {
	ctx := context.Background()

	t.Run("UC-01_CreateReservation_InactiveConfig", func(t *testing.T) {
		cfg := ab.EmployeeServiceConfig{
			TenantID:    "t_uc01",
			StaffID:     "s_uc01",
			ServiceID:   "srv_uc01",
			DurationMin: 30,
			IsActive:    false, // inactive
		}
		repo.InsertEmployeeServiceConfig(cfg)
		cfgs, _ := repo.ListEmployeeServiceConfigByStaff("t_uc01", "s_uc01")
		cfgID := cfgs[0].ID

		_, err := s.CreateReservation(ctx, ab.CreateReservationCmd{
			TenantID:                "t_uc01",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            1000,
		})
		if err != ab.ErrNotFound {
			t.Fatalf("expected ab.ErrNotFound, got: %v", err)
		}
	})

	t.Run("UC-02_CreateReservation_StaffNotFound", func(t *testing.T) {
		cfg := ab.EmployeeServiceConfig{
			TenantID:    "t_uc02",
			StaffID:     "s_uc02",
			ServiceID:   "srv_uc02",
			DurationMin: 30,
			IsActive:    true,
		}
		repo.InsertEmployeeServiceConfig(cfg)
		cfgs, _ := repo.ListEmployeeServiceConfigByStaff("t_uc02", "s_uc02")
		cfgID := cfgs[0].ID

		mockStaff := deps.Staff.(*MockStaffReader)
		mockStaff.Exists = false
		defer func() { mockStaff.Exists = true }()

		_, err := s.CreateReservation(ctx, ab.CreateReservationCmd{
			TenantID:                "t_uc02",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            1000,
		})
		if err == nil || !strings.Contains(err.Error(), "staff not found") {
			t.Fatalf("expected 'staff not found' error, got: %v", err)
		}
	})

	t.Run("UC-03_CreateReservation_ServiceNotFound", func(t *testing.T) {
		cfg := ab.EmployeeServiceConfig{
			TenantID:    "t_uc03",
			StaffID:     "s_uc03",
			ServiceID:   "srv_uc03",
			DurationMin: 30,
			IsActive:    true,
		}
		repo.InsertEmployeeServiceConfig(cfg)
		cfgs, _ := repo.ListEmployeeServiceConfigByStaff("t_uc03", "s_uc03")
		cfgID := cfgs[0].ID

		mockCatalog := deps.Catalog.(*MockCatalogReader)
		mockCatalog.Exists = false
		defer func() { mockCatalog.Exists = true }()

		_, err := s.CreateReservation(ctx, ab.CreateReservationCmd{
			TenantID:                "t_uc03",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            1000,
		})
		if err == nil || !strings.Contains(err.Error(), "service not found") {
			t.Fatalf("expected 'service not found' error, got: %v", err)
		}
	})

	t.Run("UC-04_CreateReservation_ClientNotFound", func(t *testing.T) {
		cfg := ab.EmployeeServiceConfig{
			TenantID:    "t_uc04",
			StaffID:     "s_uc04",
			ServiceID:   "srv_uc04",
			DurationMin: 30,
			IsActive:    true,
		}
		repo.InsertEmployeeServiceConfig(cfg)
		cfgs, _ := repo.ListEmployeeServiceConfigByStaff("t_uc04", "s_uc04")
		cfgID := cfgs[0].ID

		mockDirectory := deps.Directory.(*MockDirectoryReader)
		mockDirectory.Exists = false
		defer func() { mockDirectory.Exists = true }()

		_, err := s.CreateReservation(ctx, ab.CreateReservationCmd{
			TenantID:                "t_uc04",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            1000,
		})
		if err == nil || !strings.Contains(err.Error(), "client not found") {
			t.Fatalf("expected 'client not found' error, got: %v", err)
		}
	})

	setupValidConfig := func(tenant, staff string, slotStart int64) string {
		cfg := ab.EmployeeServiceConfig{
			TenantID:    tenant,
			StaffID:     staff,
			ServiceID:   "srv1",
			DurationMin: 30,
			IsActive:    true,
		}
		repo.InsertEmployeeServiceConfig(cfg)
		cfgs, _ := repo.ListEmployeeServiceConfigByStaff(tenant, staff)
		cfgID := cfgs[0].ID

		s.UpsertCalendarConfig(ctx, ab.WorkCalendarConfig{
			TenantID: tenant,
			StaffID:  staff,
			Timezone: "UTC",
			IsActive: true,
		})

		utcDate := time.Unix(slotStart, 0).UTC()
		dow := int(utcDate.Weekday())

		s.UpsertWeeklyCalendar(ctx, ab.WorkCalendarWeekly{
			TenantID:   tenant,
			StaffID:    staff,
			DayOfWeek:  int64(dow),
			WorkStart:  0,
			WorkFinish: 1440, // all day
			IsActive:   true,
		})
		return cfgID
	}

	t.Run("UC-05_ChangeReservationStatus_Cancel_FromPending", func(t *testing.T) {
		slot := time.Date(2025, 2, 1, 10, 0, 0, 0, time.UTC).Unix()
		cfgID := setupValidConfig("t_uc05", "s_uc05", slot)

		res, err := s.CreateReservation(ctx, ab.CreateReservationCmd{
			TenantID:                "t_uc05",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            slot,
		})
		if err != nil {
			t.Fatalf("CreateReservation: %v", err)
		}

		mockPub := deps.Publisher.(*MockEventPublisher)
		mockPub.PublishedEvents = nil // reset

		err = s.ChangeReservationStatus(ctx, ab.ChangeStatusCmd{
			TenantID: "t_uc05",
			ID:       res.ID,
			Event:    ab.EventCancel,
			ActorID:  "u1",
			Revision: 0,
		})
		if err != nil {
			t.Fatalf("ChangeReservationStatus: %v", err)
		}

		got, _ := s.GetReservation(ctx, "t_uc05", res.ID)
		if got.Status != ab.StatusCancelled {
			t.Fatalf("expected CANCELLED, got %s", got.Status)
		}

		foundCancel := false
		for _, e := range mockPub.PublishedEvents {
			if e == ab.EventReservationCancelled {
				foundCancel = true
				break
			}
		}
		if !foundCancel {
			t.Fatalf("expected EventReservationCancelled")
		}
	})

	t.Run("UC-06_ChangeReservationStatus_Complete_FromConfirmed", func(t *testing.T) {
		slot := time.Date(2025, 2, 2, 10, 0, 0, 0, time.UTC).Unix()
		cfgID := setupValidConfig("t_uc06", "s_uc06", slot)

		res, _ := s.CreateReservation(ctx, ab.CreateReservationCmd{
			TenantID:                "t_uc06",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            slot,
		})

		s.ChangeReservationStatus(ctx, ab.ChangeStatusCmd{
			TenantID: "t_uc06",
			ID:       res.ID,
			Event:    ab.EventConfirm,
			ActorID:  "u1",
			Revision: 0,
		})

		mockPub := deps.Publisher.(*MockEventPublisher)
		mockPub.PublishedEvents = nil

		err := s.ChangeReservationStatus(ctx, ab.ChangeStatusCmd{
			TenantID: "t_uc06",
			ID:       res.ID,
			Event:    ab.EventComplete,
			ActorID:  "u1",
			Revision: 1,
		})
		if err != nil {
			t.Fatalf("ChangeReservationStatus Complete: %v", err)
		}

		got, _ := s.GetReservation(ctx, "t_uc06", res.ID)
		if got.Status != ab.StatusCompleted {
			t.Fatalf("expected COMPLETED, got %s", got.Status)
		}

		foundComplete := false
		for _, e := range mockPub.PublishedEvents {
			if e == ab.EventReservationCompleted {
				foundComplete = true
				break
			}
		}
		if !foundComplete {
			t.Fatalf("expected EventReservationCompleted")
		}
	})

	t.Run("UC-07_CreateReservation_Reschedule", func(t *testing.T) {
		slot1 := time.Date(2025, 2, 3, 10, 0, 0, 0, time.UTC).Unix()
		slot2 := time.Date(2025, 2, 3, 11, 0, 0, 0, time.UTC).Unix()
		cfgID := setupValidConfig("t_uc07", "s_uc07", slot1)

		res1, _ := s.CreateReservation(ctx, ab.CreateReservationCmd{
			TenantID:                "t_uc07",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            slot1,
		})

		mockPub := deps.Publisher.(*MockEventPublisher)
		mockPub.PublishedEvents = nil

		res2, err := s.CreateReservation(ctx, ab.CreateReservationCmd{
			TenantID:                "t_uc07",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            slot2,
			RescheduledFromID:       res1.ID,
		})
		if err != nil {
			t.Fatalf("CreateReservation reschedule: %v", err)
		}

		if res2.Status != ab.StatusPending {
			t.Fatalf("expected new res PENDING, got %s", res2.Status)
		}

		gotOrig, _ := s.GetReservation(ctx, "t_uc07", res1.ID)
		if gotOrig.Status != ab.StatusRescheduled {
			t.Fatalf("expected orig res RESCHEDULED, got %s", gotOrig.Status)
		}

		foundCreated, foundResched := false, false
		for _, e := range mockPub.PublishedEvents {
			if e == ab.EventReservationCreated {
				foundCreated = true
			}
			if e == ab.EventReservationRescheduled {
				foundResched = true
			}
		}
		if !foundCreated || !foundResched {
			t.Fatalf("expected both Created and Rescheduled events, got %+v", mockPub.PublishedEvents)
		}
	})

	t.Run("UC-13_GetReservation_CrossTenantIsolation", func(t *testing.T) {
		slot := time.Date(2025, 2, 4, 10, 0, 0, 0, time.UTC).Unix()
		cfgID := setupValidConfig("T1", "s_uc13", slot)

		res, _ := s.CreateReservation(ctx, ab.CreateReservationCmd{
			TenantID:                "T1",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            slot,
		})

		_, err := s.GetReservation(ctx, "T2", res.ID)
		if err != ab.ErrNotFound {
			t.Fatalf("expected ErrNotFound for cross tenant Get, got %v", err)
		}
	})

	t.Run("UC-14_ChangeReservationStatus_CrossTenantIsolation", func(t *testing.T) {
		slot := time.Date(2025, 2, 5, 10, 0, 0, 0, time.UTC).Unix()
		cfgID := setupValidConfig("T1", "s_uc14", slot)

		res, _ := s.CreateReservation(ctx, ab.CreateReservationCmd{
			TenantID:                "T1",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            slot,
		})

		err := s.ChangeReservationStatus(ctx, ab.ChangeStatusCmd{
			TenantID: "T2",
			ID:       res.ID,
			Event:    ab.EventCancel,
			ActorID:  "u1",
			Revision: 0,
		})
		if err != ab.ErrNotFound {
			t.Fatalf("expected ErrNotFound for cross tenant ChangeStatus, got %v", err)
		}
	})

	t.Run("UC-12_UpsertWeeklyCalendar_CalendarConfigNotFound", func(t *testing.T) {
		err := s.UpsertWeeklyCalendar(ctx, ab.WorkCalendarWeekly{
			TenantID:  "t_uc12",
			StaffID:   "non_existent",
			DayOfWeek: 1,
			WorkStart: 540,
		})
		if err != ab.ErrCalendarConfigNotFound {
			t.Fatalf("expected ErrCalendarConfigNotFound, got %v", err)
		}
	})

	t.Run("UC-18_ChangeReservationStatus_ConfirmWithPaymentID", func(t *testing.T) {
		slot := time.Date(2025, 2, 6, 10, 0, 0, 0, time.UTC).Unix()
		cfgID := setupValidConfig("t_uc18", "s_uc18", slot)

		res, _ := s.CreateReservation(ctx, ab.CreateReservationCmd{
			TenantID:                "t_uc18",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            slot,
		})

		err := s.ChangeReservationStatus(ctx, ab.ChangeStatusCmd{
			TenantID:  "t_uc18",
			ID:        res.ID,
			Event:     ab.EventConfirm,
			ActorID:   "u1",
			PaymentID: "pay_123",
			Revision:  0,
		})
		if err != nil {
			t.Fatalf("ChangeReservationStatus: %v", err)
		}

		got, _ := s.GetReservation(ctx, "t_uc18", res.ID)
		if got.PaymentID != "pay_123" {
			t.Fatalf("expected PaymentID 'pay_123', got '%s'", got.PaymentID)
		}
	})

	t.Run("UC-19_ListReservationsByStaff_ViaService", func(t *testing.T) {
		slot1 := time.Date(2025, 2, 7, 10, 0, 0, 0, time.UTC).Unix()
		slot2 := time.Date(2025, 2, 7, 11, 0, 0, 0, time.UTC).Unix()
		cfgID := setupValidConfig("t_uc19", "s_uc19", slot1)

		s.CreateReservation(ctx, ab.CreateReservationCmd{
			TenantID:                "t_uc19",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            slot1,
		})
		time.Sleep(time.Millisecond) // avoid nanosecond collision if any
		s.CreateReservation(ctx, ab.CreateReservationCmd{
			TenantID:                "t_uc19",
			ClientID:                "c2",
			CreatorUserID:           "u2",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            slot2,
		})

		from := time.Date(2025, 2, 7, 0, 0, 0, 0, time.UTC).Unix()
		to := time.Date(2025, 2, 8, 0, 0, 0, 0, time.UTC).Unix()

		res, err := s.ListReservationsByStaff(ctx, "t_uc19", "s_uc19", from, to)
		if err != nil {
			t.Fatalf("ListReservationsByStaff: %v", err)
		}
		if len(res) != 2 {
			t.Fatalf("expected 2 reservations, got %d", len(res))
		}
		if res[0].StaffIDSnapshot != "s_uc19" || res[1].StaffIDSnapshot != "s_uc19" {
			t.Fatalf("expected staffID s_uc19")
		}
	})

	t.Run("UC-20_ExpirePendingReservations_NothingToExpire", func(t *testing.T) {
		slot := time.Date(2025, 2, 8, 10, 0, 0, 0, time.UTC).Unix()
		cfgID := setupValidConfig("t_uc20", "s_uc20", slot)

		s.CreateReservation(ctx, ab.CreateReservationCmd{
			TenantID:                "t_uc20",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            slot,
		})

		// try to expire before the reservation
		count, err := s.ExpirePendingReservations(ctx, "t_uc20", slot-3600)
		if err != nil {
			t.Fatalf("ExpirePendingReservations: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected 0 expired, got %d", count)
		}
	})
}
