package appointmentbooking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tinywasm/mcp"
)

func Register(s *mcp.MCPServer, svc SchedulingService) {
	registerCalendarTools(s, svc)
	registerReservationTools(s, svc)
}

func registerReservationTools(s *mcp.MCPServer, svc SchedulingService) {
	// create_reservation
	toolCreate := mcp.NewProtocolTool("create_reservation",
		mcp.WithDescription("Creates a new reservation."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("client_id", mcp.Required()),
		mcp.WithString("creator_user_id", mcp.Required()),
		mcp.WithString("employee_service_config_id", mcp.Required()),
		mcp.WithNumber("slot_start_utc", mcp.Required()),
		mcp.WithString("notes"),
		mcp.WithString("rescheduled_from_id"),
	)
	s.AddTool(toolCreate, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cmd := CreateReservationCmd{
			TenantID:                req.GetString("tenant_id", ""),
			ClientID:                req.GetString("client_id", ""),
			CreatorUserID:           req.GetString("creator_user_id", ""),
			EmployeeServiceConfigID: req.GetString("employee_service_config_id", ""),
			SlotStartUTC:            int64(req.GetInt("slot_start_utc", 0)),
			Notes:                   req.GetString("notes", ""),
			RescheduledFromID:       req.GetString("rescheduled_from_id", ""),
		}

		res, err := svc.CreateReservation(ctx, cmd)
		if err != nil {
			if errors.Is(err, ErrSlotTaken) {
				return mcp.NewToolResultError("The selected time slot is already taken"), nil
			}
			return mcp.NewToolResultError(err.Error()), nil
		}

		b, _ := json.Marshal(res)
		return mcp.NewToolResultText(string(b)), nil
	})

	// get_reservation
	toolGet := mcp.NewProtocolTool("get_reservation",
		mcp.WithDescription("Gets a reservation by ID."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("id", mcp.Required()),
	)
	s.AddTool(toolGet, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tenantID := req.GetString("tenant_id", "")
		id := req.GetString("id", "")

		res, err := svc.GetReservation(ctx, tenantID, id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		b, _ := json.Marshal(res)
		return mcp.NewToolResultText(string(b)), nil
	})

	// list_reservations_by_staff
	toolListStaff := mcp.NewProtocolTool("list_reservations_by_staff",
		mcp.WithDescription("Lists reservations by staff ID and date range."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("staff_id", mcp.Required()),
		mcp.WithNumber("from", mcp.Required()),
		mcp.WithNumber("to", mcp.Required()),
	)
	s.AddTool(toolListStaff, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		res, err := svc.ListReservationsByStaff(
			ctx,
			req.GetString("tenant_id", ""),
			req.GetString("staff_id", ""),
			int64(req.GetInt("from", 0)),
			int64(req.GetInt("to", 0)),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		b, _ := json.Marshal(res)
		return mcp.NewToolResultText(string(b)), nil
	})

	// list_reservations_by_client
	toolListClient := mcp.NewProtocolTool("list_reservations_by_client",
		mcp.WithDescription("Lists reservations by client ID."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("client_id", mcp.Required()),
	)
	s.AddTool(toolListClient, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		res, err := svc.ListReservationsByClient(
			ctx,
			req.GetString("tenant_id", ""),
			req.GetString("client_id", ""),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		b, _ := json.Marshal(res)
		return mcp.NewToolResultText(string(b)), nil
	})

	// change_reservation_status
	toolStatus := mcp.NewProtocolTool("change_reservation_status",
		mcp.WithDescription("Changes a reservation status via FSM event."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("id", mcp.Required()),
		mcp.WithString("event", mcp.Required()),
		mcp.WithString("actor_id", mcp.Required()),
		mcp.WithString("payment_id"),
		mcp.WithNumber("revision", mcp.Required()),
	)
	s.AddTool(toolStatus, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cmd := ChangeStatusCmd{
			TenantID:  req.GetString("tenant_id", ""),
			ID:        req.GetString("id", ""),
			Event:     req.GetString("event", ""),
			ActorID:   req.GetString("actor_id", ""),
			Revision:  int(req.GetInt("revision", 0)),
			PaymentID: req.GetString("payment_id", ""),
		}

		err := svc.ChangeReservationStatus(ctx, cmd)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Status updated successfully"), nil
	})

	// expire_pending_reservations
	toolExpire := mcp.NewProtocolTool("expire_pending_reservations",
		mcp.WithDescription("Called by an external scheduler to expire unconfirmed pending reservations."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithNumber("before", mcp.Description("Unix UTC timestamp threshold"), mcp.Required()),
	)
	s.AddTool(toolExpire, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tenantID := req.GetString("tenant_id", "")
		before := int64(req.GetInt("before", 0))

		count, err := svc.ExpirePendingReservations(ctx, tenantID, before)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Expired %d reservations", count)), nil
	})
}

func registerCalendarTools(s *mcp.MCPServer, svc SchedulingService) {
	// upsert_calendar_config
	toolCfg := mcp.NewProtocolTool("upsert_calendar_config",
		mcp.WithDescription("Sets IANA timezone for a staff member. Must be called before upsert_weekly_calendar."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("staff_id", mcp.Required()),
		mcp.WithString("timezone", mcp.Description("IANA timezone (e.g. 'America/Santiago')"), mcp.Required()),
		mcp.WithBoolean("is_active"),
	)

	s.AddTool(toolCfg, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cfg := WorkCalendarConfig{
			TenantID: req.GetString("tenant_id", ""),
			StaffID:  req.GetString("staff_id", ""),
			Timezone: req.GetString("timezone", ""),
			IsActive: req.GetBool("is_active", false),
		}

		err := svc.UpsertCalendarConfig(ctx, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Calendar config upserted successfully"), nil
	})

	// upsert_weekly_calendar
	toolWk := mcp.NewProtocolTool("upsert_weekly_calendar",
		mcp.WithDescription("Sets weekly schedule for a staff member. Must surface ErrCalendarConfigNotFound with actionable message if config is missing."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("staff_id", mcp.Required()),
		mcp.WithNumber("day_of_week", mcp.Required()),
		mcp.WithNumber("work_start", mcp.Required()),
		mcp.WithNumber("work_finish", mcp.Required()),
		mcp.WithNumber("break_start"),
		mcp.WithNumber("break_finish"),
		mcp.WithBoolean("is_active"),
	)

	s.AddTool(toolWk, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cal := WorkCalendarWeekly{
			TenantID:    req.GetString("tenant_id", ""),
			StaffID:     req.GetString("staff_id", ""),
			DayOfWeek:   int64(req.GetInt("day_of_week", 0)),
			WorkStart:   int64(req.GetInt("work_start", 0)),
			WorkFinish:  int64(req.GetInt("work_finish", 0)),
			BreakStart:  int64(req.GetInt("break_start", 0)),
			BreakFinish: int64(req.GetInt("break_finish", 0)),
			IsActive:    req.GetBool("is_active", false),
		}

		err := svc.UpsertWeeklyCalendar(ctx, cal)
		if err != nil {
			if errors.Is(err, ErrCalendarConfigNotFound) {
				return mcp.NewToolResultError("Set the staff timezone first using upsert_calendar_config"), nil
			}
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Weekly calendar upserted successfully"), nil
	})

	// add_calendar_exception
	toolExc := mcp.NewProtocolTool("add_calendar_exception",
		mcp.WithDescription("Adds a calendar exception for a specific date."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("staff_id", mcp.Required()),
		mcp.WithNumber("specific_date", mcp.Description("Unix UTC midnight"), mcp.Required()),
		mcp.WithString("exception_type", mcp.Description("HOLIDAY | SPECIAL_HOURS | BLOCKED"), mcp.Required()),
		mcp.WithNumber("start_time"),
		mcp.WithNumber("end_time"),
		mcp.WithString("notes"),
	)

	s.AddTool(toolExc, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		exc := WorkCalendarException{
			TenantID:      req.GetString("tenant_id", ""),
			StaffID:       req.GetString("staff_id", ""),
			SpecificDate:  int64(req.GetInt("specific_date", 0)),
			ExceptionType: req.GetString("exception_type", ""),
			StartTime:     int64(req.GetInt("start_time", 0)),
			EndTime:       int64(req.GetInt("end_time", 0)),
			Notes:         req.GetString("notes", ""),
		}

		err := svc.AddException(ctx, exc)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Calendar exception added successfully"), nil
	})

	// remove_calendar_exception
	toolRmExc := mcp.NewProtocolTool("remove_calendar_exception",
		mcp.WithDescription("Removes a calendar exception."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("exception_id", mcp.Required()),
	)

	s.AddTool(toolRmExc, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tenantID := req.GetString("tenant_id", "")
		exceptionID := req.GetString("exception_id", "")

		err := svc.RemoveException(ctx, tenantID, exceptionID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Calendar exception removed successfully"), nil
	})

	// list_availability
	toolAvail := mcp.NewProtocolTool("list_availability",
		mcp.WithDescription("Lists available time slots for a staff member."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("staff_id", mcp.Required()),
		mcp.WithString("config_id", mcp.Required()),
		mcp.WithNumber("from", mcp.Description("Unix UTC midnight"), mcp.Required()),
		mcp.WithNumber("to", mcp.Description("Unix UTC midnight"), mcp.Required()),
	)

	s.AddTool(toolAvail, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tenantID := req.GetString("tenant_id", "")
		staffID := req.GetString("staff_id", "")
		configID := req.GetString("config_id", "")
		from := int64(req.GetInt("from", 0))
		to := int64(req.GetInt("to", 0))

		slots, err := svc.ListAvailability(ctx, tenantID, staffID, configID, from, to)
		if err != nil {
			if errors.Is(err, ErrCalendarConfigNotFound) {
				return mcp.NewToolResultError("Set the staff timezone first using upsert_calendar_config"), nil
			}
			return mcp.NewToolResultError(err.Error()), nil
		}

		b, err := json.Marshal(slots)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(string(b)), nil
	})
}
