package tests

import (
	"context"
	"testing"
	"time"

	"github.com/tinywasm/orm"
	ab "github.com/veltylabs/appointment-booking"
)

func RunAvailabilityTests(t *testing.T, s ab.SchedulingService, repo *ab.Repository, db *orm.DB) {
	ctx := context.Background()

	setupValidConfig := func(tenant, staff string, slotStart int64) string {
		cfg := ab.EmployeeServiceConfig{
			TenantID:    tenant,
			StaffID:     staff,
			ServiceID:   "srv1",
			DurationMin: 60,
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
			WorkStart:  540,  // 09:00
			WorkFinish: 1020, // 17:00
			IsActive:   true,
		})
		return cfgID
	}

	t.Run("UC-08_ListAvailability_HolidayException", func(t *testing.T) {
		slot := time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC).Unix()
		cfgID := setupValidConfig("t_uc08", "s_uc08", slot)

		dayStart := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC).Unix()
		s.AddException(ctx, ab.WorkCalendarException{
			TenantID:      "t_uc08",
			StaffID:       "s_uc08",
			ExceptionType: "HOLIDAY",
			SpecificDate:  dayStart,
		})

		slots, err := s.ListAvailability(ctx, "t_uc08", "s_uc08", cfgID, dayStart, dayStart+86400)
		if err != nil {
			t.Fatalf("ListAvailability: %v", err)
		}
		if len(slots) != 0 {
			t.Fatalf("expected 0 slots on holiday, got %d", len(slots))
		}
	})

	t.Run("UC-09_ListAvailability_BlockedException", func(t *testing.T) {
		slot := time.Date(2025, 3, 2, 10, 0, 0, 0, time.UTC).Unix()
		cfgID := setupValidConfig("t_uc09", "s_uc09", slot)

		dayStart := time.Date(2025, 3, 2, 0, 0, 0, 0, time.UTC).Unix()
		s.AddException(ctx, ab.WorkCalendarException{
			TenantID:      "t_uc09",
			StaffID:       "s_uc09",
			ExceptionType: "BLOCKED",
			SpecificDate:  dayStart,
			StartTime:     600, // 10:00
			EndTime:       720, // 12:00
		})

		slots, err := s.ListAvailability(ctx, "t_uc09", "s_uc09", cfgID, dayStart, dayStart+86400)
		if err != nil {
			t.Fatalf("ListAvailability: %v", err)
		}

		for _, s := range slots {
			startLocal := (s.StartUTC - dayStart) / 60
			endLocal := (s.EndUTC - dayStart) / 60
			// blocked from 600 to 720
			if startLocal < 720 && endLocal > 600 {
				t.Fatalf("slot %v overlaps blocked time (600-720)", s)
			}
		}
	})

	t.Run("UC-10_ListAvailability_SpecialHoursException", func(t *testing.T) {
		slot := time.Date(2025, 3, 3, 10, 0, 0, 0, time.UTC).Unix()
		cfgID := setupValidConfig("t_uc10", "s_uc10", slot)

		dayStart := time.Date(2025, 3, 3, 0, 0, 0, 0, time.UTC).Unix()
		s.AddException(ctx, ab.WorkCalendarException{
			TenantID:      "t_uc10",
			StaffID:       "s_uc10",
			ExceptionType: "SPECIAL_HOURS",
			SpecificDate:  dayStart,
			StartTime:     480, // 08:00
			EndTime:       600, // 10:00
		})

		slots, err := s.ListAvailability(ctx, "t_uc10", "s_uc10", cfgID, dayStart, dayStart+86400)
		if err != nil {
			t.Fatalf("ListAvailability: %v", err)
		}

		if len(slots) != 2 { // 8:00-9:00, 9:00-10:00
			t.Fatalf("expected 2 slots in special hours, got %d", len(slots))
		}
		for _, s := range slots {
			startLocal := (s.StartUTC - dayStart) / 60
			endLocal := (s.EndUTC - dayStart) / 60
			if startLocal < 480 || endLocal > 600 {
				t.Fatalf("slot %v outside special hours (480-600)", s)
			}
		}
	})

	t.Run("UC-11_ListAvailability_BreakTime", func(t *testing.T) {
		slot := time.Date(2025, 3, 4, 10, 0, 0, 0, time.UTC).Unix()
		cfgID := setupValidConfig("t_uc11", "s_uc11", slot)

		dayStart := time.Date(2025, 3, 4, 0, 0, 0, 0, time.UTC).Unix()

		// Update weekly config to add break
		s.UpsertWeeklyCalendar(ctx, ab.WorkCalendarWeekly{
			TenantID:    "t_uc11",
			StaffID:     "s_uc11",
			DayOfWeek:   int64(time.Unix(slot, 0).UTC().Weekday()),
			WorkStart:   540,  // 09:00
			WorkFinish:  1020, // 17:00
			BreakStart:  720,  // 12:00
			BreakFinish: 780,  // 13:00
			IsActive:    true,
		})

		slots, err := s.ListAvailability(ctx, "t_uc11", "s_uc11", cfgID, dayStart, dayStart+86400)
		if err != nil {
			t.Fatalf("ListAvailability: %v", err)
		}

		for _, s := range slots {
			startLocal := (s.StartUTC - dayStart) / 60
			endLocal := (s.EndUTC - dayStart) / 60
			if startLocal < 780 && endLocal > 720 {
				t.Fatalf("slot %v overlaps break time (720-780)", s)
			}
		}
	})

	t.Run("UC-15_ListAvailability_NoCalendarConfig", func(t *testing.T) {
		_, err := s.ListAvailability(ctx, "t_uc15", "non_existent_staff", "cfg_id", 0, 1000)
		if err != ab.ErrCalendarConfigNotFound {
			t.Fatalf("expected ab.ErrCalendarConfigNotFound, got: %v", err)
		}
	})

	t.Run("UC-16_ListAvailability_InactiveCalendarConfig", func(t *testing.T) {
		slot := time.Date(2025, 3, 6, 10, 0, 0, 0, time.UTC).Unix()
		cfgID := setupValidConfig("t_uc16", "s_uc16", slot)

		s.UpsertCalendarConfig(ctx, ab.WorkCalendarConfig{
			TenantID: "t_uc16",
			StaffID:  "s_uc16",
			Timezone: "UTC",
			IsActive: false, // inactive
		})

		dayStart := time.Date(2025, 3, 6, 0, 0, 0, 0, time.UTC).Unix()
		slots, err := s.ListAvailability(ctx, "t_uc16", "s_uc16", cfgID, dayStart, dayStart+86400)
		if err != nil {
			t.Fatalf("ListAvailability: %v", err)
		}
		if len(slots) != 0 {
			t.Fatalf("expected 0 slots for inactive calendar config, got %d", len(slots))
		}
	})

	t.Run("UC-17_CreateReservation_SlotOutsideWorkHours", func(t *testing.T) {
		slot := time.Date(2025, 3, 7, 10, 0, 0, 0, time.UTC).Unix() // 10:00 UTC (Work hours 09:00 - 17:00)
		cfgID := setupValidConfig("t_uc17", "s_uc17", slot)

		dayStart := time.Date(2025, 3, 7, 0, 0, 0, 0, time.UTC).Unix()
		midnightSlot := dayStart // 00:00 UTC, which is outside 09:00 - 17:00

		_, err := s.CreateReservation(ctx, ab.CreateReservationCmd{
			TenantID:                "t_uc17",
			ClientID:                "c1",
			CreatorUserID:           "u1",
			EmployeeServiceConfigID: cfgID,
			SlotStartUTC:            midnightSlot,
		})
		if err != ab.ErrSlotTaken {
			t.Fatalf("expected ab.ErrSlotTaken for slot outside work hours, got: %v", err)
		}
	})
}