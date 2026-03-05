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
	toolCreate := mcp.NewTool("create_reservation",
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
		args, ok := req.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		cmd := CreateReservationCmd{
			TenantID:                args["tenant_id"].(string),
			ClientID:                args["client_id"].(string),
			CreatorUserID:           args["creator_user_id"].(string),
			EmployeeServiceConfigID: args["employee_service_config_id"].(string),
			SlotStartUTC:            getInt(args["slot_start_utc"]),
		}
		if notes, ok := args["notes"].(string); ok {
			cmd.Notes = notes
		}
		if resch, ok := args["rescheduled_from_id"].(string); ok {
			cmd.RescheduledFromID = resch
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
	toolGet := mcp.NewTool("get_reservation",
		mcp.WithDescription("Gets a reservation by ID."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("id", mcp.Required()),
	)
	s.AddTool(toolGet, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := req.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		tenantID := args["tenant_id"].(string)
		id := args["id"].(string)

		res, err := svc.GetReservation(ctx, tenantID, id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		b, _ := json.Marshal(res)
		return mcp.NewToolResultText(string(b)), nil
	})

	// list_reservations_by_staff
	toolListStaff := mcp.NewTool("list_reservations_by_staff",
		mcp.WithDescription("Lists reservations by staff ID and date range."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("staff_id", mcp.Required()),
		mcp.WithNumber("from", mcp.Required()),
		mcp.WithNumber("to", mcp.Required()),
	)
	s.AddTool(toolListStaff, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := req.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		res, err := svc.ListReservationsByStaff(
			ctx,
			args["tenant_id"].(string),
			args["staff_id"].(string),
			getInt(args["from"]),
			getInt(args["to"]),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		b, _ := json.Marshal(res)
		return mcp.NewToolResultText(string(b)), nil
	})

	// list_reservations_by_client
	toolListClient := mcp.NewTool("list_reservations_by_client",
		mcp.WithDescription("Lists reservations by client ID."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("client_id", mcp.Required()),
	)
	s.AddTool(toolListClient, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := req.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		res, err := svc.ListReservationsByClient(
			ctx,
			args["tenant_id"].(string),
			args["client_id"].(string),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		b, _ := json.Marshal(res)
		return mcp.NewToolResultText(string(b)), nil
	})

	// change_reservation_status
	toolStatus := mcp.NewTool("change_reservation_status",
		mcp.WithDescription("Changes a reservation status via FSM event."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("id", mcp.Required()),
		mcp.WithString("event", mcp.Required()),
		mcp.WithString("actor_id", mcp.Required()),
		mcp.WithString("payment_id"),
		mcp.WithNumber("revision", mcp.Required()),
	)
	s.AddTool(toolStatus, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := req.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		cmd := ChangeStatusCmd{
			TenantID: args["tenant_id"].(string),
			ID:       args["id"].(string),
			Event:    args["event"].(string),
			ActorID:  args["actor_id"].(string),
			Revision: int(getInt(args["revision"])),
		}
		if pay, ok := args["payment_id"].(string); ok {
			cmd.PaymentID = pay
		}

		err := svc.ChangeReservationStatus(ctx, cmd)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Status updated successfully"), nil
	})

	// expire_pending_reservations
	toolExpire := mcp.NewTool("expire_pending_reservations",
		mcp.WithDescription("Called by an external scheduler to expire unconfirmed pending reservations."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithNumber("before", mcp.Description("Unix UTC timestamp threshold"), mcp.Required()),
	)
	s.AddTool(toolExpire, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := req.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		tenantID := args["tenant_id"].(string)
		before := getInt(args["before"])

		count, err := svc.ExpirePendingReservations(ctx, tenantID, before)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Expired %d reservations", count)), nil
	})
}

func getInt(val any) int64 {
	switch v := val.(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	}
	return 0
}

func registerCalendarTools(s *mcp.MCPServer, svc SchedulingService) {
	// upsert_calendar_config
	toolCfg := mcp.NewTool("upsert_calendar_config",
		mcp.WithDescription("Sets IANA timezone for a staff member. Must be called before upsert_weekly_calendar."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("staff_id", mcp.Required()),
		mcp.WithString("timezone", mcp.Description("IANA timezone (e.g. 'America/Santiago')"), mcp.Required()),
		mcp.WithBoolean("is_active"),
	)

	s.AddTool(toolCfg, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := req.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		tenantID, _ := args["tenant_id"].(string)
		staffID, _ := args["staff_id"].(string)
		timezone, _ := args["timezone"].(string)
		isActive, _ := args["is_active"].(bool)

		cfg := WorkCalendarConfig{
			TenantID: tenantID,
			StaffID:  staffID,
			Timezone: timezone,
			IsActive: isActive,
		}

		err := svc.UpsertCalendarConfig(ctx, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Calendar config upserted successfully"), nil
	})

	// upsert_weekly_calendar
	toolWk := mcp.NewTool("upsert_weekly_calendar",
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
		args, ok := req.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		tenantID, _ := args["tenant_id"].(string)
		staffID, _ := args["staff_id"].(string)
		isActive, _ := args["is_active"].(bool)

		cal := WorkCalendarWeekly{
			TenantID:    tenantID,
			StaffID:     staffID,
			DayOfWeek:   getInt(args["day_of_week"]),
			WorkStart:   getInt(args["work_start"]),
			WorkFinish:  getInt(args["work_finish"]),
			BreakStart:  getInt(args["break_start"]),
			BreakFinish: getInt(args["break_finish"]),
			IsActive:    isActive,
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
	toolExc := mcp.NewTool("add_calendar_exception",
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
		args, ok := req.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		tenantID, _ := args["tenant_id"].(string)
		staffID, _ := args["staff_id"].(string)
		exceptionType, _ := args["exception_type"].(string)
		notes, _ := args["notes"].(string)

		exc := WorkCalendarException{
			TenantID:      tenantID,
			StaffID:       staffID,
			SpecificDate:  getInt(args["specific_date"]),
			ExceptionType: exceptionType,
			StartTime:     getInt(args["start_time"]),
			EndTime:       getInt(args["end_time"]),
			Notes:         notes,
		}

		err := svc.AddException(ctx, exc)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Calendar exception added successfully"), nil
	})

	// remove_calendar_exception
	toolRmExc := mcp.NewTool("remove_calendar_exception",
		mcp.WithDescription("Removes a calendar exception."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("exception_id", mcp.Required()),
	)

	s.AddTool(toolRmExc, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := req.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		tenantID, _ := args["tenant_id"].(string)
		exceptionID, _ := args["exception_id"].(string)

		err := svc.RemoveException(ctx, tenantID, exceptionID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Calendar exception removed successfully"), nil
	})

	// list_availability
	toolAvail := mcp.NewTool("list_availability",
		mcp.WithDescription("Lists available time slots for a staff member."),
		mcp.WithString("tenant_id", mcp.Required()),
		mcp.WithString("staff_id", mcp.Required()),
		mcp.WithString("config_id", mcp.Required()),
		mcp.WithNumber("from", mcp.Description("Unix UTC midnight"), mcp.Required()),
		mcp.WithNumber("to", mcp.Description("Unix UTC midnight"), mcp.Required()),
	)

	s.AddTool(toolAvail, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := req.Params.Arguments.(map[string]any)
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		tenantID, _ := args["tenant_id"].(string)
		staffID, _ := args["staff_id"].(string)
		configID, _ := args["config_id"].(string)
		from := getInt(args["from"])
		to := getInt(args["to"])

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
